package main

import (
	"fmt"
	"io"
	stdlog "log"
	"os"
)

const (
	logToStdout = iota
	logToFile
	logBoot
)

var (
	log   *stdlog.Logger
	debug *stdlog.Logger
)

type NoDebugLogger struct{}

func (l NoDebugLogger) Write(data []byte) (int, error) {
	return len(data), nil
}

func initLogger(m int) {
	var (
		fp     io.Writer
		err    error
		prefix = ""
		flags  = stdlog.LstdFlags
	)
	if DEBUG {
		fp = os.Stdout
	} else {
		fp = NoDebugLogger{}
	}
	debug = stdlog.New(fp, prefix, flags)

	switch m {
	default:
		fmt.Println("Invalid logging mode; logging to stdout:", err)
		fp = os.Stdout
	case logToStdout:
		fp = os.Stdout
	case logToFile:
		fp, err = os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		// flags = flags|stdlog.Lshortfile
		if err != nil {
			fmt.Println("Error opening log file:", err)
			fp = os.Stdout
		}
	case logBoot:
		fp = os.Stdout
		prefix = " --[ "
		flags = 0
	}
	log = stdlog.New(fp, prefix, flags)
}
