package proxy

import (
	"github.com/pmylund/sniffy/common"

	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

const (
	version           = "0.1"
	DefaultProxyAgent = "Sniffy/" + version
)

var (
	DefaultConnectResponseHeader = []byte("HTTP/1.1 200 Connection established\r\nProxy-agent: " + DefaultProxyAgent + "\r\n\r\n")
)

type ProxyHandler interface {
	HandleProxy(*ProxySession)
}

type ProxyServer struct {
	Host                  string
	Port                  uint16
	Handler               ProxyHandler
	UseEnvProxy           bool
	ProxyLoopTestUrl      string
	ProxyAgent            string
	ConnectResponseHeader []byte
	client                *http.Client
}

func (ps *ProxyServer) ListenAndServe() error {
	srv, err := ps.getServer()
	if err != nil {
		return err
	}
	err = srv.ListenAndServe()
	return err
}

func (ps *ProxyServer) ListenAndServeTLS(certFile, keyFile string) error {
	srv, err := ps.getServer()
	if err != nil {
		return err
	}
	err = srv.ListenAndServeTLS(certFile, keyFile)
	return err
}

func (ps *ProxyServer) getServer() (*http.Server, error) {
	var (
		noEnvProxy bool
		err        error
	)
	if ps.UseEnvProxy && ps.ProxyLoopTestUrl != "" {
		noEnvProxy, err = CheckProxyLoop(ps.Port, ps.ProxyLoopTestUrl)
		if err != nil {
			return nil, fmt.Errorf("Failed to check for proxy loop: %s", err)
		}
	} else {
		noEnvProxy = true
	}
	ps.client = new(http.Client)
	if noEnvProxy {
		ps.client.Transport = &http.Transport{Proxy: nil}
	} else {
		ps.client.Transport = http.DefaultTransport
	}
	if ps.ProxyAgent != "" {
		ps.ConnectResponseHeader = []byte("HTTP/1.1 200 Connection established\r\nProxy-agent: " + ps.ProxyAgent + "\r\n\r\n")
	} else {
		ps.ConnectResponseHeader = DefaultConnectResponseHeader
	}
	srv := http.Server{
		Addr:    fmt.Sprintf("%s:%d", ps.Host, ps.Port),
		Handler: ps,
	}
	return &srv, nil
}

// should be allowedproxyhost middleware
// } else {
// 	http.Error(w, "This client is not allowed to use this proxy server", http.StatusServiceUnavailable)
// }

func (ps *ProxyServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	req.URL.Scheme = strings.ToLower(req.URL.Scheme) // Curl does "HTTP://" for some reason
	s := &ProxySession{
		Request: req,
		Ps:      ps,
		W:       w,
	}
	ps.Handler.(ProxyHandler).HandleProxy(s)
}

// for dial err: http.Error(w, "A connection to the remote host could not be established", http.StatusServiceUnavailable)
func (ps *ProxyServer) ProxyCONNECT(w http.ResponseWriter, req *http.Request) error {
	addr := req.URL.String()
	dest, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("Error establishing SSL connection to %s: %s", addr, err)
	}
	c, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return fmt.Errorf("Error hijacking HTTP request: %s", err)
	}
	c.Write(ps.ConnectResponseHeader)
	go func() {
		defer c.Close()
		io.Copy(c, dest)
	}()
	go func() {
		defer dest.Close()
		io.Copy(dest, c)
	}()
	return nil
}

type ProxySession struct {
	Request  *http.Request
	Response *http.Response
	Ps       *ProxyServer
	W        http.ResponseWriter
}

// GetResponse performs the client's request, setting s.Response, and returning an error,
// if any. If the request type is CONNECT (SSL proxy), GetResponse sets s.Response to
// 200 Connection established, the last message the client receives before an SSL tunnel
// is created.
func (s *ProxySession) GetResponse() error {
	if s.Request.Method == "CONNECT" {
		// This is what sniffy/proxy will tell the client before it establishes
		// the SSL tunnel.
		s.Response = &http.Response{
			Status:     "200 Connection established",
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header: http.Header{
				"Proxy-agent": []string{s.Ps.ProxyAgent},
			},
			ContentLength:    0,
			TransferEncoding: []string{},
			// Close
			// Trailer
			Request: s.Request,
		}
		return nil
	}
	proxyHeader, found := s.Request.Header["Proxy-Connection"]
	if found {
		_, exists := s.Request.Header["Connection"]
		if !exists {
			s.Request.Header["Connection"] = proxyHeader
		}
		delete(s.Request.Header, "Proxy-Connection")
	}
	res, err := s.Ps.client.Transport.RoundTrip(s.Request)
	s.Response = res
	return err
}

// Do sends the response to the client, performing GetResponse() if it hasn't already been.
// If the request type is CONNECT, Do opens a tunnel to the destination host, and returns
// an error, if any, when the tunnel is closed.
func (s *ProxySession) Do() error {
	if s.Request.Method == "CONNECT" {
		return s.Ps.ProxyCONNECT(s.W, s.Request)
	}
	if s.Response == nil {
		err := s.GetResponse()
		if err != nil {
			return fmt.Errorf("Could not perform GetResponse: %s", err)
		}
	}
	h := s.W.Header()
	for k, v := range s.Response.Header {
		h[k] = v
	}
	// TEMP: Remove "non-proper" headers so HTTP lib doesn't complain
	if s.Response.StatusCode == http.StatusNotModified {
		for _, v := range []string{"Content-Type", "Content-Length", "Transfer-Encoding"} {
			delete(h, v)
		}
	}
	s.W.WriteHeader(s.Response.StatusCode)
	if s.Response.Body != nil {
		io.Copy(s.W, s.Response.Body)
	}
	return nil
}

// TODO: Add
//       1. HTTP Reverse Proxy Server
//       2. HTTPS Reverse Proxy Server / SSL termination proxy

type EnvProxyTest struct {
	Random string
	Loop   bool
}

func (t *EnvProxyTest) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.FormValue("sniffyEnvProxyTestRandom") == t.Random {
		t.Loop = true
	}
	w.WriteHeader(http.StatusServiceUnavailable) // Just in case clients send requests
}

// Connects to a URL (preferably external) to determine if we reach ourselves, i.e. this
// machine is its own proxy server, and returns true if it is. If there is an error running
// a dummy server on the designated port, that is returned.
func CheckProxyLoop(port uint16, url string) (bool, error) {
	pstr := fmt.Sprintf("127.0.0.1:%d", port)
	testRandom := common.RandomString(32)
	testHandler := &EnvProxyTest{
		Random: testRandom,
	}
	testSrv := &http.Server{
		Addr:    pstr,
		Handler: testHandler,
	}
	l, err := net.Listen("tcp", pstr)
	if err != nil {
		return false, fmt.Errorf("Couldn't listen on %s for EnvProxyTest", pstr)
	}
	defer l.Close() // this kills testSrv
	go testSrv.Serve(l)
	http.DefaultClient.Get(url + "?sniffyEnvProxyTestRandom=" + testRandom)
	if testHandler.Loop {
		return true, nil
	}
	return false, nil
}
