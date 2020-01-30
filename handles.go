package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/segmentio/ksuid"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	goodForMin                = time.Minute * 1
	goodForMax                = time.Hour * 24
	respMsgErrPinInfo         = "error pinning info"
	errMsgRequestBodyTooLarge = "request body too large"
	errMsgPinNotFound         = "pin not found"
)

func (s *pinServer) HandleTaskCreatePin() httprouter.Handle {
	type createPinResponse struct {
		PinID  string    `json:"pinID"`
		Expiry time.Time `json:"expiry"`
	}
	var (
		once           sync.Once
		tmpl           *template.Template
		tmplPath       = "templates/create_pin.html"
		maxReqBodySize = viper.GetInt64(envReqBodySizeMaxByte)
	)
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		switch r.Method {
		case http.MethodGet:
			once.Do(func() {
				t, err := template.ParseFiles(tmplPath)
				if err != nil {
					log.WithError(err).Errorf("error parsing html template %s", tmplPath)
					return
				}
				tmpl = t
			})
			if tmpl == nil {
				log.Errorf("html template %s is not loaded", tmplPath)
				http.Error(w, "error serving web page", http.StatusInternalServerError)
				return
			}
			if err := tmpl.Execute(w, nil); err != nil {
				log.WithError(err).WithFields(logrus.Fields{
					"handleName": "HandleTaskCreatePin",
					"httpMethod": "GET",
				}).Error("error executing html template")
			}
		case http.MethodPost:
			// TODO: if we got error while validating input, or any error after the pin data is already built,
			// we shall return the pin data back to the customer, along with the error we encounter, so that
			// customer can retry without re-typing all the information
			// Meanwhile in case of error we should always direct the user back to the create-pin page so that
			// they can retry

			// 0. limit request size and parse request form
			r.Body = http.MaxBytesReader(w, r.Body, maxReqBodySize)
			if err := r.ParseMultipartForm(128); err != nil {
				http.Error(w, "got malformed form data", http.StatusBadRequest)
				return
			}
			// 1. assemble and validate pin data
			p, err := s.buildPin(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			clog := log.WithField("pinID", p.ID)
			p.CreationTime = time.Now()
			pinExpiry := p.CreationTime.Add(p.GoodFor)
			// 5. save pin data
			if err := s.PS.Save(p); err != nil {
				// TODO: handle
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// 6. save pin attachment data
			for _, fh := range r.MultipartForm.File["attachments"] {
				ref := p.Attachments[fh.Filename]
				f, err := fh.Open()
				if err != nil {
					http.Error(w, "error reading uploaded file content", http.StatusInternalServerError)
					return
				}
				defer f.Close()
				if err := s.FS.Save(ref, f); err != nil {
					// TODO: handle
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			// 7. return pin id for web frontend to assemble final url
			resp := &createPinResponse{
				PinID:  p.ID,
				Expiry: pinExpiry,
			}
			respBytes, merr := json.Marshal(resp)
			if merr != nil {
				clog.WithError(merr).Error("error marshalling response")
				http.Error(w, respMsgErrPinInfo, http.StatusInternalServerError)
				return
			}
			if _, err := w.Write(respBytes); err != nil {
				clog.WithError(err).Error("error writing response to client")
			}
		}
	}
}

func (s *pinServer) buildPin(r *http.Request) (*pin, *pinErr) {
	// 3. generate pin id
	pinKsuid, err := ksuid.NewRandom()
	if err != nil {
		log.WithError(err).Error("fail to generate pin id")
		return nil, errServiceFailure(respMsgErrPinInfo).WithCause(err)
	}
	p := &pin{
		ID:    pinKsuid.String(),
		Title: r.FormValue("title"),
		Note:  r.FormValue("note"),
	}
	p.Mode = accessModePublic
	if r.FormValue("private") == "true" {
		p.Mode = accessModePrivate
	}
	p.ReadAndBurn = false
	if r.FormValue("read-and-burn") == "true" {
		p.ReadAndBurn = true
	}
	goodFor, err := time.ParseDuration(r.FormValue("good-for"))
	if err != nil {
		return nil, errBadRequest("error parsing good-for period").WithCause(err)
	}
	if goodFor < goodForMin || goodFor > goodForMax {
		return nil, errBadRequest("good-for period out of range")
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
	var (
		once     sync.Once
		tmpl     *template.Template
		tmplPath = "templates/get_pin.html"
	)
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// 0. validate input pin id
		pinID := ps.ByName("id")
		clog := log.WithField("pinID", pinID)
		if _, err := ksuid.Parse(pinID); err != nil {
			clog.WithError(err).Error("got invalid pin ID")
			http.Error(w, "pin not found", http.StatusNotFound)
		}
		// 1. get pin data from pin store
		p, err := s.PS.Get(pinID)
		if err != nil {
			switch err.Code {
			case errCodeNotFound:
				http.Error(w, errMsgPinNotFound, http.StatusNotFound)
			default:
				http.Error(w, "service error getting pin info", http.StatusInternalServerError)
			}
			return
		}
		// TODO: 2. check if the pin is accessible to the requester or not <- this should be done by pinStore
		// TODO: 3. update view count
		// 4. assemble response and return
		once.Do(func() {
			t, err := template.ParseFiles(tmplPath)
			if err != nil {
				clog.WithError(err).Errorf("error parsing html template %s", tmplPath)
				return
			}
			tmpl = t
		})
		if tmpl == nil {
			clog.Errorf("html template %s is not loaded", tmplPath)
			http.Error(w, "error serving web page", http.StatusInternalServerError)
		}
		pinView := &struct {
			*pin
			FilenameToURL map[string]string
		}{
			pin:           p,
			FilenameToURL: map[string]string{},
		}
		for fn := range p.Attachments {
			url := fmt.Sprintf("/pin/%s/attachment/%s", p.ID, url.PathEscape(fn))
			pinView.FilenameToURL[fn] = url
		}
		if err := tmpl.Execute(w, pinView); err != nil {
			clog.WithError(err).Errorf("error executing html template %s", tmplPath)
		}
	}
}

// HandleTaskDeletePin handles request to remove a specified pinned information. Note
func (s *pinServer) HandleTaskDeletePin() httprouter.Handle {
	// TODO: implement
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	}
}

func (s *pinServer) HandleTaskGetPinAttachment() httprouter.Handle {
	// TODO: implement
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

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
	// TODO: implement
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

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
