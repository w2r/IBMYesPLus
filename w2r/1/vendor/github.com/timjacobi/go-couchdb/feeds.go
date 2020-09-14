package couchdb

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// DBUpdatesFeed is an iterator for the _db_updates feed.
// This feed receives an event whenever any database is created, updated
// or deleted. On each call to the Next method, the event fields are updated
// for the current event.
//
//     feed, err := client.DbUpdates(nil)
//     ...
//     for feed.Next() {
//	       fmt.Printf("changed: %s %s", feed.Event, feed.Db)
//     }
//     err = feed.Err()
//     ...
type DBUpdatesFeed struct {
	Event string `json:"type"`    // "created" | "updated" | "deleted"
	OK    bool   `json:"ok"`      // Event operation status
	DB    string `json:"db_name"` // Event database name

	end  bool
	err  error
	conn io.Closer
	dec  *json.Decoder
}

// DBUpdates opens the _db_updates feed.
// For the possible options, please see the CouchDB documentation.
// Pleas note that the "feed" option is currently always set to "continuous".
//
// http://docs.couchdb.org/en/latest/api/server/common.html#db-updates
func (c *Client) DBUpdates(options Options) (*DBUpdatesFeed, error) {
	newopts := options.clone()
	newopts["feed"] = "continuous"
	path, err := optpath(newopts, nil, "_db_updates")
	if err != nil {
		return nil, err
	}
	resp, err := c.request("GET", path, nil)
	if err != nil {
		return nil, err
	}
	feed := &DBUpdatesFeed{
		conn: resp.Body,
		dec:  json.NewDecoder(resp.Body),
	}
	return feed, nil
}

// Next decodes the next event in a _db_updates feed. It returns false when
// the feeds end has been reached or an error has occurred.
func (f *DBUpdatesFeed) Next() bool {
	if f.end {
		return false
	}
	f.Event, f.DB, f.OK = "", "", false
	if f.err = f.dec.Decode(f); f.err != nil {
		if f.err == io.EOF {
			f.err = nil
		}
		f.Close()
	}
	return !f.end
}

// Err returns the last error that occurred during iteration.
func (f *DBUpdatesFeed) Err() error {
	return f.err
}

// Close terminates the connection of a feed.
func (f *DBUpdatesFeed) Close() error {
	f.end = true
	return f.conn.Close()
}

// ChangesFeed is an iterator for the _changes feed of a database.
// On each call to the Next method, the event fields are updated
// for the current event. Next is designed to be used in a for loop:
//
//     feed, err := client.Changes("db", nil)
//     ...
//     for feed.Next() {
//	       fmt.Printf("changed: %s", feed.ID)
//     }
//     err = feed.Err()
//     ...
type ChangesFeed struct {
	// DB is the database. Since all events in a _changes feed
	// belong to the same database, this field is always equivalent to the
	// database from the DB.Changes call that created the feed object
	DB *DB `json:"-"`

	// ID is the document ID of the current event.
	ID string `json:"id"`

	// Deleted is true when the event represents a deleted document.
	Deleted bool `json:"deleted"`

	// Seq is the database update sequence number of the current event.
	// After all items have been processed, set to the last_seq value sent
	// by CouchDB. In CouchDB1.X, this is an int64. In CouchDB2, this is
	// an opaque string.
	Seq interface{} `json:"seq"`

	// Changes is the list of the document's leaf revisions.
	Changes []struct {
		Rev string `json:"rev"`
	} `json:"changes"`

	// The document. This is populated only if the feed option
	// "include_docs" is true.
	Doc json.RawMessage `json:"doc"`

	end    bool
	err    error
	conn   io.Closer
	parser func() error
}

// Changes opens the _changes feed of a database. This feed receives an event
// whenever a document is created, updated or deleted.
//
// The implementation supports both poll-style and continuous feeds.
// The default feed mode is "normal", which retrieves changes up to some point
// and then closes the feed. If you want a never-ending feed, set the "feed"
// option to "continuous":
//
//     feed, err := client.Changes("db", couchdb.Options{"feed": "continuous"})
//
// There are many other options that allow you to customize what the
// feed returns. For information on all of them, see the official CouchDB
// documentation:
//
// http://docs.couchdb.org/en/latest/api/database/changes.html#db-changes
func (db *DB) Changes(options Options) (*ChangesFeed, error) {
	path, err := optpath(options, nil, db.name, "_changes")
	if err != nil {
		return nil, err
	}
	resp, err := db.request("GET", path, nil)
	if err != nil {
		return nil, err
	}
	feed := &ChangesFeed{DB: db, conn: resp.Body}

	switch options["feed"] {
	case nil, "normal", "longpoll":
		scan := newScanner(resp.Body)
		if err := scan.tokens("{", "\"results\"", ":", "["); err != nil {
			feed.Close()
			return nil, err
		}
		feed.parser = feed.pollParser(scan)
	case "continuous":
		feed.parser = feed.contParser(resp.Body)
	default:
		err := fmt.Errorf(
			`couchdb: unsupported value for option "feed": %#v`,
			options["feed"],
		)
		feed.Close()
		return nil, err
	}

	return feed, nil
}

// Next decodes the next event. It returns false when the feeds end has been
// reached or an error has occurred.
func (f *ChangesFeed) Next() bool {
	if f.end {
		return false
	}
	if f.err = f.parser(); f.err != nil || f.end {
		f.Close()
	}
	return !f.end
}

// Err returns the last error that occurred during iteration.
func (f *ChangesFeed) Err() error {
	return f.err
}

// Close terminates the connection of the feed.
// If Next returns false, the feed has already been closed.
func (f *ChangesFeed) Close() error {
	f.end = true
	return f.conn.Close()
}

func (f *ChangesFeed) contParser(r io.Reader) func() error {
	dec := json.NewDecoder(r)
	return func() error {

		var row struct {
			ID      string      `json:"id"`
			Seq     interface{} `json:"seq"`
			Changes []struct {
				Rev string `json:"rev"`
			} `json:"changes"`
			Doc     json.RawMessage `json:"doc"`
			Deleted bool            `json:"deleted"`
			LastSeq bool            `json:"last_seq"`
		}

		if err := dec.Decode(&row); err != nil {
			return err
		}

		f.ID, f.Seq, f.Deleted, f.Changes, f.Doc =
			row.ID, row.Seq, row.Deleted, row.Changes, row.Doc

		if row.LastSeq {
			f.end = true
			return nil
		}
		return nil
	}
}

func (f *ChangesFeed) pollParser(scan *scanner) func() error {
	first := true
	return func() error {
		// reset fields.
		f.ID, f.Deleted = "", false

		next, err := scan.peek()
		switch {
		case err != nil:
			return err
		case next == ']':
			// decode last_seq key
			f.end = true
			scan.skipByte()
			if err = scan.tokens(",", "\"last_seq\"", ":"); err != nil {
				return err
			}

			// CouchDB1 and CouchDB2 differ in their Seq fields: ints for the former, and strings
			// for the latter.
			var tmp string
			_, err = fmt.Fscanf(scan.in, "%s", &tmp)
			if err != nil {
				return err
			}

			if tmp[0] == '"' { // String, so CouchDB2-style
				f.Seq = tmp[1 : len(tmp)-1] // Skip leading and trailling double-quote
			} else if i, err := strconv.ParseInt(tmp, 10, 64); err == nil {
				f.Seq = i // int, so we're done
			}

			return err
		case next == ',' && !first:
			scan.skipByte()
		}
		first = false
		return scan.decodeObject(f)
	}
}

type scanner struct {
	in *bufio.Reader

	// stuff for the JSON buffering FSM
	jsbuf bytes.Buffer
	state scanState
	stack []scanState
}

func newScanner(r io.Reader) *scanner {
	return &scanner{in: bufio.NewReaderSize(r, 4096)}
}

// peek returns the next non-whitespace byte in the input stream
func (s *scanner) peek() (byte, error) {
	b, err := s.skipSpace()
	if err != nil {
		return 0, err
	}
	s.in.UnreadByte()
	return b, nil
}

// skipByte drops the next byte from the input stream.
func (s *scanner) skipByte() error {
	_, err := s.in.ReadByte()
	return err
}

// tokens verifies that the given tokens are present in the
// input stream. Whitespace between tokens is skipped.
func (s *scanner) tokens(toks ...string) error {
	for _, tok := range toks {
		b, err := s.skipSpace()
		if err != nil {
			return err
		}
		tbuf := make([]byte, len(tok))
		tbuf[0] = b
		if len(tok) > 1 {
			if _, err := io.ReadFull(s.in, tbuf[1:]); err != nil {
				return err
			}
		}
		for i := 0; i < len(tok); i++ {
			if tbuf[i] != tok[i] {
				return fmt.Errorf(
					"unexpected token: found %q, want %q", tbuf, tok,
				)
			}
		}
	}
	return nil
}

// skipSpace searches for the next non-whitespace byte
// in the input stream.
func (s *scanner) skipSpace() (byte, error) {
	for {
		b, err := s.in.ReadByte()
		if err != nil {
			return 0, err
		}
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return b, nil
		}
	}
}

// decodeInt64 reads an integer from the input stream.
func (s *scanner) decodeInt64() (num int64, err error) {
	_, err = fmt.Fscanf(s.in, "%d", &num)
	return
}

// decodeObject reads a JSON object from the input stream.
func (s *scanner) decodeObject(target interface{}) error {
	b, err := s.skipSpace()
	switch {
	case err != nil:
		return err
	case b == '{':
		s.state = jsonStateObj
	default:
		return fmt.Errorf("invalid character %q at start of JSON object", b)
	}

	// buffer input until there's a complete value in the buffer
	s.jsbuf.Reset()
	s.jsbuf.WriteByte(b)
	s.stack = nil
	for s.state != nil {
		b, err := s.in.ReadByte()
		if err != nil {
			return err
		}
		s.state = s.state(s, b)
		s.jsbuf.WriteByte(b)
	}

	return json.Unmarshal(s.jsbuf.Bytes(), target)
}

// JSON buffering FSM.

type scanState func(s *scanner, b byte) scanState

func (s *scanner) popState() scanState {
	if len(s.stack) == 0 {
		return nil
	}
	prev := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	return prev
}

func (s *scanner) pushState(next scanState) scanState {
	s.stack = append(s.stack, s.state)
	return next
}

func jsonStateObj(s *scanner, b byte) scanState {
	switch b {
	case '}':
		return s.popState()
	case '{':
		return s.pushState(jsonStateObj)
	case '"':
		return s.pushState(jsonStateString)
	default:
		return jsonStateObj
	}
}

func jsonStateString(s *scanner, b byte) scanState {
	switch b {
	case '\\':
		return jsonStateStringEsc
	case '"':
		return s.popState()
	default:
		return jsonStateString
	}
}

func jsonStateStringEsc(*scanner, byte) scanState {
	return jsonStateString
}
