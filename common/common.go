package common

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
)

var (
	ErrStop = errors.New("Not continuing middleware/request evaluation")
)

func EscapeSQL(s string) string {
	// TODO: \ will still cause problems with exp/sql
	s = strings.Replace(s, "'", "''", -1)
	return s
}

func RandomString(bytes int) string {
	b := make([]byte, bytes)
	rand.Read(b)
	en := base64.URLEncoding
	d := make([]byte, en.EncodedLen(len(b)))
	en.Encode(d, b)
	return string(d)
}
