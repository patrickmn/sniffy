package main

import (
	"github.com/pmylund/sniffy/dummy"
)

type dummyServer struct {
	Id          uint64
	Name        string
	CertFile    string
	KeyFile     string
	LogRequests bool
	ds          *dummy.DummyServer
}
