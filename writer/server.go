package main

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	hr "github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"wuyrush.io/pin/common/logging"
	se "wuyrush.io/pin/errors"
	md "wuyrush.io/pin/models"
)

const (
	envWriterServerAddr = "PIN_WRITER_SERVER_ADDR"
	envTrapName         = "PIN_TRAP_NAME"
)

// writer handles write traffic of pin application. Multiple writers form the service
// component to handle the application's write operations
type writer struct {
	R       *hr.Router
	PinDAO  PinDAO
	UserDAO UserDAO
}

func (wrt *writer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wrt.R.ServeHTTP(w, r)
}

func serve() error {
	s, err := setup()
	if err != nil {
		return err
	}
	return s.ListenAndServe()
}

func setup() (*http.Server, error) {
	logging.SetupLog("pin-writer")
	viper.AutomaticEnv()
	wrt := &writer{PinDAO: dummyPinDAO{}}
	if err := wrt.SetupRoutes(); err != nil {
		return nil, err
	}
	return &http.Server{
		Addr:    viper.GetString(envWriterServerAddr),
		Handler: wrt,
		// TODO: tweak setups
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}, nil
}

func (wrt *writer) SetupRoutes() error {
	r := hr.New()
	trap := viper.GetString(envTrapName)
	tmplCreatePin, err := template.ParseFiles("templates/create-pin.html")
	if err != nil {
		log.Error("error loading html template: create-pin")
		return err
	}
	r.POST("/create", wrt.HandleTaskCreatePin(tmplCreatePin, trap))
	r.PATCH("/access/:pid", wrt.HandleTaskChangePinAccessMode)
	r.DELETE("/delete/:pid", wrt.HandleTaskDeletePin)
	r.POST("/register", wrt.HandleAuthRegister)
	r.POST("/login", wrt.HandleAuthLogin)
	r.POST("/logout", wrt.HandleAuthLogout)
	r.PUT("/user", wrt.HandleAuthUpdateUserProfile)
	// TODO: at the moment these handles only service static html pages, which is irrelevant to application
	// logic. Therefore why not have a dedicated web server like Nginx service such requests so that
	// application server can dedicated to servicing requests which need business logic?
	// r.GET("/create", w.HandleTaskGetCreatePin)
	// r.GET("/register", w.HandleAuthGetRegister)
	// r.GET("/login", w.HandleAuthGetLogin)
	// r.GET("/logout", w.HandleAuthLoggedout)

	// static assets
	// r.Handler(
	// 	http.MethodGet,
	// 	"/static/*filepath",
	// 	http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	// )
	wrt.R = r
	return nil
}

/*
Alg:
	1. Bot detection
	2. Process(and validate) form input related to pin
	3. Save pin data
	4. Return response to client

*/
func (wrt *writer) HandleTaskCreatePin(tmpl *template.Template, trap string) hr.Handle {
	type View struct {
		Err, URL, Title, Body string
	}
	return func(w http.ResponseWriter, r *http.Request, p hr.Params) {
		reader, err := r.MultipartReader()
		if err != nil {
			log.WithError(err).Error("error getting multiform reader")
			resp(w, http.StatusBadRequest, tmpl, View{Err: "error reading form data"})
			return
		}
		var pin md.Pin
		perr := processParts(reader,
			detectSpam(trap),
			parsePin(&pin),
		)
		if perr != nil {
			switch perr.Code {
			case se.ErrCodeSpam:
				log.WithField("remoteAddr", r.RemoteAddr).Warning("spam attempt detected. Rejecting request")
				// TODO: save remote address for further restrictions
			}
			log.WithError(perr).Error("error parsing pin data from html form")
			resp(w, perr.StatusCode(), tmpl, View{Err: perr.Error()})
			return
		}
		if err := wrt.PinDAO.Create(&pin); err != nil {
			resp(w, err.StatusCode(), tmpl, View{Err: perr.Error()})
			return
		}
		resp(w, http.StatusOK, tmpl, nil)
	}
}

func (wrt *writer) HandleTaskChangePinAccessMode(w http.ResponseWriter, r *http.Request, p hr.Params) {
	log.Info("hit FlipPinAccessMode")
}

func (wrt *writer) HandleTaskDeletePin(w http.ResponseWriter, r *http.Request, p hr.Params) {
	log.Info("hit DeletePin")
}

func (wrt *writer) HandleAuthRegister(w http.ResponseWriter, r *http.Request, _ hr.Params) {
	log.Info("hit Register")

}

func (wrt *writer) HandleAuthLogin(w http.ResponseWriter, r *http.Request, _ hr.Params) {
	log.Info("hit Login")

}

func (wrt *writer) HandleAuthLogout(w http.ResponseWriter, r *http.Request, _ hr.Params) {
	log.Info("hit Logout")

}

func (wrt *writer) HandleAuthUpdateUserProfile(w http.ResponseWriter, r *http.Request, _ hr.Params) {
	log.Info("hit UpdateUserProfile")
}

func resp(w http.ResponseWriter, statusCode int, tmpl *template.Template, data interface{}) {
	w.WriteHeader(statusCode)
	if err := tmpl.Execute(w, data); err != nil {
		log.WithField("templateName", tmpl.Name()).WithError(err).Error("error executing response template")
	}
}

// TODO: implement
func PanicRecover(h hr.Handle) hr.Handle {
	return h
}

// TODO: implement
func RateLimiter(h hr.Handle, burst int, rate float64) hr.Handle {
	return h
}

// TODO: implement
func HSTSer(h hr.Handle) hr.Handle {
	return h
}

type PinDAO interface {
	Create(p *md.Pin) *se.Err
}

type dummyPinDAO struct{}

func (dao dummyPinDAO) Create(p *md.Pin) *se.Err { return nil }

type UserDAO interface {
	Register(u *md.User) se.Err
}

/*
	Utilities to stream-process http multipart form data.

	NOTE the order in which parts get processed is the same as the tree order in which corresponding
	entries are placed in the html DOM. See
	https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#multipart-form-data

	NOTE It is the service that dictates the form processing logic instead of client. Thus the service
	doesn't need to care about the exact number of parts client sends to it. It must only considers the
	parts it cares. This prevents the service from blindly reading and processing whatever data sent
	from the client(e.g., super large requests with unnecessary parts), which is what http.ParseForm does.

	This means we ditch processing approach like following:
	for {
		p, err := r.NextPart()
		// handle err
		workOn(p)
	}
*/
func processParts(r *multipart.Reader, ps ...partProcessor) *se.Err {
	for _, p := range ps {
		if err := p(r); err != nil {
			return err
		}
	}
	return nil
}

type partProcessor func(*multipart.Reader) *se.Err

// detectSpam returns a PartProcessor to detect naive bot attempts by checking whether the honeypot form field,
// which is designed to be invisible to human users, is set or not.
// NOTE It stumbles on more sophisticated and dedicated bot attempts.
func detectSpam(trap string) partProcessor {
	return func(r *multipart.Reader) *se.Err {
		cerr := se.NewBadInput("error processing form data")
		// by convention bot trap form field is placed at the beginning
		part, err := r.NextPart()
		if part != nil {
			defer part.Close()
		}
		if err != nil {
			msg := "error reading next part from multiform reader"
			if err == io.EOF {
				msg = "spam trap not found"
			}
			log.WithError(err).Error(msg)
			return cerr.WithCause(err)
		}
		if name := part.FormName(); name != trap {
			log.Errorf("spam trap not found. Got unexpected form name %s", name)
			return cerr
		}
		if _, err := ioutil.ReadAll(NewLimitReader(part, 0)); err != nil {
			if v, ok := err.(*se.Err); ok && v.Code == se.ErrCodeOversized {
				return se.NewSpam()
			}
			log.WithError(err).Error("error reading value of spam trap")
			return cerr.WithCause(err)
		}
		return nil
	}
}

func parsePin(pin *md.Pin) partProcessor {
	type partProcCfg struct {
		FormName   string                        // form field name to process
		LimitBytes int64                         // form field value size limit in bytes
		Process    func(string, *md.Pin) *se.Err // logic to parse and validate form field value
	}
	// generate logic to process individual non-file form field
	gen := func(cfg partProcCfg) partProcessor {
		return func(r *multipart.Reader) *se.Err {
			part, err := r.NextPart()
			if err != nil {
				return se.NewBadInput("error reading form part").WithCause(err)
			} else if part.FormName() != cfg.FormName {
				return se.NewBadInput(fmt.Sprintf("failed to find form field name %s", cfg.FormName))
			}
			lr := NewLimitReader(part, cfg.LimitBytes)
			bytes, err := ioutil.ReadAll(lr)
			if err != nil {
				if v, ok := err.(*se.Err); ok && v.Code == se.ErrCodeOversized {
					return v.WithMsg(fmt.Sprintf("got oversized data for field field %s", cfg.FormName))
				}
				return se.NewBadInput(fmt.Sprintf("failed to read value of form field %s", cfg.FormName)).WithCause(err)
			}
			if err := cfg.Process(string(bytes), pin); err != nil {
				return err
			}
			return nil
		}
	}
	return func(r *multipart.Reader) *se.Err {
		if err := processParts(r,
			gen(partProcCfg{
				FormName:   "title",
				LimitBytes: 1 << 8,
				Process: func(s string, pin *md.Pin) *se.Err {
					pin.Title = s
					return nil
				},
			}),
			gen(partProcCfg{
				FormName:   "private",
				LimitBytes: 5,
				Process: func(s string, pin *md.Pin) *se.Err {
					private, err := strconv.ParseBool(s)
					if err != nil {
						return se.NewBadInput("invalid access mode value").WithCause(err)
					}
					pin.AccessMode = md.AccessModePublic
					if private {
						pin.AccessMode = md.AccessModePrivate
					}
					return nil
				},
			}),
			gen(partProcCfg{
				FormName:   "read-only-once",
				LimitBytes: 5,
				Process: func(s string, pin *md.Pin) *se.Err {
					readOnceOnly, err := strconv.ParseBool(s)
					if err != nil {
						return se.NewBadInput("invalid read-once-only value").WithCause(err)
					}
					pin.ReadAndBurn = false
					if readOnceOnly {
						pin.ReadAndBurn = true
					}
					return nil
				},
			}),
			gen(partProcCfg{
				FormName:   "body",
				LimitBytes: 1 << 18, // TODO: read from env var?
				Process: func(s string, pin *md.Pin) *se.Err {
					pin.Body = s
					return nil
				},
			}),
			genFilesProc(r, pin),
		); err != nil {
			return err
		}
		return nil
	}
}

// TODO: genFilesProc generates logic to process files in multipart form
func genFilesProc(r *multipart.Reader, pin *md.Pin) partProcessor {
	return func(r *multipart.Reader) *se.Err { return nil }
}

// LimitReader dedicates to detecting oversized data
type LimitReader struct {
	R io.Reader // underlying reader
	n int64     // max bytes remaining
}

func NewLimitReader(r io.Reader, max int64) *LimitReader {
	// idea: try reading one more byte above given limit from given reader. If there is no more data left from r
	// then r shall return (0, io.EOF), otherwise it can return more bytes and potentially a non-nil error. We
	// take the risk of rejecting a legit request when the last read attempt returns non-io.EOF error.
	// skip overflow check since we won't read such huge amount of data in practice
	return &LimitReader{R: r, n: max + 1}
}

func (r *LimitReader) Read(p []byte) (n int, err error) {
	// tweak based on io.LiimitReader.Read
	if int64(len(p)) > r.n {
		p = p[0:r.n]
	}
	n, err = r.R.Read(p)
	r.n -= int64(n)
	if r.n <= 0 {
		return 0, se.NewOversized()
	}
	return
}
