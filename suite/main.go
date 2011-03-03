package main

import (
	"github.com/pmylund/sniffy/cert"
	"github.com/pmylund/sniffy/sniff"

	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

const (
	version = "0.1"
	banner  = `
  _________      .__   _____   _____        .~~_!! |\/|    ____
 /   _____/ ____ |__|_/ ____\_/ ____\___.__.    .__.. \   /\  /
 \_____  \ /    \|  |\   __\ \   __\<   |  |     \_   /__/  \/
 /        \   |  \  | |  |    |  |   \___  |     _/  __   __/
/_______  /___|  /__| |__|    |__|   / ____|    /___/____/
        \/     \/                    \/
`
)

var (
	DEBUG          bool
	config         *SniffyConfig
	configFile     = "db.cfg"
	logFile        = "sniffy.log"
	db             *sql.DB
	proxyServers   []*proxyServer
	dummyServers   []*dummyServer
	sslInterceptor *sniff.SSLInterceptor
)

func main() {
	var verbose *bool = flag.Bool("v", false, "show additional information")
	flag.Parse()
	if *verbose {
		DEBUG = true
	}
	fmt.Println(banner)
	boot()
}

func boot() {
	var err error
	initLogger(logBoot)

	config, err = loadConfig()
	if err != nil {
		setup()
		return
	}
	err = connectDB()
	if err != nil {
		log.Fatal("Couldn't open database:", err)
	}
	err = initDB()
	if err != nil {
		log.Fatalln("Failed to initialize database:", err, "\r\nIf the database settings have changed, please delete "+configFile+", then re-run\r\nSniffy to define new database connection settings. (Existing information in\r\nthe database, if it exists, will not be removed.)")
	}
	err = loadSettings()
	if err != nil {
		log.Fatalln("Failed to load settings from database:", err)
	}
	debug.Println("Settings loaded")

	os.Mkdir(config.certFolder, 0755)
	_, err = cert.GetOrGenerateKeyPair(config.webCertFile, config.webKeyFile, "web.sniffy.local", []string{"Sniffy"}, false, nil)
	if err != nil {
		log.Fatalln("Couldn't generate web interface RSA key pair:", err)
	}

	loadTemplates()

	pss, err := getProxyServers("")
	if err != nil {
		log.Println("Error starting proxy servers:", err)
	} else {
		for _, v := range pss {
			proxyServers = append(proxyServers, v)

			go func(ps *proxyServer) {
				var err error
				if ps.CertFile != "" && ps.KeyFile != "" {
					err = ps.ps.ListenAndServeTLS(v.CertFile, v.KeyFile)
				} else {
					err = ps.ps.ListenAndServe()
				}
				if err != nil {
					log.Println("Proxy server", ps.Name, "stopped:", err)
				}
			}(v)
		}
	}

	dss, err := getDummyServers("")
	if err != nil {
		log.Println("Error starting dummy servers:", err)
	} else {
		for _, v := range dss {
			dummyServers = append(dummyServers, v)
			if v.CertFile != "" && v.KeyFile != "" {
				_, err = cert.GetOrGenerateKeyPair(v.CertFile, v.KeyFile, "dummy.sniffy.local", []string{"Sniffy"}, false, nil)
			}
			go v.ds.Run()
		}
	}

	os.Mkdir(config.interceptorCertFolder, 0755)
	sslInterceptor, err = sniff.NewSSLInterceptor(proxyServers[0], config.interceptorCACertFile, config.interceptorCAKeyFile)
	if err != nil {
		log.Fatalln("Couldn't create SSL interceptor:", err)
	}
	if config.preloadInterceptorCerts {
		rows, err := db.Query("SELECT cn FROM certs")
		if err == nil {
			for rows.Next() {
				var cn string
				rows.Scan(&cn)
				if cn != "web.sniffy.local" && cn != "interceptor.sniffy.local" {
					go func() {
						_, err := sslInterceptor.GetHostKeyPair(cn)
						if err != nil {
							log.Println("Couldn't preload RSA key pair for", cn+":", err)
						}
					}()
				}
			}
		}
	}

	// BEGIN TESTING!
	test := false
	if test {
		http.HandleFunc("/", testfunc)
		testkp, _ := sslInterceptor.GetHostKeyPair("sheeped.com")
		// testkp.Certificate = append(testkp.Certificate, si.caKeyPair.Certificate...)
		config := &tls.Config{
			Rand:       rand.Reader,
			NextProtos: []string{"http/1.1"},
			Certificates: []tls.Certificate{
				*testkp,
				// si.caKeyPair,
			},
		}
		log.Println("LAUNCHING TEST SSL SERVER :20010")
		l, _ := tls.Listen("tcp", ":20010", config)
		srv := http.Server{}
		srv.Serve(l)
		// END TESTING!
	}

	wsHost, err := getSetting("WebServerHost")
	if err != nil {
		wsHost = "127.0.0.1"
	}
	wsPort, err := getIntSetting("WebServerPort")
	if err != nil || wsPort < 1 || wsPort > 65535 {
		wsPort = defaultWebServerPort
	}
	// log.Println("Logging to", logFile)
	initLogger(logToFile)
	fmt.Println("")
	fmt.Printf("Sniffy interface running on https://%s:%d\n", wsHost, wsPort)
	ws := NewWebServer(wsHost, uint16(wsPort))
	go ws.Run()
	fmt.Println("")
	fmt.Print("For recovery, enter administrator password: ")
	prompt(stateLogin)
}

// TESTING
func testfunc(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("LOL!\n"))
}

// TESTING

type SniffyConfig struct {
	DBType                  string
	DBHost                  string
	DBPort                  int
	DBName                  string
	DBUser                  string
	DBPass                  string
	certFolder              string
	webCertFile             string
	webKeyFile              string
	preloadInterceptorCerts bool
	interceptorCertFolder   string
	interceptorCACertFile   string
	interceptorCAKeyFile    string
}

func loadConfig() (*SniffyConfig, error) {
	var c SniffyConfig
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("Couldn't open Sniffy config file: " + err.Error())
	}
	err = json.Unmarshal(data, &c)
	if err != nil {
		return nil, fmt.Errorf("Couldn't parse Sniffy config file: " + err.Error())
	}
	return &c, nil
}

func saveConfig() error {
	f, err := os.OpenFile(configFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("Couldn't open Sniffy config file for writing: " + err.Error())
	}
	defer f.Close()
	json, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("Couldn't save Sniffy config file: " + err.Error())
	}
	f.Write(json)
	return nil
}

func loadSettings() error {
	opts, err := getSettings()
	if err != nil {
		return err
	}
	config.certFolder = opts["CertFolder"]
	config.webCertFile = opts["WebCertFile"]
	config.webKeyFile = opts["WebKeyFile"]
	if opts["PreloadInterceptorCerts"] == "1" {
		config.preloadInterceptorCerts = true
	}
	config.interceptorCertFolder = opts["InterceptorCertFolder"]
	config.interceptorCACertFile = opts["InterceptorCACertFile"]
	config.interceptorCAKeyFile = opts["InterceptorCAKeyFile"]
	return nil
}
