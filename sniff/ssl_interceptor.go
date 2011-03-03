package sniff

import (
	"github.com/pmylund/sniffy/cert"
	"github.com/pmylund/sniffy/proxy"

	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
)

// TODO: Expand so it can connect with anything; not just ProxyServer

type InterceptHandler interface {
	HandleIntercept(http.ResponseWriter, *http.Request, *http.Request)
}

type SSLInterceptor struct {
	Handler               InterceptHandler
	GenerateHostCerts     bool
	HostCertFolder        string
	ConnectResponseHeader []byte
	caKeyPair             *tls.Certificate
	caParentCert          *x509.Certificate
	keyPairCache          map[string]*tls.Certificate
	mu                    *sync.Mutex
}

type requestInterceptor struct {
	Addr    string
	OrigReq *http.Request
	si      *SSLInterceptor
	server  *http.Server
}

func (si *SSLInterceptor) Intercept(w http.ResponseWriter, req *http.Request) error {
	addr := req.URL.String()
	c, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return fmt.Errorf("Error hijacking CONNECT request: %s", err)
	}
	ri := &requestInterceptor{
		OrigReq: req,
		Addr:    addr,
		si:      si,
	}
	srv := &http.Server{
		Handler: ri,
	}
	// TODO: Use tls.Config.NameToCertificate to map domain:cert, and run only one intercept srv
	config := &tls.Config{
		Rand:       rand.Reader,
		NextProtos: []string{"http/1.1"},
	}
	split := strings.Split(addr, ":")
	keypair, err := si.GetHostKeyPair(split[0])
	if err != nil {
		return fmt.Errorf("Couldn't generate interceptor key pair for %s: %s", split[0], err)
	}
	keypair.Certificate = append(keypair.Certificate, si.caKeyPair.Certificate...)
	// TODO: 1. the complete issuer chain isn't included?
	//       2. tls.Certificate can just contain the certs for all domains?
	//       3. emulate the SSL certificate of the destination? Expiry, etc.
	//       4. impossible to avoid e.g. Firefox built-in certs for mail.google.com, etc.?
	config.Certificates = []tls.Certificate{
		*keypair,
		// *si.caKeyPair,
	}
	// TODO: 1. Change this so we don't initiate another connection
	//          We should be able to accomplish the same without it
	//       2. Intercepted connections have 127.0.0.1 RemoteAddr
	//          because of this
	l, err := tls.Listen("tcp", "127.0.0.1:0", config)
	if err != nil {
		return fmt.Errorf("Couldn't create SSL interceptor TLS listener: %s", err)
	}
	oc, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		return fmt.Errorf("Couldn't create SSL interceptor connection: %s", err)
	}
	if si.ConnectResponseHeader != nil {
		c.Write(si.ConnectResponseHeader)
	} else {
		c.Write(proxy.DefaultConnectResponseHeader)
	}
	go func() {
		defer c.Close()
		defer l.Close() // this kills the HTTP server (srv)
		io.Copy(c, oc)
	}()
	go func() {
		defer oc.Close()
		io.Copy(oc, c)
	}()
	srv.Serve(l)
	return nil
}

// This is a question of removing the middleware from the ps
// func (si *SSLInterceptor) toggleInterceptSSL() bool {
// 	ps.mu.Lock()
// 	defer ps.mu.Unlock()
// 	si.InterceptSSL = !ps.InterceptSSL
// }

func (si *SSLInterceptor) GetHostKeyPair(cn string) (*tls.Certificate, error) {
	si.mu.Lock()
	defer si.mu.Unlock()
	keypair, found := si.keyPairCache[cn]
	if found {
		return keypair, nil
	}
	keypair, err := cert.GetOrGenerateKeyPair(path.Join(si.HostCertFolder, cn+"_cert.pem"), path.Join(si.HostCertFolder, cn+"_key.pem"), cn, []string{"Sniffy"}, false, si.caParentCert)
	if err == nil {
		si.keyPairCache[cn] = keypair
	}
	return keypair, err
}

func (ri *requestInterceptor) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	req.URL.Scheme = "https"
	req.URL.Host = ri.Addr
	ri.si.Handler.(InterceptHandler).HandleIntercept(w, req, ri.OrigReq)
}

func NewSSLInterceptor(handler InterceptHandler, caCertFile, caKeyFile string) (*SSLInterceptor, error) {
	si := SSLInterceptor{
		Handler:           handler,
		GenerateHostCerts: true,               // TODO: TEMP
		HostCertFolder:    "cert/interceptor", // TODO: TEMP
		keyPairCache:      map[string]*tls.Certificate{},
		mu:                &sync.Mutex{},
	}
	_, err := cert.GetOrGenerateKeyPair(caCertFile, caKeyFile, "interceptor.sniffy.local", []string{"Sniffy"}, true, nil)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get or generate interceptor CA key pair: %s", err)
	}
	caKeyPair, err := tls.LoadX509KeyPair(caCertFile, caKeyFile)
	if err != nil {
		return nil, fmt.Errorf("Couldn't load interceptor CA key pair: %s", err)
	}
	si.caKeyPair = &caKeyPair
	si.caParentCert, err = x509.ParseCertificate(caKeyPair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("Couldn't parse interceptor CA key pair: %s", err)
	}
	return &si, nil
}
