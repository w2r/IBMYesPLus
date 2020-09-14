package couchdb

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Auth is implemented by HTTP authentication mechanisms.
type Auth interface {
	// AddAuth should add authentication information (e.g. headers)
	// to the given HTTP request.
	AddAuth(*http.Request)
}

type basicauth string

// BasicAuth returns an Auth that performs HTTP Basic Authentication.
func BasicAuth(username, password string) Auth {
	auth := []byte(username + ":" + password)
	hdr := "Basic " + base64.StdEncoding.EncodeToString(auth)
	return basicauth(hdr)
}

func (a basicauth) AddAuth(req *http.Request) {
	req.Header.Set("Authorization", string(a))
}

type proxyauth struct {
	username, roles, tok string
}

// ProxyAuth returns an Auth that performs CouchDB proxy authentication.
// Please consult the CouchDB documentation for more information on proxy
// authentication:
//
// http://docs.couchdb.org/en/latest/api/server/authn.html?highlight=proxy#proxy-authentication
func ProxyAuth(username string, roles []string, secret string) Auth {
	pa := &proxyauth{username, strings.Join(roles, ","), ""}
	if secret != "" {
		mac := hmac.New(sha1.New, []byte(secret))
		io.WriteString(mac, username)
		pa.tok = fmt.Sprintf("%x", mac.Sum(nil))
	}
	return pa
}

func (a proxyauth) AddAuth(req *http.Request) {
	req.Header.Set("X-Auth-CouchDB-UserName", a.username)
	req.Header.Set("X-Auth-CouchDB-Roles", a.roles)
	if a.tok != "" {
		req.Header.Set("X-Auth-CouchDB-Token", a.tok)
	}
}
