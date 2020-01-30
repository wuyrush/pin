package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	envVerbose     = "PIN_VERBOSE"
	envAppHost     = "PIN_HOST"
	envAppPort     = "PIN_PORT"
	envRedisHost   = "REDIS_HOST"
	envRedisPort   = "REDIS_PORT"
	envRedisPasswd = "REDIS_PASSWD"
	envRedisDB     = "REDIS_DB"

	envReqBodySizeMaxByte       = "PIN_REQ_BODY_SIZE_MAX_BYTE"
	envPinTitleSizeMaxByte      = "PIN_TITLE_SIZE_MAX_BYTE"
	envPinNoteSizeMaxByte       = "PIN_NOTE_SIZE_MAX_BYTE"
	envPinAttachmentSizeMaxByte = "PIN_ATTACHMENT_SIZE_MAX_BYTE"
	envPinAttachmentCntMax      = "PIN_ATTACHMENT_COUNT_MAX"
)

type pinServer struct {
	PS     pinStore
	FS     fileStore
	Router *httprouter.Router
}

func (s *pinServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Router.ServeHTTP(w, r)
}

// start up application server and serve incoming requests
func serve() error {
	// read configuration from env vars
	viper.AutomaticEnv()

	setupLog()
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

	host, port := viper.GetString(envAppHost), viper.GetString(envAppPort)
	log.WithFields(log.Fields{
		"host": host,
		"port": port,
	}).Infof("server is starting up")
	addr := fmt.Sprintf("%s:%s", host, port)
	return http.ListenAndServe(addr, svr)
}

func setupLog() {
	log.SetOutput(os.Stdout)
	// use unix timestamp instead of zonal one
	f := &unixTimeFormatter{
		&log.JSONFormatter{DisableTimestamp: true},
	}
	log.SetFormatter(f)
	log.SetLevel(log.InfoLevel)
	if viper.GetBool(envVerbose) {
		log.SetLevel(log.DebugLevel)
	}
}

func setupPinStore() (pinStore, error) {
	retryOpts := []retryOption{
		withTimeout(3 * time.Second),
		withBaseDelay(100 * time.Millisecond),
		withExp(2.0),
		withRetryOn(isDepOffline),
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:       fmt.Sprintf("%s:%s", viper.GetString(envRedisHost), viper.GetString(envRedisPort)),
		Password:   viper.GetString(envRedisPasswd),
		DB:         viper.GetInt(envRedisDB),
		MaxRetries: 3,
	})
	// verify the client is up correctly
	pingFn := func() error {
		_, err := redisClient.Ping().Result()
		return err
	}
	if err := retry(pingFn, retryOpts...); err != nil {
		return nil, errServiceFailure("failed initializing Redis").WithCause(err)
	}
	return &redisStore{DB: redisClient}, nil
}

func setupFileStore() (fileStore, error) {
	return &localFileStore{}, nil
}

func isDepOffline(e error) bool {
	if e != nil && strings.Index(e.Error(), "connect: connection refused") >= 0 {
		return true
	}
	return false
}
