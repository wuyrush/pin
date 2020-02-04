// Package deleter vends a long-running worker to delete stale pin data and attachments.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bluele/gcache"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"wuyrush.io/pin/common/logging"
	rt "wuyrush.io/pin/common/retry"
	cst "wuyrush.io/pin/constants"
	pe "wuyrush.io/pin/errors"
	md "wuyrush.io/pin/models"
	st "wuyrush.io/pin/stores"
)

func main() {
	if err := runDeleter(); err != nil {
		log.WithError(err).Fatal("error running deleter")
	}
}

func setupPinStore() (st.PinStore, error) {
	retryOpts := []rt.RetryOption{
		rt.WithTimeout(3 * time.Second),
		rt.WithBaseDelay(100 * time.Millisecond),
		rt.WithExp(2.0),
		rt.WithRetryOn(rt.IsDepOffline),
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:       fmt.Sprintf("%s:%s", viper.GetString(cst.EnvRedisHost), viper.GetString(cst.EnvRedisPort)),
		Password:   viper.GetString(cst.EnvRedisPasswd),
		DB:         viper.GetInt(cst.EnvRedisDB),
		MaxRetries: 3,
	})
	// verify the client is up correctly
	pingFn := func() error {
		_, err := redisClient.Ping().Result()
		return err
	}
	if err := rt.Retry(pingFn, retryOpts...); err != nil {
		return nil, pe.ErrServiceFailure("failed initializing Redis").WithCause(err)
	}
	return &st.RedisStore{DB: redisClient}, nil
}

func setupFileStore() (st.FileStore, error) {
	return &st.LocalFileStore{}, nil
}

type deleter struct {
	FS       st.FileStore
	PS       st.PinStore
	wipCache gcache.Cache
}

func runDeleter() error {
	viper.AutomaticEnv()
	logging.SetupLog("PinDeleter")
	// setup dependencies
	clog := logging.WithFuncName()
	ps, err := setupPinStore()
	if err != nil {
		clog.WithError(err).Error("error setting up PinStore")
		return err
	}
	defer ps.Close()
	fs, err := setupFileStore()
	if err != nil {
		clog.WithError(err).Error("error setting up FileStore")
		return err
	}
	defer fs.Close()
	localCacheSize := viper.GetInt(cst.EnvPinDeleterLocalCacheSize)
	wipCache := gcache.New(localCacheSize).LRU().Build()
	d := &deleter{FS: fs, PS: ps, wipCache: wipCache}
	return d.Run()
}

func (d *deleter) Run() *pe.PinErr {
	clog := logging.WithFuncName()
	freq := viper.GetDuration(cst.EnvDeleterSweepFreq)
	if freq <= 0 {
		clog.WithField("sweepFrequency", freq).Fatal("got non-positive deleter sweep frequency")
	}
	execPoolSize := viper.GetInt(cst.EnvDeleterExecutorPoolSize)
	if execPoolSize <= 0 {
		clog.WithField("deleterExecutorPoolSize", execPoolSize).Fatal("got non-positive deleter executor pool size")
	}
	quotas := make(chan struct{}, execPoolSize)
	maxLoad := viper.GetInt(cst.EnvDeleterMaxSweepLoad)
	loadTkr := time.NewTicker(freq)
	// ensure the worker can be responsive to system signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM)
LoopRun:
	for {
		select {
		case <-loadTkr.C:
			jks, err := d.Load(maxLoad)
			if err != nil {
				clog.WithError(err).Error("error loading junk pins")
				// TODO: terminate when dependencies are hard-down
				return err
			}
			clog.WithField("count", len(jks)).Debug("junk pins loaded")
			// dispatch junk to workers in pool for disposal
			for _, jk := range jks {
				go func(jk *md.Junk) {
					quotas <- struct{}{}
					defer func() { <-quotas }()
					if err := d.Delete(jk); err != nil {
						clog.WithError(err).WithField("junk", jk).Error("error deleting junk pin")
					}
					clog.WithField("junk", jk).Debug("successfully deleting junk pin")
				}(jk)
			}
		case <-sigChan:
			clog.Info("got termination signal from kernel. Stopping")
			break LoopRun
		}
	}
	return nil
}

// Load loads up to max junk pins from PinStore for cleanup. It loads all junk pins available in PinStore if
// max == 0.
func (d *deleter) Load(max int) ([]*md.Junk, *pe.PinErr) {
	clog := logging.WithFuncName()
	// get stale pin data with PinStore
	jks, err := d.PS.Junk(max)
	if err != nil {
		clog.WithError(err).Error("error loading junk pins from PinStore")
		return nil, err
	}
	clog.Debug("successfully loaded junk pins from PinStore")
	// query local cache to filter out pins which are already WIP
	newJks := []*md.Junk{}
	for _, jk := range jks {
		if _, err := d.wipCache.Get(jk.PinID); err != nil {
			if err == gcache.KeyNotFoundError {
				newJks = append(newJks, jk)
			} else {
				msg := "error getting pin id from local cache"
				clog.WithError(err).Error(msg)
				return nil, pe.ErrServiceFailure(msg).WithCause(err)
			}
		}
	}
	// cache the ids of these pins in WIP cache in best-effort manner - pin id which we failed to set in cache
	// will be picked up by deleter in its next sweep
	execPoolSize := viper.GetInt(cst.EnvDeleterExecutorPoolSize)
	exp := viper.GetDuration(cst.EnvDeleterWIPCacheEntryExpiry)
	quotas := make(chan struct{}, execPoolSize)
	for _, jk := range newJks {
		go func(id string) {
			quotas <- struct{}{}
			defer func() { <-quotas }()
			if err := d.wipCache.SetWithExpire(id, struct{}{}, exp); err != nil {
				clog.WithError(err).Errorf("error keying pin id %s in local cache", id)
			}
		}(jk.PinID)
	}
	return newJks, nil
}

func (d *deleter) Delete(j *md.Junk) *pe.PinErr {
	clog := logging.WithFuncName().WithField("pinID", j.PinID)
	// remove all pin attachment files from FileStore
	errs, done := make(chan *pe.PinErr, len(j.FileRefs)), make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(len(j.FileRefs))
	// waiter
	go func() {
		wg.Wait()
		close(done)
	}()
	// spawn ref deleters
	for _, ref := range j.FileRefs {
		// no need to limit the concurrency since we had limited the number of attachments
		// per pin in a very low number
		go func(r string) {
			defer wg.Done()
			if err := d.FS.Delete(r); err != nil {
				clog.WithError(err).WithField("ref", r).
					Errorf("error deleting pin attachment with FileStore")
				errs <- err
			}
		}(ref)
	}
	select {
	case err := <-errs:
		// ok to return without getting all errors due to the use of buffered chan
		return err
	case <-done:
	}
	// At this point ALL the pin's attachments are cleaned up; Deregister pin from PinStore.
	if err := d.PS.Deregister(j.PinID); err != nil {
		clog.WithError(err).Error("error deregistering pin from PinStore")
		return err
	}
	// remove corresponding pin id from local cache
	d.wipCache.Remove(j.PinID)
	return nil
}
