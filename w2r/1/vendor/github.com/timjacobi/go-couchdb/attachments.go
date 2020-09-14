package couchdb

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
)

// Attachment represents document attachments.
type Attachment struct {
	Name string    // Filename
	Type string    // MIME type of the Body
	MD5  []byte    // MD5 checksum of the Body
	Body io.Reader // The body itself
}

// Attachment retrieves an attachment.
// The rev argument can be left empty to retrieve the latest revision.
// The caller is responsible for closing the attachment's Body if
// the returned error is nil.
func (db *DB) Attachment(docid, name, rev string) (*Attachment, error) {
	if docid == "" {
		return nil, fmt.Errorf("couchdb.GetAttachment: empty docid")
	}
	if name == "" {
		return nil, fmt.Errorf("couchdb.GetAttachment: empty attachment Name")
	}

	resp, err := db.request("GET", revpath(rev, db.name, docid, name), nil)
	if err != nil {
		return nil, err
	}
	att, err := attFromHeaders(name, resp)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	att.Body = resp.Body
	return att, nil
}

// AttachmentMeta requests attachment metadata.
// The rev argument can be left empty to retrieve the latest revision.
// The returned attachment's Body is always nil.
func (db *DB) AttachmentMeta(docid, name, rev string) (*Attachment, error) {
	if docid == "" {
		return nil, fmt.Errorf("couchdb.GetAttachment: empty docid")
	}
	if name == "" {
		return nil, fmt.Errorf("couchdb.GetAttachment: empty attachment Name")
	}

	path := revpath(rev, db.name, docid, name)
	resp, err := db.closedRequest("HEAD", path, nil)
	if err != nil {
		return nil, err
	}
	return attFromHeaders(name, resp)
}

// PutAttachment creates or updates an attachment.
// To create an attachment on a non-existing document, pass an empty rev.
func (db *DB) PutAttachment(docid string, att *Attachment, rev string) (newrev string, err error) {
	if docid == "" {
		return rev, fmt.Errorf("couchdb.PutAttachment: empty docid")
	}
	if att.Name == "" {
		return rev, fmt.Errorf("couchdb.PutAttachment: empty attachment Name")
	}
	if att.Body == nil {
		return rev, fmt.Errorf("couchdb.PutAttachment: nil attachment Body")
	}

	path := revpath(rev, db.name, docid, att.Name)
	req, err := db.newRequest("PUT", path, att.Body)
	if err != nil {
		return rev, err
	}
	req.Header.Set("content-type", att.Type)

	resp, err := db.http.Do(req)
	if err != nil {
		return rev, err
	}
	var result struct{ Rev string }
	if err := readBody(resp, &result); err != nil {
		// TODO: close body if it implements io.ReadCloser
		return rev, fmt.Errorf("couchdb.PutAttachment: couldn't decode rev: %v", err)
	}
	return result.Rev, nil
}

// DeleteAttachment removes an attachment.
func (db *DB) DeleteAttachment(docid, name, rev string) (newrev string, err error) {
	if docid == "" {
		return rev, fmt.Errorf("couchdb.PutAttachment: empty docid")
	}
	if name == "" {
		return rev, fmt.Errorf("couchdb.PutAttachment: empty name")
	}

	path := revpath(rev, db.name, docid, name)
	resp, err := db.closedRequest("DELETE", path, nil)
	return responseRev(resp, err)
}

func attFromHeaders(name string, resp *http.Response) (*Attachment, error) {
	att := &Attachment{Name: name, Type: resp.Header.Get("content-type")}
	md5 := resp.Header.Get("content-md5")
	if md5 != "" {
		if len(md5) < 22 || len(md5) > 24 {
			return nil, fmt.Errorf("couchdb: Content-MD5 header has invalid size %d", len(md5))
		}
		sum, err := base64.StdEncoding.DecodeString(md5)
		if err != nil {
			return nil, fmt.Errorf("couchdb: invalid base64 in Content-MD5 header: %v", err)
		}
		att.MD5 = sum
	}
	return att, nil
}
