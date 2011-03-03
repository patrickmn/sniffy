package main

import (
	"github.com/pmylund/sniffy/proxy"

	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func (ws *WebServer) auditorDashboard(w http.ResponseWriter, req *http.Request) {
	ws.template(w, "auditor_dashboard", nil)
}

func (ws *WebServer) auditorInterceptor(w http.ResponseWriter, req *http.Request) {
	var (
		ps  *proxyServer
		err error
	)
	psIdStr := req.FormValue("ps")
	if psIdStr != "" {
		ps, err = getActiveProxyServer(psIdStr)
		if err != nil {
			http.Error(w, "Could not get proxy server: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		ps = proxyServers[0]
	}
	ws.template(w, "auditor_interceptor", map[string]interface{}{
		"proxyservers": proxyServers,
		"ps":           ps,
	})
}

type auditorMakeRequestPayload struct {
	Emulate int64
	Type    string
	Form    map[string]string
}

func (ws *WebServer) auditorMakeRequest(w http.ResponseWriter, req *http.Request) {
	// errorMessage := func() {
	// 	http.Error(w, "Request not found", http.StatusNotFound)
	// }

	if req.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusBadRequest)
		return
	}

	// id, err := strconv.ParseInt(req.FormValue("id"), 10, 0)
	// if err != nil {
	// 	log.Println("Couldn't parse id for auditorMakeRequest:", err)
	// 	errorMessage()
	// 	return
	// }
	// r, err := getRequest(id, false)
	// if err != nil {
	// 	log.Println("Couldn't retrieve request", id, "-", err)
	// 	errorMessage()
	// 	return
	// }
	ps, err := getActiveProxyServer(req.FormValue("ps"))
	if err != nil {
		http.Error(w, "Could not get proxy server: "+err.Error(), http.StatusBadRequest)
		return
	}

	req.ParseForm()
	urlstr := req.FormValue("url")
	body := bytes.NewBufferString(req.FormValue("body"))
	method := req.FormValue("method")
	if method == "custom" {
		method = req.FormValue("methodcustom")
	}
	r, err := http.NewRequest(method, urlstr, body)
	if err != nil {
		http.Error(w, "Couldn't parse URL", http.StatusInternalServerError)
		return
	}
	r.Proto = req.FormValue("proto")

	for k, v := range req.Form {
		if strings.HasPrefix(k, "HEADER:") {
			r.Header[k[7:]] = v // "HEADER:" is 7 characters
		} else if strings.HasPrefix(k, "TRANSFERENCODING:") {
			// TODO
		}
	}
	r.Host = req.FormValue("host")
	r.RemoteAddr = req.RemoteAddr

	// func (*Client) Post(url string, bodyType string, body io.Reader)
	// r..Write(req.FormValue("data"))
	debug.Println("MakeRequest r:", r)
	go func() {
		bh := BlackHole{}
		s := &proxy.ProxySession{
			Request: r,
			Ps:      ps.ps,
			W:       bh,
		}
		ps.ps.Handler.HandleProxy(s)
	}()
	w.WriteHeader(http.StatusOK)
}

func (ws *WebServer) auditorJsonToggle(w http.ResponseWriter, req *http.Request) {
	// errorMessage := func() {
	// 	http.Error(w, "Could not toggle option", 500)
	// }
	ps, err := getActiveProxyServer(req.FormValue("ps"))
	if err != nil {
		http.Error(w, "Could not get proxy server: "+err.Error(), http.StatusBadRequest)
		return
	}

	switch req.FormValue("option") {
	default:
		http.Error(w, "Invalid option", http.StatusBadRequest)
		return
	case "logrequests":
		ps.toggleLogRequests()
	case "moderaterequests":
		ps.toggleModerateRequests()
	case "interceptssl":
		ps.toggleInterceptSSL()
	}
	w.WriteHeader(http.StatusOK)
}

// Writes a JSON payload with all requests since ?since=<UNIX> in ascending order
func (ws *WebServer) auditorJsonGetRequests(w http.ResponseWriter, req *http.Request) {
	var (
		rs           []requestEntry
		joinRes      = false
		errorMessage = func() {
			http.Error(w, "Couldn't get posts", http.StatusInternalServerError)
		}
	)

	ps, err := getActiveProxyServer(req.FormValue("ps"))
	if err != nil {
		http.Error(w, "Could not get proxy server: "+err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: Detect "summary" type and reduce the number of columns queried since they're not used
	if req.FormValue("type") == "full" {
		joinRes = true
	}
	since, err := strconv.ParseInt(req.FormValue("since"), 10, 0)
	if err != nil {
		log.Println("Couldn't parse 'since' integer in auditorJsonGetRequests:", err)
		errorMessage()
		return
	}
	if since == 0 {
		// TODO: Could do another SQL query for e.g. the 100th, then set since from that
		var temp []requestEntry
		// Need to get in DESC order, then reverse it, to get the most recent entries with LIMIT
		temp, err = getRequests(joinRes, "WHERE ps_id = $1 ORDER BY requests.time DESC LIMIT 100", ps.Id)
		num := len(temp)
		if num > 0 {
			rs = make([]requestEntry, num)
			oi := 0
			for i := num - 1; i >= 0; i-- {
				rs[oi] = temp[i]
				oi++
			}
		}
	} else {
		cached := false
		if last, found := dbCache.Get("lastRequestUpdate|int64"); found {
			if last.(int64) <= since {
				cached = true
				rs = []requestEntry{}
				err = nil
			}
		}
		if !cached {
			rs, err = getRequests(joinRes, "WHERE ps_id = $1 AND requests.time > $2 ORDER BY requests.time ASC", ps.Id, since)
		}
	}
	if err != nil {
		errorMessage()
		return
	}

	num := len(rs)
	if num > 0 {
		since = rs[num-1].Time
	}

	var queue []int64
	if ps.ModerateRequests {
		queue = ps.queue.List()
	}

	data := map[string]interface{}{
		"since": since,
		"rs":    rs,
		"queue": queue,
	}
	json, err := json.Marshal(data)
	if err != nil {
		errorMessage()
	} else {
		w.Header()["Pragma"] = []string{"no-cache"}
		w.Write(json)
	}
}

func (ws *WebServer) auditorJsonGetRequest(w http.ResponseWriter, req *http.Request) {
	errorMessage := func() {
		http.Error(w, "Couldn't get post", http.StatusInternalServerError)
	}
	id, err := strconv.ParseInt(req.FormValue("id"), 10, 0)
	if err != nil {
		log.Println("Couldn't parse 'id' integer in auditorJsonGetRequests:", err)
		errorMessage()
		return
	}
	r, err := getRequest(id, true)
	if err != nil {
		errorMessage()
		return
	}
	data := map[string]interface{}{
		"r": r,
	}
	json, err := json.Marshal(data)
	if err != nil {
		errorMessage()
	} else {
		w.Write(json)
	}
}

func (ws *WebServer) auditorJsonDeleteRequests(w http.ResponseWriter, req *http.Request) {
	errorMessage := func() {
		http.Error(w, "Couldn't delete posts", http.StatusInternalServerError)
	}

	ps, err := getActiveProxyServer(req.FormValue("ps"))
	if err != nil {
		http.Error(w, "Could not get proxy server: "+err.Error(), http.StatusBadRequest)
		return
	}

	_, err = db.Exec("DELETE FROM requests WHERE ps_id = $1", ps.Id) // cascades responses
	if err != nil {
		debug.Println("Failed to delete requests for ps_id", ps.Id, "- Error:", err)
		errorMessage()
		return
	}
	w.WriteHeader(http.StatusOK)
}
