package main

import (
	"net/http"
	"time"

	hr "github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"wuyrush.io/pin/common/logging"
)

const (
	envReaderVerbose = "PIN_READER_VERBOSE"
	envReaderAddr    = "PIN_READER_SERVER_ADDR"
)

// Reader handles read traffic of pin application. Multiple Readers form the service
// component to handle the application's read operations
type reader struct {
	R *hr.Router
}

func (rd *reader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rd.R.ServeHTTP(w, r)
}

func serve() error {
	s, err := setup()
	if err != nil {
		return err
	}
	// TODO: response to system signals and graceful shutdown: s.Shutdown(ctx) and s.RegisterOnShutdown(ctx)
	return s.ListenAndServe()
}

func setup() (*http.Server, error) {
	viper.AutomaticEnv()
	logging.SetupLog("pin-reader", viper.GetBool(envReaderVerbose))
	r := &reader{}
	r.SetupRoutes()
	return &http.Server{
		Addr:    viper.GetString(envReaderAddr),
		Handler: r,
		// TODO: tweak setups
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 10,
	}, nil
}

func (rd *reader) SetupRoutes() {
	r := hr.New()
	r.GET("/pin/:id", rd.HandleTaskGetPin)
	r.GET("/pins/public", rd.HandleTaskListPublicPins)
	r.GET("/pins/private", rd.HandleTaskListPrivatePins)
	r.GET("/user", rd.HandleAuthGetUserProfile)
	rd.R = r
	return
}

func (rd *reader) HandleTaskGetPin(w http.ResponseWriter, r *http.Request, p hr.Params) {
	log.Infof("hit GetPin with pin id %s", p.ByName("id"))
}
func (rd *reader) HandleTaskListPublicPins(w http.ResponseWriter, r *http.Request, p hr.Params)  {}
func (rd *reader) HandleTaskListPrivatePins(w http.ResponseWriter, r *http.Request, p hr.Params) {}
func (rd *reader) HandleAuthGetUserProfile(w http.ResponseWriter, r *http.Request, p hr.Params)  {}
