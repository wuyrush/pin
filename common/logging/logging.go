package logging

import (
	"os"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	cst "wuyrush.io/pin/constants"
)

// ServiceFormatter is a Formatter that:
// 1. logs the unix time in milliseconds;
// 2. logs specified service/service component name;
type ServiceFormatter struct {
	svcName string
	log.Formatter
}

// I've noticed passing a mutated *log.Entry value to downstream formatter results in logs with panic level
// and empty message, but never sure about why it happens
func (f *ServiceFormatter) Format(e *log.Entry) ([]byte, error) {
	e.Data["epochTimeMillis"] = e.Time.UnixNano() / int64(time.Millisecond)
	e.Data["service"] = f.svcName
	return f.Formatter.Format(e)
}

// SetupLog setups service-specific logging.
func SetupLog(name string) {
	log.SetOutput(os.Stdout)
	// use unix timestamp instead of zonal one
	f := &ServiceFormatter{
		svcName:   name,
		Formatter: &log.JSONFormatter{DisableTimestamp: true},
	}
	log.SetFormatter(f)
	log.SetLevel(log.InfoLevel)
	if viper.GetBool(cst.EnvVerbose) {
		log.SetLevel(log.DebugLevel)
	}
}

// WithFuncName returns a *logrus.Entry marked with the name of function calling  WithFuncName
func WithFuncName() *logrus.Entry {
	// get the pc of the function that calls the current function
	pc, _, _, ok := runtime.Caller(1)
	var funcName string
	if ok {
		frs := runtime.CallersFrames([]uintptr{pc})
		fr, _ := frs.Next()
		funcName = fr.Function
	}
	return log.WithField(cst.LogFieldFuncName, funcName)
}
