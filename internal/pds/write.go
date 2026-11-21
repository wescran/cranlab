package pds

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
)

// ErrSwapConflict means the document has changed since it was read.
var ErrSwapConflict = errors.New("document CID changed")

// Never follow a redirect carrying credentials, or include response text (which
// can echo credentials) in an error. Sessions are scoped to one sync run.
func (c *Client) procedure(ctx context.Context, method, token string, input, output any) error {
	data, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("%s: invalid input", method)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/xrpc/"+method, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	hc := *c.hc
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("%s: request failed", method)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 400 && method == "com.atproto.repo.putRecord" {
			var body struct {
				Error string `json:"error"`
			}
			if json.UnmarshalRead(resp.Body, &body) == nil && body.Error == "InvalidSwap" {
				return ErrSwapConflict
			}
		}
		return fmt.Errorf("%s: HTTP %d", method, resp.StatusCode)
	}
	if err := json.UnmarshalRead(resp.Body, output); err != nil {
		return fmt.Errorf("%s: invalid response", method)
	}
	return nil
}

func (c *Client) CreateSession(ctx context.Context, did, password string) (string, error) {
	var session struct {
		DID       string `json:"did"`
		AccessJWT string `json:"accessJwt"`
	}
	err := c.procedure(ctx, "com.atproto.server.createSession", "", map[string]string{"identifier": did, "password": password}, &session)
	if err != nil {
		return "", err
	}
	if session.DID != did || session.AccessJWT == "" {
		return "", errors.New("unexpected session identity or missing token")
	}
	return session.AccessJWT, nil
}

// LinkDocument patches only bskyPostRef, conditional on the exact version read.
func (c *Client) LinkDocument(ctx context.Context, token, did, key string, doc Record[Document], ref StrongRef) error {
	if doc.CID == "" || doc.URI != "at://"+did+"/"+DocumentCollection+"/"+key {
		return errors.New("missing document identity")
	}
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(doc.Raw, &fields); err != nil || fields == nil {
		return errors.New("invalid document")
	}
	raw, err := json.Marshal(ref)
	if err != nil {
		return err
	}
	fields["bskyPostRef"] = raw
	var result StrongRef
	err = c.procedure(ctx, "com.atproto.repo.putRecord", token, map[string]any{"repo": did, "collection": DocumentCollection, "rkey": key, "record": fields, "swapRecord": doc.CID}, &result)
	if err != nil {
		return err
	}
	if result.URI != doc.URI || result.CID == "" {
		return errors.New("unexpected putRecord result")
	}
	return nil
}
