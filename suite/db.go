package main

// TODO: Maybe use Cassandra or MongoDB for Request/Response storage

import (
	"github.com/pmylund/sniffy/common/queue"
	"github.com/pmylund/sniffy/dummy"
	"github.com/pmylund/sniffy/proxy"

	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	// _ "github.com/bmizerany/pq"
	_ "github.com/jbarham/gopgsqldriver"
	"github.com/pmylund/go-cache"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	CurrentSchemaVersion    = uint64(1)
	ErrInvalidSchemaVersion = errors.New("Invalid DB schema version--corrupt database?")
	defaultDBSchema         = `
CREATE TABLE version(
    name    VARCHAR(64) PRIMARY KEY NOT NULL,
    version INTEGER NOT NULL
);

CREATE TABLE settings(
    name  VARCHAR(64) PRIMARY KEY NOT NULL,
    value TEXT NOT NULL
);

CREATE TABLE proxyservers(
    id               SERIAL PRIMARY KEY NOT NULL,
    name             VARCHAR(64) NOT NULL,
    port             INTEGER NOT NULL,
    certfile         VARCHAR(255) NOT NULL,
    keyfile          VARCHAR(255) NOT NULL,
    moderaterequests BOOL NOT NULL,
    interceptssl     BOOL NOT NULL,
    logrequests      BOOL NOT NULL
);

CREATE TABLE dummyservers(
    id               SERIAL PRIMARY KEY NOT NULL,
    name             VARCHAR(64) NOT NULL,
    port             INTEGER NOT NULL,
    certfile         VARCHAR(255) NOT NULL,
    keyfile          VARCHAR(255) NOT NULL
);

CREATE TABLE certs(
    id BIGSERIAL PRIMARY KEY NOT NULL,
    cn VARCHAR(255) NOT NULL
);

CREATE TABLE requests(
    id               BIGSERIAL PRIMARY KEY NOT NULL,
    time             INTEGER NOT NULL,
    method           VARCHAR(10) NOT NULL,
    url              TEXT NOT NULL,
    proto            VARCHAR(10) NOT NULL,
    header           TEXT NOT NULL,
    contentlength    INTEGER NOT NULL,
    transferencoding TEXT NOT NULL,
    host             VARCHAR(255) NOT NULL,
    remoteaddr       VARCHAR(255) NOT NULL,
    tls              BOOL NOT NULL,
    ps_id            INTEGER NOT NULL REFERENCES proxyservers(id)
);

CREATE TABLE responses(
    id               BIGSERIAL PRIMARY KEY NOT NULL,
    time             INTEGER NOT NULL,
    status           VARCHAR(64) NOT NULL,
    statuscode       INTEGER NOT NULL,
    proto            VARCHAR(10) NOT NULL,
    header           TEXT NOT NULL,
    contentlength    INTEGER NOT NULL,
    transferencoding TEXT NOT NULL,
    close            BOOL NOT NULL,
    req_id           BIGINT NOT NULL REFERENCES requests(id) ON DELETE CASCADE
);
`
	defaultDBData = `
INSERT INTO version(name, version) VALUES('dbSchema', 1);
INSERT INTO settings(name, value) VALUES('CertFolder', 'cert');
INSERT INTO settings(name, value) VALUES('WebServerHost', '0.0.0.0'); -- TODO: should be localhost/127.0.0.1
INSERT INTO settings(name, value) VALUES('WebServerPort', '8005');
INSERT INTO settings(name, value) VALUES('WebCertFile', 'cert/web_cert.pem');
INSERT INTO settings(name, value) VALUES('WebKeyFile', 'cert/web_key.pem');
INSERT INTO settings(name, value) VALUES('EnvProxyTestUrl', 'http://envproxytest.mylund.com');
INSERT INTO settings(name, value) VALUES('InterceptorCertFolder', 'cert/interceptor');
INSERT INTO settings(name, value) VALUES('InterceptorCACertFile', 'cert/interceptor_ca_cert.pem');
INSERT INTO settings(name, value) VALUES('InterceptorCAKeyFile', 'cert/interceptor_ca_key.pem');
INSERT INTO settings(name, value) VALUES('PreloadInterceptorCerts', '1');
INSERT INTO settings(name, value) VALUES('iCertSnTop', '0');

INSERT INTO proxyservers(name, port, certfile, keyfile, moderaterequests,
                         interceptssl, logrequests)
VALUES      ('Default Proxy Server', 8000, '', '', false, false, true);
INSERT INTO proxyservers(name, port, certfile, keyfile, moderaterequests,
                         interceptssl, logrequests)
VALUES      ('8001 Proxy Server', 8001, '', '', false, false, true);

INSERT INTO dummyservers(name, port, certfile, keyfile)
VALUES      ('default', 8002, '', '');

INSERT INTO dummyservers(name, port, certfile, keyfile)
VALUES      ('default', 8003, 'cert/dummy_cert.pem', 'cert/dummy_key.pem');
`
	dbCache *cache.Cache
)

// TODO: Embed http.Request maybe yes
type requestEntry struct {
	Id               int64
	Time             int64
	Method           string
	URL              *url.URL
	Proto            string
	Header           http.Header
	ContentLength    int64
	TransferEncoding []string
	Host             string
	RemoteAddr       string
	TLSHandshakeDone bool
	Response         *responseEntry
}

type responseEntry struct {
	Id               int64
	Time             int64
	Status           string
	StatusCode       int
	Proto            string
	Header           http.Header
	ContentLength    int64
	TransferEncoding []string
	Close            bool
}

// Splits a string containing SQL at each semicolon, runs each statement in
// a transaction, and rolls back if there is an error in any of the statements.
// Never pass untrusted input to this function; the SQL statements are executed
// without any form of input validation.
func SplitRunSQLStringInTransaction(s string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, v := range strings.Split(s, ";") {
		if v == "" || v == "\n" {
			continue
		}
		_, err = tx.Exec(v)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

func connectDB() error {
	var err error
	db, err = sql.Open(config.DBType, fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=%s", config.DBUser, config.DBPass, config.DBHost, config.DBPort, config.DBName))
	return err
}

func initDB() error {
	var v uint64
	row := db.QueryRow("SELECT version FROM version WHERE name = 'dbSchema' LIMIT 1")
	err := row.Scan(&v)
	if err == nil {
		if v == 0 {
			return ErrInvalidSchemaVersion
		}
		if v < CurrentSchemaVersion {
			err = migrateDBFrom(v)
			if err != nil {
				log.Println("Failed to migrate DB to latest version:", err)
				return err
			}
		}
		debug.Println("Database", config.DBName, "opened")
		dbCache = cache.New(5*time.Minute, 1*time.Minute)
		dbCache.Set("lastRequestUpdate|int64", time.Now().Unix(), -1)
		return nil
	}
	debug.Println("Initializing database", config.DBName)
	err = SplitRunSQLStringInTransaction(defaultDBSchema + defaultDBData)
	if err != nil {
		return err
	}
	return initDB()
}

func migrateDBFrom(v uint64) error {
	var err error
	migrations := map[uint64][]string{
	// Version 1 is defaultDBSchema/defaultDBData
	// 2: {dbMigrate002data},
	}
	if v == 0 || v >= CurrentSchemaVersion {
		return ErrInvalidSchemaVersion
	}
	for i := v + 1; i <= CurrentSchemaVersion; i++ {
		log.Println("Migrating to database schema", i)
		stmts, found := migrations[i]
		if !found {
			return errors.New(fmt.Sprintf("No migration to schema version %d", i))
		}
		for _, v := range stmts {
			err = SplitRunSQLStringInTransaction(v)
			if err != nil {
				return err
			}
		}
		_, err = db.Exec("UPDATE version SET version = $1 WHERE name = 'dbSchema'", i)
		if err != nil {
			return err
		}
	}
	return nil
}

func getSetting(name string) (string, error) {
	var val string
	row := db.QueryRow("SELECT value FROM settings WHERE name = $1", name)
	err := row.Scan(&val)
	if err != nil {
		log.Println("Couldn't load setting", name, "from database:", err)
		return "", err
	}
	return val, nil
}

func getSettings() (map[string]string, error) {
	vals := map[string]string{}
	rows, err := db.Query("SELECT name, value FROM settings")
	if err != nil {
		log.Println("Couldn't load settings from database:", err)
		return nil, err
	}
	for rows.Next() {
		var name, value string
		rows.Scan(&name, &value)
		vals[name] = value
	}
	return vals, nil
}

func getIntSetting(name string) (int64, error) {
	opt, err := getSetting(name)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(opt, 10, 0)
}

func setSetting(name string, val string) error {
	_, err := db.Exec("UPDATE settings SET value = $1 WHERE name = $2", val, name)
	return err
}

func setIntSetting(name string, val int64) error {
	str := strconv.FormatInt(val, 10)
	return setSetting(name, str)
}

func nextCertSerialNumber(cn string) (int64, error) {
	last, err := getIntSetting("iCertSnTop")
	if err != nil {
		return 0, err
	}
	next := last + 1
	err = setIntSetting("iCertSnTop", next)
	if err != nil {
		return 0, err
	}
	return next, nil
}

func saveRequest(ps *proxyServer, req *http.Request) (int64, error) {
	headerjson, err := json.Marshal(req.Header)
	if err != nil {
		log.Println("Failed to marshal req.Header", req.Header)
	}
	transferencodingjson, err := json.Marshal(req.TransferEncoding)
	if err != nil {
		log.Println("Failed to marshal req.TransferEncoding", req.TransferEncoding)
	}
	handshakecomplete := false
	if req.TLS != nil {
		handshakecomplete = req.TLS.HandshakeComplete
	}
	now := time.Now().Unix()
	row := db.QueryRow(`
INSERT INTO requests(time, method, url, proto, header, contentlength,
                     transferencoding, host, remoteaddr, tls, ps_id)
VALUES      ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING   id`, now, req.Method, req.URL.String(), req.Proto, string(headerjson), req.ContentLength, string(transferencodingjson), req.Host, req.RemoteAddr, handshakecomplete, ps.Id)
	if err != nil {
		log.Println("Failed to save request:", req, "- Error:", err)
		return 0, err
	}
	var lid int64
	err = row.Scan(&lid)
	if err != nil {
		log.Println("Failed to retrieve resultant id for request:", req, "- Error:", err)
		return 0, err
	}
	dbCache.Set("lastRequestUpdate|int64", now, -1)
	return lid, nil
}

func saveResponse(reqId int64, res *http.Response) (int64, error) {
	if reqId == 0 {
		return 0, errors.New(fmt.Sprintf("Invalid req_id %d in call to save res: %s", reqId, res))
	}
	headerjson, err := json.Marshal(res.Header)
	if err != nil {
		log.Println("Failed to marshal res.Header", res.Header)
	}
	transferencodingjson, err := json.Marshal(res.TransferEncoding)
	if err != nil {
		log.Println("Failed to marshal res.TransferEncoding", res.TransferEncoding)
	}
	row := db.QueryRow(`
INSERT INTO responses(time, status, statuscode, proto, header, contentlength,
                      transferencoding, close, req_id)
VALUES      ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING   id`, time.Now().Unix(), res.Status, res.StatusCode, res.Proto, string(headerjson), res.ContentLength, string(transferencodingjson), res.Close, reqId)
	var lid int64
	err = row.Scan(&lid)
	if err != nil {
		log.Println("Failed to scan last insert id for response:", res, "- Error:", err)
		return 0, err
	}
	return lid, nil
}

func getRequests(joinRes bool, constraint string, vals ...interface{}) ([]requestEntry, error) {
	var (
		rows *sql.Rows
		res  []requestEntry
		err  error
	)
	if joinRes {
		rows, err = db.Query(`
SELECT     requests.id, requests.time, requests.method, requests.url,
           requests.proto, requests.header, requests.contentlength,
           requests.transferencoding, requests.host, requests.remoteaddr,
           requests.tls,

           responses.id, responses.time, responses.status, responses.statuscode,
           responses.proto, responses.header, responses.contentlength,
           responses.transferencoding, responses.close
FROM       requests
LEFT JOIN  responses
ON         requests.id = responses.req_id `+constraint, vals...)
	} else {
		rows, err = db.Query(`
SELECT id, time, method, url, proto, header, contentlength, transferencoding,
       host, remoteaddr, tls
FROM   requests `+constraint, vals...)
	}
	if err != nil {
		log.Println("Error fetching requests (constraint "+constraint+"):", err, "Responses joined:", joinRes)
		return res, err
	}
	for rows.Next() {
		var headerjson, transferencodingjson, rawurl string
		r := requestEntry{}
		if joinRes {
			var rehjson, retejson string
			re := responseEntry{}
			err = rows.Scan(&r.Id, &r.Time, &r.Method, &rawurl, &r.Proto, &headerjson, &r.ContentLength, &transferencodingjson, &r.Host, &r.RemoteAddr, &r.TLSHandshakeDone, &re.Id, &re.Time, &re.Status, &re.StatusCode, &re.Proto, &rehjson, &re.ContentLength, &retejson, &re.Close)
			if err == nil { // There is an error if the (joined) result can't be scanned
				err = json.Unmarshal([]byte(rehjson), &re.Header)
				if err != nil {
					log.Println("Couldn't unmarshal headerjson for request response:", err)
				}
				err = json.Unmarshal([]byte(retejson), &re.TransferEncoding)
				if err != nil {
					log.Println("Couldn't unmarshal transferencodingjson for request response:", err)
				}
				r.Response = &re
			}
		} else {
			err = rows.Scan(&r.Id, &r.Time, &r.Method, &rawurl, &r.Proto, &headerjson, &r.ContentLength, &transferencodingjson, &r.Host, &r.RemoteAddr, &r.TLSHandshakeDone)
			if err != nil {
				log.Println("Error scanning SQL:", err, "Responses joined:", joinRes)
				continue
			}
		}
		err := json.Unmarshal([]byte(headerjson), &r.Header)
		if err != nil {
			log.Println("Couldn't unmarshal headerjson for request:", err)
		}
		err = json.Unmarshal([]byte(transferencodingjson), &r.TransferEncoding)
		if err != nil {
			log.Println("Couldn't unmarshal transferencodingjson for request:", err)
		}
		r.URL, err = url.Parse(rawurl)
		if err != nil {
			log.Println("Couldn't parse URL for request:", err)
		}
		res = append(res, r)
	}
	return res, nil
}

func getRequest(id int64, joinRes bool) (*requestEntry, error) {
	reqs, err := getRequests(joinRes, "WHERE requests.id = $1 LIMIT 1", id)
	if err != nil {
		return nil, err
	}
	return &reqs[0], nil
}

func getProxyServers(constraint string, vals ...interface{}) ([]*proxyServer, error) {
	var res []*proxyServer
	rows, err := db.Query(`
SELECT id, name, port, certfile, keyfile, moderaterequests,
       interceptssl, logrequests
FROM   proxyservers `+constraint, vals...)
	if err != nil {
		log.Println("Error fetching requests (constraint "+constraint+"):", err)
		return res, err
	}
	for rows.Next() {
		ps := &proxyServer{ps: &proxy.ProxyServer{}}
		ps.ps.Handler = ps
		err = rows.Scan(&ps.Id, &ps.Name, &ps.ps.Port, &ps.CertFile, &ps.KeyFile, &ps.ModerateRequests, &ps.InterceptSSL, &ps.LogRequests)
		if err != nil {
			log.Println("Error scanning proxy server SQL:", err)
		}
		ps.queue = queue.New()
		res = append(res, ps)
	}
	return res, nil
}

func getProxyServer(id int64) (*proxyServer, error) {
	pss, err := getProxyServers("WHERE id = $1 LIMIT 1", id)
	if err != nil {
		return nil, err
	}
	return pss[0], nil
}

func getActiveProxyServer(psIdStr string) (*proxyServer, error) {
	psId, err := strconv.ParseUint(psIdStr, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("Invalid proxy server ID from client: %s", psIdStr)
	}
	for _, v := range proxyServers {
		if v.Id == psId {
			return v, nil
		}
	}
	return nil, fmt.Errorf("Proxy server does not exist, or it is not active")
}

func getDummyServers(constraint string, vals ...interface{}) ([]*dummyServer, error) {
	var res []*dummyServer
	rows, err := db.Query(`
SELECT id, name, port, certfile, keyfile
FROM   dummyservers `+constraint, vals...)
	if err != nil {
		log.Println("Error fetching requests (constraint "+constraint+"):", err)
		return res, err
	}
	for rows.Next() {
		ds := &dummyServer{
			ds: &dummy.DummyServer{},
		}
		err = rows.Scan(&ds.Id, &ds.Name, &ds.ds.Port, &ds.CertFile, &ds.KeyFile)
		if err != nil {
			log.Println("Error scanning dummy server SQL:", err)
		}
		res = append(res, ds)
	}
	return res, nil
}
