package proxy

import (
	"fmt"
	"math/rand"
	"net/http"
)

const (
	StrategyFirst = iota
	StrategyRandom
	StrategyRoundrobin
	StrategyFair
)

type LoadBalancerHandler struct {
	Strategy int
	Routes   map[string][]string
	dist     map[string]chan string
	ps       *ProxyServer
}

func (lb *LoadBalancerHandler) HandleProxy(s *ProxySession) {
	dest, err := lb.chooseHost(s.Request.Host)
	if err != nil {
		http.Error(s.W, "Not found", http.StatusNotFound)
		return
	}
	s.Request.URL.Scheme = "http"
	s.Request.URL.Host = dest
	s.Request.Header.Add("X-Forwarded-For", s.Request.RemoteAddr)
	s.Do()
}

func (lb *LoadBalancerHandler) chooseHost(k string) (string, error) {
	ds, found := lb.Routes[k]
	if !found {
		return "", fmt.Errorf("Host not found")
	}
	var d string
	switch lb.Strategy {
	case StrategyFirst:
		d = ds[0]
	case StrategyRandom:
		d = ds[rand.Intn(len(ds))]
	case StrategyRoundrobin:
		d = <-lb.dist[k]
	}
	return d, nil
}

func RoundRobinDispatcher(routes []string, dist chan<- string) {
	for {
		for _, v := range routes {
			dist <- v
		}
	}
}

func NewHTTPLoadBalancer(r map[string][]string, s int) *LoadBalancerHandler {
	lb := &LoadBalancerHandler{
		Routes:   r,
		Strategy: s,
	}
	if s == StrategyRoundrobin {
		lb.dist = map[string]chan string{}
		for k, v := range r {
			dist := make(chan string, 2)
			go RoundRobinDispatcher(v, dist)
			lb.dist[k] = dist
		}
	}
	return lb
}
