package main

import (
	"net/http"
)

func (ws *WebServer) proxyDashboard(w http.ResponseWriter, req *http.Request) {
	ws.template(w, "proxy_dashboard", nil)
}

func (ws *WebServer) proxySettings(w http.ResponseWriter, req *http.Request) {
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
	ws.template(w, "proxy_settings", map[string]interface{}{
		"proxyservers": proxyServers,
		"ps":           ps,
	})
}
