package main

import (
	"github.com/pmylund/sniffy/acl"
	"github.com/pmylund/sniffy/common/queue"
	"github.com/pmylund/sniffy/proxy"

	"fmt"
	"net/http"
)

type proxyServer struct {
	Id               uint64
	Name             string
	CertFile         string
	KeyFile          string
	ModerateRequests bool
	InterceptSSL     bool
	LogRequests      bool
	queue            *queue.Queue
	ps               *proxy.ProxyServer
}

func (ps *proxyServer) HandleIntercept(w http.ResponseWriter, req *http.Request, origReq *http.Request) {
	s := &proxy.ProxySession{
		Request: req,
		Ps:      ps.ps,
		W:       w,
	}
	ps.HandleProxy(s)
}

func (ps *proxyServer) HandleProxy(s *proxy.ProxySession) {
	req := s.Request
	ch := make(chan int64)
	if ps.LogRequests {
		if ps.ModerateRequests {
			lid, err := saveRequest(ps, req)
			if err != nil {
				log.Println("Failed to save request:", req, "- Error:", err)
			}
			ch <- lid

			done := make(chan bool)
			ps.queue.Add(lid, done)
			<-done
			ps.queue.Remove(lid)
		} else {
			go func() {
				lid, err := saveRequest(ps, req)
				if err != nil {
					log.Println("Failed to save request:", req, "- Error:", err)
					return
				}
				ch <- lid
			}()
		}
	}
	err := s.GetResponse()
	if err != nil {
		log.Println("Error proxying request:", err)
		http.Error(s.W, "This page is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	res := s.Response
	if ps.LogRequests {
		go func() {
			lid := <-ch
			_, err := saveResponse(lid, res)
			if err != nil {
				log.Println("Failed to save request", lid, "response:", res, "- Error:", err)
			}
		}()
	}

	action := acl.ActionNone // TODO: Implement
	switch action {
	case acl.ActionDeny:
		s.W.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(s.W, "Access denied")
		return
	}
	if req.Method == "CONNECT" && ps.InterceptSSL {
		sslInterceptor.Intercept(s.W, req)
	} else {
		s.Do()
	}
}

func (ps *proxyServer) toggleLogRequests() bool {
	ps.LogRequests = !ps.LogRequests
	_, err := db.Exec("UPDATE proxyservers SET logrequests = $1 WHERE id = $2", ps.LogRequests, ps.Id)
	if err != nil {
		log.Println("Couldn't update proxyserver", ps.Id, "status, but instance's LogRequests toggled")
	}
	return ps.LogRequests
}

func (ps *proxyServer) toggleInterceptSSL() bool {
	ps.InterceptSSL = !ps.InterceptSSL
	_, err := db.Exec("UPDATE proxyservers SET interceptssl = $1 WHERE id = $2", ps.InterceptSSL, ps.Id)
	if err != nil {
		log.Println("Couldn't update proxyserver", ps.Id, "status, but instance's InterceptSSL toggled")
	}
	return ps.InterceptSSL
}

func (ps *proxyServer) toggleModerateRequests() bool {
	if ps.ModerateRequests {
		ps.queue.Flush()
	}
	ps.ModerateRequests = !ps.ModerateRequests
	_, err := db.Exec("UPDATE proxyservers SET moderaterequests = $1 WHERE id = $2", ps.ModerateRequests, ps.Id)
	if err != nil {
		log.Println("Couldn't update proxyserver", ps.Id, "status, but instance's ModerateRequests toggled")
	}
	return ps.ModerateRequests
}
