package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/go-pg/pg/v9"
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
	envDBHost      = "DB_HOST"
	envDBPort      = "DB_PORT"
	envDBUser      = "DB_USER"
	envDBPasswd    = "DB_PASSWD"
	envDBName      = "DB_NAME"
)

type pinServer struct {
	metadataCache pinCache
	db            pinDB
	router        *httprouter.Router
}

func (s *pinServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

type pinServerOption func(*pinServer)

func newPinServer(opts ...pinServerOption) *pinServer {
	s := &pinServer{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// start up application server and serve incoming requests
func serve() error {
	// read configuration from env vars
	viper.AutomaticEnv()

	setupLog()
	// initialize dependencies in data layer
	// TODO: add retry with tight timeout to poll the status of dependencies till they are ready or we timeout
	// NOTE docker compose's depends_on feature only guarantee the startup order of service containers,
	// it is us who define when a service becomes ready
	metadataCache, err := setupMetadataCache()
	if err != nil {
		return err
	}
	defer metadataCache.Close()
	db, err := setupDB()
	if err != nil {
		return err
	}
	defer db.Close()

	svr := newPinServer(
		func(s *pinServer) {
			s.metadataCache = metadataCache
			s.db = db
		},
		setupMux(),
	)

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

func setupMetadataCache() (pinCache, error) {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", viper.GetString(envRedisHost), viper.GetString(envRedisPort)),
		Password: viper.GetString(envRedisPasswd),
		DB:       viper.GetInt(envRedisDB),
	})
	// verify the client is up correctly
	if _, err := redisClient.Ping().Result(); err != nil {
		return nil, errServiceFailure("failed initializing Redis").WithCause(err)
	}
	return &pinRedis{redisClient}, nil
}

func setupDB() (pinDB, error) {
	db := pg.Connect(&pg.Options{
		Addr:     fmt.Sprintf("%s:%s", viper.GetString(envDBHost), viper.GetString(envDBPort)),
		User:     viper.GetString(envDBUser),
		Password: viper.GetString(envDBPasswd),
		Database: viper.GetString(envDBName),
	})
	// verify the connection is up correctly
	// https://github.com/go-pg/pg/wiki/FAQ#how-to-check-connection-health
	// TODO: test out what the query does and see if there is any performance impact
	if _, err := db.Exec("SELECT 1"); err != nil {
		return nil, errServiceFailure("failed initializing Postgres DB").WithCause(err)
	}
	return &pgr{db}, nil
}

// set up routes
func setupMux() pinServerOption {
	return func(s *pinServer) {
		r := httprouter.New()
		r.GET("/pin/:id", s.HandleTaskGetPin())

		s.router = r
	}
}

// --------------- Handles ---------------

func (s *pinServer) HandleTaskGetPin() httprouter.Handle {
	// TODO: implement
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		fmt.Fprintf(w, "visiting pin with id %s", ps.ByName("id"))
	}
}
