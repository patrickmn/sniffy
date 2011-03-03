package proxy

import (
	"net"
	"net/http"
	"net/url"
	"testing"
)

type TestServer struct {
	Addr string
	ch   chan *http.Request
	l    net.Listener
}

func (ts *TestServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("X-This-Is", ts.Addr)
	ts.ch <- req
}

func (ts *TestServer) Close() {
	ts.l.Close()
}

func GetNoProxyClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: nil,
		},
	}

}

func GetTestServer() (*TestServer, error) {
	ts := &TestServer{
		ch: make(chan *http.Request),
	}
	srv := http.Server{
		Handler: ts,
	}
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}
	ts.Addr = l.Addr().String()
	go srv.Serve(l)
	return ts, nil
}

func GetTestServerGroup(n int) ([]*TestServer, error) {
	var err error
	servers := make([]*TestServer, n)
	for i := 0; i < n; i++ {
		s, err := GetTestServer()
		if err != nil {
			break
		}
		servers[i] = s
	}
	return servers, err
}

// TODO: Add tests for the different strategies
func TestLoadBalancer(t *testing.T) {
	var (
		hosts = [][]string{
			{"a.com"},
			{"b.com"},
			{"c.com"},
			{"abc.com", "def.com", "ghi.com"},
			{"a-bc.com", "d-ef.com", "g-hi.com"},
			{"ab-c.com", "de-f.com", "gh-i.com"},
			{"jkl.com", "mno.com", "pqr.com", "stu.com"},
			{"j-kl.com", "m-no.com", "p-qr.com", "s-tu.com"},
			{"jk-l.com", "mn-o.com", "pq-r.com", "st-u.com"},
		}
		routes = map[string][]string{}
		g, _   = GetTestServerGroup(len(hosts))
	)
	for i, v := range g {
		addr := v.Addr
		for _, ov := range hosts[i] {
			routes[ov] = []string{addr}
		}

		go func(ts *TestServer) {
			for {
				req := <-ts.ch
				if routes[req.Host][0] != ts.Addr {
					t.Fatal("Request dispatched to wrong server")
				}
			}
		}(v)
	}
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	pl, _ := net.ListenTCP("tcp", addr)
	lbHandler := NewHTTPLoadBalancer(routes, StrategyRandom)
	ps := &ProxyServer{
		Handler: lbHandler,
	}
	srv, _ := ps.getServer()
	go srv.Serve(pl)
	lbUrl, _ := url.Parse("http://" + pl.Addr().String())
	c := GetNoProxyClient()
	for k, v := range routes {
		go func(host string, should []string) {
			req, _ := http.NewRequest("GET", lbUrl.String()+"/", nil)
			req.Host = host
			res, err := c.Do(req)
			if err != nil {
				t.Error(err)
			}
			srvName := res.Header["X-This-Is"][0]
			for _, v := range should {
				if srvName == v {
					return
				}
			}
			t.Fatalf("Client seeking %s came to %s when it should have been one of %s", k, srvName, v)
		}(k, v)
	}
}
