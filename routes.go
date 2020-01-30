package main

import "github.com/julienschmidt/httprouter"

// set up routes
func (s *pinServer) SetupMux() {
	r := httprouter.New()
	r.GET("/", s.HandleTaskCreatePin())
	r.GET("/pin", s.HandleTaskCreatePin())
	r.POST("/pin", s.HandleTaskCreatePin())
	r.GET("/pin/:id", s.HandleTaskGetPin())
	r.GET("/pins/anonymous", s.HandleTaskListAnonymousPins())
	r.GET("/pins/user", s.HandleTaskListUserPins())
	r.DELETE("/pin/:id", s.HandleTaskDeletePin())
	r.GET("/pin/:id/attachment/:filename", s.HandleTaskGetPinAttachment())
	// user related
	r.GET("/register", s.HandleTaskRegister())
	r.POST("/register", s.HandleTaskRegister())
	r.GET("/profile", s.HandleTaskGetUserProfile())
	r.GET("/login", s.HandleAuthLogin())
	r.POST("/login", s.HandleAuthLogin())
	r.POST("/logout", s.HandleAuthLogout())
	s.Router = r
}
