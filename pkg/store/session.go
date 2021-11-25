package store

import (
	"crypto/tls"
	"net"
	"os"
	"time"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gopkg.in/mgo.v2"
)

// Session models each database session
type Session struct {
	Certificate *os.File
	URI         *URI
	sess        *mgo.Session
	Timeout     int
}

// NewSession configure URI database info and returns a mgo.Session struct
// The goal is centralize all data access in one layer
func NewSession() (sess *mgo.Session, err error) {

	cfg := config.GetConfig()

	uri := &URI{
		Database:    cfg.GetString("storage.database"),
		Hosts:       []string{cfg.GetString("storage.hosts")},
		Password:    cfg.GetString("storage.password"),
		User:        cfg.GetString("storage.user"),
		SSL:         cfg.GetBool("storage.ssl"),
		Certificate: cfg.GetString("storage.certificate"),
		URL:         cfg.GetString("storage.url"),
	}

	return SessionFrom(uri, cfg.GetInt("storage.timeout"))
}

// SessionFrom accepts an URI and timeout as parameters for create a mongo Session.
func SessionFrom(uri *URI, timeout int) (sess *mgo.Session, err error) {
	s := &Session{
		URI:     uri,
		Timeout: timeout,
	}
	if err = s.LoadCertificate(); err != nil {
		return nil, err
	}

	return s.Open()
}

// LoadCertificate open the file certification when URI.SSL is `true`
func (s *Session) LoadCertificate() (err error) {
	if s.URI.SSL && s.URI.Certificate != "" {
		filename := s.URI.Certificate
		s.Certificate, err = os.Open(filename)
	}
	return
}

// Open a session from mongo server
func (s *Session) Open() (sess *mgo.Session, err error) {
	var info *mgo.DialInfo
	if info, err = mgo.ParseURL(s.URI.String()); err != nil {
		core.PrintError(err, "ParseURL")
		return nil, err
	}

	// COMMENT: Setting timeout field could be throw an unexpected error (1000000000 = 1s)
	info.Timeout = time.Duration(s.Timeout * 1000000000)
	if s.URI.SSL {
		logger.Get().Info("-- SSL is enabled ... ")
		if s.URI.Certificate != "" {
			info.Mechanism = "MONGODB-X509"
		}
		info.DialServer = s.Connection
	}
	if sess, err = mgo.DialWithInfo(info); err != nil {
		core.PrintError(err, "DialWithInfo()")
		return
	}
	defer sess.Close()

	cloned := sess.Clone()
	err = cloned.Ping()
	if err != nil {
		core.PrintError(err, "PING")
	}
	cloned.SetMode(mgo.Monotonic, true)
	logger.Get().Info("5 - Open() ... ")
	return cloned, err
}

// Connection returns net.Conn established with mgo.ServerAddr info
func (s *Session) Connection(addr *mgo.ServerAddr) (conn net.Conn, err error) {
	var tlsConfig *tls.Config
	if tlsConfig, err = ConfigureCert(s.Certificate, s.URI.Certificate == ""); err != nil {
		core.PrintError(err, "ConfigureCert()")
		return nil, err
	}
	conn, err = tls.Dial("tcp", addr.String(), tlsConfig)
	if err != nil {
		core.PrintError(err, "tls.Dial()")
	}
	return conn, err
}

// DropDB removes the entire database including all of its collections.
func DropDB() error {
	if os.Getenv("ENV") != "testing" {
		panic("Database can be dropped only in testing environment")
	}
	return session.DB(config.GetConfig().GetString("storage.database")).DropDatabase()
}
