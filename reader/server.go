package main

import (
	"github.com/gin-gonic/gin"
)

// Reader handles read traffic of pin application. Multiple Readers form the service
// component to handle the application's read operations
type reader struct {
	Router *gin.Engine
}

func serve() error {
	s := setup()
	return s.Router.Run()
}

func setup() *reader {
	r := &reader{}
	r.SetupRoutes()
	return r
}

func (r *reader) SetupRoutes() {
	rt := gin.Default()

	rt.GET("/pin/:pid", r.HandleTaskGetPin)
	rt.GET("/pins/public", r.HandleTaskListPublicPins)
	rt.GET("/pins/private", r.HandleTaskListPrivatePins)
	rt.GET("/user", r.HandleAuthGetUserProfile)
	r.Router = rt
	return
}

func (r *reader) HandleTaskGetPin(ctx *gin.Context)          {}
func (r *reader) HandleTaskListPublicPins(ctx *gin.Context)  {}
func (r *reader) HandleTaskListPrivatePins(ctx *gin.Context) {}
func (r *reader) HandleAuthGetUserProfile(ctx *gin.Context)  {}
