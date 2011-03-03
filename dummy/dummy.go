package dummy

import (
	"fmt"
	// "io/ioutil"
	"net/http"
)

// TODO: Make a dashboard for dummy server testing, modify JSON getrequests
//       to take some sort of type parameter to get dummy requests instead
//       of proxy requests. Store the requests in the same table--differentiate
//       with some kind of type

type DummyServer struct {
	Id       uint64
	Name     string
	Port     uint16
	CertFile string
	KeyFile  string
}

func (ds *DummyServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// var bodyStr string
	// body, err := ioutil.ReadAll(req.Body)
	// if err != nil {
	// 	bodyStr = "Couldn't read: " + err.Error()
	// } else {
	// 	bodyStr = string(body)
	// }
	// if ds.SSL {
	// 	tStr = "HTTPS"
	// } else {
	// 	tStr = "HTTP"
	// }
	//log.Println(tStr, "dummy server", ds.Name, "request:", req, "- Body:", bodyStr)
	w.WriteHeader(http.StatusOK)
}

func (ds *DummyServer) Run() error {
	r := http.NewServeMux()
	r.Handle("/", ds)

	lstr := fmt.Sprintf(":%d", ds.Port)
	srv := &http.Server{
		Addr:    lstr,
		Handler: r,
	}
	var err error
	if ds.CertFile != "" && ds.KeyFile != "" {
		err = srv.ListenAndServeTLS(ds.CertFile, ds.KeyFile)
	} else {
		err = srv.ListenAndServe()
	}
	if err != nil {
		return err
	}
	return nil
}

func NewDummyServer(port uint16) *DummyServer {
	ds := DummyServer{
		Port: port,
	}
	return &ds
}
