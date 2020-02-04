package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"wuyrush.io/pin/common/logging"
	rt "wuyrush.io/pin/common/retry"
	cst "wuyrush.io/pin/constants"
	pe "wuyrush.io/pin/errors"
	st "wuyrush.io/pin/stores"
)

// a combination of web and application server since it serves both application logic and web page rendering
type pinServer struct {
	PS     st.PinStore
	FS     st.FileStore
	Router *httprouter.Router
}

func (s *pinServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Router.ServeHTTP(w, r)
}

// start up application server and serve incoming requests
func serve() error {
	// read configuration from env vars
	viper.AutomaticEnv()
	logging.SetupLog("PinServer")
	// initialize dependencies in data layer
	// NOTE docker compose's depends_on feature only guarantee the startup order of *service containers*,
	// instead of the services themselves - It is us who define when the services are ready
	ps, err := setupPinStore()
	if err != nil {
		return err
	}
	defer ps.Close()
	fs, err := setupFileStore()
	if err != nil {
		return err
	}
	defer fs.Close()

	svr := &pinServer{}
	svr.PS = ps
	svr.FS = fs
	svr.SetupMux()

	host, port := viper.GetString(cst.EnvAppHost), viper.GetString(cst.EnvAppPort)
	log.WithFields(log.Fields{
		"host": host,
		"port": port,
	}).Infof("pin server is starting up")
	addr := fmt.Sprintf("%s:%s", host, port)
	return http.ListenAndServe(addr, svr)
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
