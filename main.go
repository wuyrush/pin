package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	envVarPrefix = "PIN"

	envVerbose = "VERBOSE"
	envHost    = "HOST"
	envPort    = "PORT"
)

func main() {
	// read configuration from env vars
	viper.SetEnvPrefix(envVarPrefix)
	viper.AutomaticEnv()

	// setup logging
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

	log.Debug("test viper")

	mainHandler := func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, "welcome to pin!")
	}
	http.HandleFunc("/", mainHandler)

	host, port := viper.GetString(envHost), viper.GetString(envPort)
	log.WithFields(log.Fields{
		"host": host,
		"port": port,
	}).Infof("server is starting up")
	addr := fmt.Sprintf("%s:%s", host, port)
	log.Fatal(http.ListenAndServe(addr, nil))
}
