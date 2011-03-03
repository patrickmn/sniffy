package main

import (
	"errors"
	"fmt"
	"net/http"
)

const (
	defaultWebServerPort = 8005
)

type WebServer struct {
	Host     string
	Port     uint16
	Language string
	CertFile string
	KeyFile  string
}

func (ws *WebServer) Run() {
	r := http.NewServeMux()
	r.Handle("/static/", http.FileServer(http.Dir("public")))
	r.Handle("/", ws)

	lstr := fmt.Sprintf("%s:%d", ws.Host, ws.Port)
	srv := &http.Server{
		Addr:    lstr,
		Handler: r,
	}
	debug.Println("Launching web interface on", lstr)
	err := srv.ListenAndServeTLS(ws.CertFile, ws.KeyFile)
	if err != nil {
		log.Fatal("Web interface ListenAndServeTLS:", err)
	}
}

func (ws *WebServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	debug.Println("Web request from", req.RemoteAddr+":", req.URL)
	switch req.URL.Path {
	default:
		http.Error(w, "Page not found", http.StatusNotFound)
	case "/":
		ws.front(w, req)
	case "/auditor":
		ws.auditorDashboard(w, req)
	case "/auditor/interceptor":
		ws.auditorInterceptor(w, req)
	case "/auditor/json/toggle":
		ws.auditorJsonToggle(w, req)
	case "/auditor/json/getrequest":
		ws.auditorJsonGetRequest(w, req)
	case "/auditor/json/getrequests":
		ws.auditorJsonGetRequests(w, req)
	case "/auditor/json/deleterequests":
		ws.auditorJsonDeleteRequests(w, req)
	case "/auditor/json/makerequest":
		ws.auditorMakeRequest(w, req)
	case "/proxy":
		ws.proxyDashboard(w, req)
	case "/proxy/settings":
		ws.proxySettings(w, req)
	}
}

func (ws *WebServer) template(w http.ResponseWriter, name string, data ...interface{}) error {
	set, found := templates[ws.Language]
	if !found {
		return errors.New("Invalid site language")
	}
	err := set.ExecuteTemplate(w, name, data)
	return err
}

func (ws *WebServer) front(w http.ResponseWriter, req *http.Request) {
	ws.template(w, "front", nil)
}

func NewWebServer(host string, port uint16) *WebServer {
	ws := WebServer{
		Host:     host,
		Port:     port,
		Language: "en",
		CertFile: config.webCertFile,
		KeyFile:  config.webKeyFile,
	}
	return &ws
}

type BlackHole struct{}

func (b BlackHole) Header() http.Header {
	return http.Header{}
}

func (b BlackHole) WriteHeader(int) {
}

func (b BlackHole) Write(data []byte) (int, error) {
	return len(data), nil
}
