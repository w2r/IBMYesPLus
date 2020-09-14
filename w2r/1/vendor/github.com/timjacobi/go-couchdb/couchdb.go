// Package couchdb implements wrappers for the CouchDB HTTP API.
//
// Unless otherwise noted, all functions in this package
// can be called from more than one goroutine at the same time.
package couchdb

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// Client represents a remote CouchDB server.
type Client struct{ *transport }

// NewClient creates a new client object.
//
// If rawurl contains credentials, the client will authenticate
// using HTTP Basic Authentication. If rawurl has a query string,
// it is ignored.
//
// The second argument can be nil to use http.Transport,
// which should be good enough in most cases.
func NewClient(rawurl string, rt http.RoundTripper) (*Client, error) {
	url, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	url.RawQuery, url.Fragment = "", ""
	var auth Auth
	if url.User != nil {
		passwd, _ := url.User.Password()
		auth = BasicAuth(url.User.Username(), passwd)
		url.User = nil
	}
	return &Client{newTransport(url.String(), rt, auth)}, nil
}

// URL returns the URL prefix of the server.
// The url will not contain a trailing '/'.
func (c *Client) URL() string {
	return c.prefix
}

// Ping can be used to check whether a server is alive.
// It sends an HTTP HEAD request to the server's URL.
func (c *Client) Ping() error {
	_, err := c.closedRequest("HEAD", "/", nil)
	return err
}

// SetAuth sets the authentication mechanism used by the client.
// Use SetAuth(nil) to unset any mechanism that might be in use.
// In order to verify the credentials against the server, issue any request
// after the call the SetAuth.
func (c *Client) SetAuth(a Auth) {
	c.transport.setAuth(a)
}

// CreateDB creates a new database.
// The request will fail with status "412 Precondition Failed" if the database
// already exists. A valid DB object is returned in all cases, even if the
// request fails.
func (c *Client) CreateDB(name string) (*DB, error) {
	if _, err := c.closedRequest("PUT", path(name), nil); err != nil {
		return c.DB(name), err
	}
	return c.DB(name), nil
}

// EnsureDB ensures that a database with the given name exists.
func (c *Client) EnsureDB(name string) (*DB, error) {
	db, err := c.CreateDB(name)
	if err != nil && !ErrorStatus(err, http.StatusPreconditionFailed) {
		return nil, err
	}
	return db, nil
}

// DeleteDB deletes an existing database.
func (c *Client) DeleteDB(name string) error {
	_, err := c.closedRequest("DELETE", path(name), nil)
	return err
}

// AllDBs returns the names of all existing databases.
func (c *Client) AllDBs() (names []string, err error) {
	resp, err := c.request("GET", "/_all_dbs", nil)
	if err != nil {
		return names, err
	}
	err = readBody(resp, &names)
	return names, err
}

// DB represents a remote CouchDB database.
type DB struct {
	*transport
	name string
}

// DB creates a database object.
// The database inherits the authentication and http.RoundTripper
// of the client. The database's actual existence is not verified.
func (c *Client) DB(name string) *DB {
	return &DB{c.transport, name}
}

// Name returns the name of a database.
func (db *DB) Name() string {
	return db.name
}

var getJsonKeys = []string{"open_revs", "atts_since"}

// Get retrieves a document from the given database.
// The document is unmarshalled into the given object.
// Some fields (like _conflicts) will only be returned if the
// options require it. Please refer to the CouchDB HTTP API documentation
// for more information.
//
// http://docs.couchdb.org/en/latest/api/document/common.html?highlight=doc#get--db-docid
func (db *DB) Get(id string, doc interface{}, opts Options) error {
	path, err := optpath(opts, getJsonKeys, db.name, id)
	if err != nil {
		return err
	}
	resp, err := db.request("GET", path, nil)
	if err != nil {
		return err
	}
	return readBody(resp, &doc)
}

// Rev fetches the current revision of a document.
// It is faster than an equivalent Get request because no body
// has to be parsed.
func (db *DB) Rev(id string) (string, error) {
	return responseRev(db.closedRequest("HEAD", path(db.name, id), nil))
}

// Put stores a document into the given database.
func (db *DB) Put(id string, doc interface{}, rev string) (newrev string, err error) {
	path := revpath(rev, db.name, id)
	// TODO: make it possible to stream encoder output somehow
	json, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	b := bytes.NewReader(json)
	return responseRev(db.closedRequest("PUT", path, b))
}

// Post creates a new document in the given database.
// Returns the id and revision of the newly created document.
//
// See http://docs.couchdb.org/en/latest/api/database/common.html#post--db
func (db *DB) Post(doc interface{}) (string, string, error) {
	path := "/" + db.name

	json, err := json.Marshal(doc)
	if err != nil {
		return "", "", err
	}
	b := bytes.NewReader(json)

	resp, err := db.request("POST", path, b)
	if err != nil {
		return "", "", err
	}

	var data struct {
		ID  string `json:"id"`
		OK  bool   `json:"ok"`
		Rev string `json:"rev"`
	}

	err = readBody(resp, &data)
	if err != nil {
		return "", "", err
	}

	return data.ID, data.Rev, nil
}

// Delete marks a document revision as deleted.
func (db *DB) Delete(id, rev string) (newrev string, err error) {
	path := revpath(rev, db.name, id)
	return responseRev(db.closedRequest("DELETE", path, nil))
}

// Security represents database security objects.
type Security struct {
	Admins  Members `json:"admins"`
	Members Members `json:"members"`
}

// Members represents member lists in database security objects.
type Members struct {
	Names []string `json:"names,omitempty"`
	Roles []string `json:"roles,omitempty"`
}

// Security retrieves the security object of a database.
func (db *DB) Security() (*Security, error) {
	secobj := new(Security)
	resp, err := db.request("GET", path(db.name, "_security"), nil)
	if err != nil {
		return nil, err
	}
	if resp.ContentLength == 0 {
		// empty reply means defaults
		return secobj, nil
	}
	if err = readBody(resp, secobj); err != nil {
		return nil, err
	}
	return secobj, nil
}

// PutSecurity sets the database security object.
func (db *DB) PutSecurity(secobj *Security) error {
	json, _ := json.Marshal(secobj)
	body := bytes.NewReader(json)
	_, err := db.request("PUT", path(db.name, "_security"), body)
	return err
}

var viewJsonKeys = []string{"startkey", "start_key", "key", "endkey", "end_key"}

// View invokes a view.
// The ddoc parameter must be the full name of the design document
// containing the view definition, including the _design/ prefix.
//
// The output of the query is unmarshalled into the given result.
// The format of the result depends on the options. Please
// refer to the CouchDB HTTP API documentation for all the possible
// options that can be set.
//
// http://docs.couchdb.org/en/latest/api/ddoc/views.html
func (db *DB) View(ddoc, view string, result interface{}, opts Options) error {
	if !strings.HasPrefix(ddoc, "_design/") {
		return errors.New("couchdb.View: design doc name must start with _design/")
	}
	path, err := optpath(opts, viewJsonKeys, db.name, ddoc, "_view", view)
	if err != nil {
		return err
	}
	resp, err := db.request("GET", path, nil)
	if err != nil {
		return err
	}
	return readBody(resp, &result)
}

// AllDocs invokes the _all_docs view of a database.
//
// The output of the query is unmarshalled into the given result.
// The format of the result depends on the options. Please
// refer to the CouchDB HTTP API documentation for all the possible
// options that can be set.
//
// http://docs.couchdb.org/en/latest/api/database/bulk-api.html#db-all-docs
func (db *DB) AllDocs(result interface{}, opts Options) error {
	path, err := optpath(opts, viewJsonKeys, db.name, "_all_docs")
	if err != nil {
		return err
	}
	resp, err := db.request("GET", path, nil)
	if err != nil {
		return err
	}
	return readBody(resp, &result)
}
