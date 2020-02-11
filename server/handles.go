package main

import (
	"bufio"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/segmentio/ksuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"wuyrush.io/pin/common/logging"
	cst "wuyrush.io/pin/constants"
	pe "wuyrush.io/pin/errors"
	md "wuyrush.io/pin/models"
)

const (
	goodForMin        = time.Second * 30
	goodForMax        = time.Hour * 24
	errMsgPinNotFound = "pin not found"
)

func (s *pinServer) HandleTaskGetCreatePinPage() httprouter.Handle {
	clog := logging.WithFuncName().WithField("httpMethod", http.MethodGet)
	tmplPath := "templates/create_pin.html"
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		// fail early if err since this is critical path
		clog.WithError(err).WithField("templatePath", tmplPath).Fatal("html template not loaded")
	}
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if err := tmpl.Execute(w, nil); err != nil {
			clog.WithError(err).WithField("templatePath", tmplPath).Error("error executing html template")
		}
	}
}

/*
	Always returns the (maybe partial) resulting pin data back to customer, no matter we process the request
	with success or not, so that they can always double check. Meanwhile in case of error, direct customer to
	CreatePin page and fill the form with data customer had already input, so that they can save lots of
	typing when retry
*/
func (s *pinServer) HandleTaskCreatePin() httprouter.Handle {
	clog := logging.WithFuncName().WithField("httpMethod", http.MethodPost)
	tmplPathCreatePin := "templates/create_pin.html"
	tmplPathGetPin := "templates/get_pin.html"
	maxReqBodySize := viper.GetInt64(cst.EnvReqBodySizeMaxByte)
	// fail early if err since this is critical path
	tmplCreatePin, err := template.ParseFiles(tmplPathCreatePin)
	if err != nil {
		clog.WithError(err).WithField("templatePath", tmplPathCreatePin).Fatal("html template not loaded")
	}
	tmplGetPin, err := template.ParseFiles(tmplPathGetPin)
	if err != nil {
		clog.WithError(err).WithField("templatePath", tmplPathGetPin).Fatal("html template not loaded")
	}
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// limit request size and parse request form
		r.Body = http.MaxBytesReader(w, r.Body, maxReqBodySize)
		if err := r.ParseMultipartForm(128); err != nil {
			code, msg := http.StatusBadRequest, "error parsing form"
			if strings.Index(err.Error(), "http: request body too large") >= 0 {
				msg = fmt.Sprintf("request oversized. Request size must be under %f mebibyte",
					float64(maxReqBodySize)/(1024.*1024.))
				code = http.StatusRequestEntityTooLarge
			}
			clog.WithError(err).Error(msg)
			http.Error(w, msg, code)
			return
		}
		// 1. assemble and validate pin data
		p, err := s.buildPin(r)
		if err != nil {
			clog.WithError(err).Error("error building pin from input data")
			w.WriteHeader(err.StatusCode())
			execTemplateLog(tmplCreatePin, w, md.PinView{Pin: *p, Err: err.Error()},
				clog.WithField("templatePath", tmplPathCreatePin))
			return
		}
		plog := clog.WithField("pinID", p.ID)
		p.CreationTime = time.Now()
		pinExpiry := p.CreationTime.Add(p.GoodFor)
		// register pin
		if err := s.PS.Register(p); err != nil {
			plog.WithError(err).Error("error registering pin data")
			w.WriteHeader(err.StatusCode())
			execTemplateLog(tmplCreatePin, w, md.PinView{Pin: *p, Err: err.Error()},
				plog.WithField("templatePath", tmplPathCreatePin))
			return
		}
		// save pin metadata
		if err := s.PS.Save(p); err != nil {
			plog.WithError(err).Error("error saving pin metadata")
			w.WriteHeader(err.StatusCode())
			execTemplateLog(tmplCreatePin, w, md.PinView{Pin: *p, Err: err.Error()},
				plog.WithField("templatePath", tmplPathCreatePin))
			return
		}
		// save pin attachment data
		for _, fh := range r.MultipartForm.File["attachments"] {
			flog := plog.WithField("filename", fh.Filename)
			ref := p.Attachments[fh.Filename]
			f, err := fh.Open()
			if err != nil {
				flog.WithError(err).WithField("filename", fh.Filename).Error("error opening pin attachment")
				w.WriteHeader(http.StatusInternalServerError)
				execTemplateLog(tmplCreatePin, w, md.PinView{
					Pin: *p,
					Err: fmt.Sprintf("error opening attachment %s: %s", fh.Filename, err),
				}, flog.WithField("templatePath", tmplPathCreatePin))
				return
			}
			defer f.Close()
			if err := s.FS.Save(ref, f); err != nil {
				flog.WithError(err).WithField("filename", fh.Filename).Error("error saving pin attachment")
				w.WriteHeader(err.StatusCode())
				execTemplateLog(tmplCreatePin, w, md.PinView{
					Pin: *p,
					Err: fmt.Sprintf("error saving attachment %s: %s", fh.Filename, err),
				}, flog.WithField("templatePath", tmplPathCreatePin))
				return
			}
		}
		// rendered the saved pin info page so that customer can double check if the info is expected
		pv := md.PinView{
			Pin:           *p,
			URL:           fmt.Sprintf("/pin/%s", p.ID), // TODO: compute the full url
			Expiry:        pinExpiry,
			FilenameToURL: map[string]string{},
		}
		for fn := range p.Attachments {
			pv.FilenameToURL[fn] = fmt.Sprintf("/pin/%s/attachment/%s", p.ID, url.PathEscape(fn))
		}
		execTemplateLog(tmplGetPin, w, pv, plog.WithField("templatePath", tmplPathGetPin))
	}
}

func (s *pinServer) buildPin(r *http.Request) (*md.Pin, *pe.PinErr) {
	const respMsgErrPinInfo = "error pinning info"
	// 3. generate pin id
	pinKsuid, err := ksuid.NewRandom()
	if err != nil {
		logging.WithFuncName().WithError(err).Error("fail to generate pin id")
		return nil, pe.ErrServiceFailure(respMsgErrPinInfo).WithCause(err)
	}
	p := &md.Pin{
		ID:    pinKsuid.String(),
		Title: r.FormValue("title"),
		Note:  r.FormValue("note"),
	}
	p.Mode = md.AccessModePublic
	if r.FormValue("private") == "true" {
		p.Mode = md.AccessModePrivate
	}
	p.ReadAndBurn = false
	if r.FormValue("read-and-burn") == "true" {
		p.ReadAndBurn = true
	}
	goodFor, err := time.ParseDuration(r.FormValue("good-for"))
	if err != nil {
		return p, pe.ErrBadInput("error parsing good-for period").WithCause(err)
	}
	if goodFor < goodForMin || goodFor > goodForMax {
		return p, pe.ErrBadInput("good-for period out of range")
	}
	p.GoodFor = goodFor
	p.Attachments = map[string]string{}
	fs := r.MultipartForm.File
	for _, fh := range fs["attachments"] {
		p.Attachments[fh.Filename] = s.FS.Ref(p.ID, fh.Filename)
	}
	return p, nil
}

func (s *pinServer) HandleTaskGetPin() httprouter.Handle {
	clog := logging.WithFuncName()
	tmplPath := "templates/get_pin.html"
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		clog.WithError(err).WithField("templatePath", tmplPath).Fatal("html template not loaded")
	}
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// 0. validate input pin id
		pinID := ps.ByName("id")
		plog := clog.WithField("pinID", pinID)
		if _, err := ksuid.Parse(pinID); err != nil {
			plog.WithError(err).Error("got invalid pin ID")
			http.NotFound(w, r)
		}
		// 1. get pin data from pin store
		p, err := s.PS.Get(pinID)
		if err != nil {
			plog.WithError(err).Error("error getting pin from pinStore")
			w.WriteHeader(err.StatusCode())
			execTemplateLog(tmpl, w, md.PinView{Err: err.Error()}, plog.WithField("templatePath", tmplPath))
			return
		}
		// TODO: 2. access control - check if the pin is accessible to the requester or not
		// TODO: 3. read-and-burn - update view count  <- this should be done by pinStore
		// 4. assemble response and return
		pv := md.PinView{
			Pin:           *p,
			FilenameToURL: map[string]string{},
		}
		for fn := range p.Attachments {
			url := fmt.Sprintf("/pin/%s/attachment/%s", p.ID, url.PathEscape(fn))
			pv.FilenameToURL[fn] = url
		}
		execTemplateLog(tmpl, w, pv, plog.WithField("templatePath", tmplPath))
	}
}

// HandleTaskDeletePin handles request to remove a specified pinned information. Note
func (s *pinServer) HandleTaskDeletePin() httprouter.Handle {
	// TODO: implement
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	}
}

func (s *pinServer) HandleTaskGetPinAttachment() httprouter.Handle {
	clog := logging.WithFuncName()
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		pinID, urlencodedFn := ps.ByName("id"), ps.ByName("filename")
		if _, err := ksuid.Parse(pinID); err != nil {
			clog.WithError(err).Error("got invalid pin ID")
			http.Error(w, "pin not found", http.StatusNotFound)
			return
		}
		filename, err := url.PathUnescape(urlencodedFn)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid url-encoded filename %s", urlencodedFn), http.StatusBadRequest)
			return
		}
		flog := clog.WithFields(logrus.Fields{"pinID": pinID, "filename": filename})
		p, gerr := s.PS.Get(pinID)
		if gerr != nil {
			flog.WithError(gerr).Error("error getting pin data")
			http.Error(w, gerr.Error(), gerr.StatusCode())
			return
		}
		ref, ok := p.Attachments[filename]
		if !ok {
			flog.Error("pin attachment not found")
			http.Error(w, fmt.Sprintf("attachment named %s for pin %s no found", filename, pinID),
				http.StatusNotFound)
			return
		}
		rc, gerr := s.FS.Get(ref)
		if gerr != nil {
			flog.WithError(gerr).Error("error getting io stream of attachment")
			http.Error(w, gerr.Error(), gerr.StatusCode())
			return
		}
		defer rc.Close()
		rd := bufio.NewReader(rc)
		// header to force download behavior on browser clients
		headers := w.Header()
		headers.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		w.WriteHeader(http.StatusOK)
		if n, err := rd.WriteTo(w); err != nil {
			// TODO: discern client errors(client closed connection etc) from server ones
			flog.WithError(err).Error("error sending attachment data to requester")
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			flog.WithField("bytesWritten", n).Info("attachment sent to requester successfully")
		}
	}
}

func (s *pinServer) HandleTaskListAnonymousPins() httprouter.Handle {
	// TODO: implement
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	}
}

func (s *pinServer) HandleTaskListUserPins() httprouter.Handle {
	// TODO: implement
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	}
}

func (s *pinServer) HandleTaskRegister() httprouter.Handle {
	clog := logging.WithFuncName()
	tmplPath := "templates/register.html"
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		clog.WithError(err).WithField("templatePath", tmplPath).Fatal("html template not loaded")
	}
	type View struct {
		Err   string
		Email string
	}
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		switch r.Method {
		case http.MethodGet:
			execTemplateLog(tmpl, w, View{}, clog.WithField("templatePath", tmplPath))
		case http.MethodPost:
			http.Error(w, "unsupported http method", http.StatusBadRequest)
		default:
			http.Error(w, "unsupported http method", http.StatusBadRequest)
		}
	}
}

func (s *pinServer) HandleTaskGetUserProfile() httprouter.Handle {
	// TODO: implement
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	}
}

func (s *pinServer) HandleAuthLogin() httprouter.Handle {
	// TODO: implement
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	}
}

func (s *pinServer) HandleAuthLogout() httprouter.Handle {
	// TODO: implement
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	}
}

// HandleAuthN is a middleware for authentication
func (s *pinServer) HandleAuthN(h httprouter.Handle) httprouter.Handle {
	// TODO: implement
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		h(w, r, ps)
	}
}

// HandleAuthZ is a middleware for authorization
func (s *pinServer) HandleAuthZ(h httprouter.Handle) httprouter.Handle {
	// TODO: implement
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		h(w, r, ps)
	}
}

// TODO: emit request latency metrics by implementing instrument middlewares

// -------------- utils --------------
func execTemplateLog(t *template.Template, w io.Writer, data interface{}, log *logrus.Entry) {
	if err := t.Execute(w, data); err != nil {
		log.WithError(err).Error("error executing html template")
	}
}
