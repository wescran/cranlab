package pdstest

import (
	"crypto/sha256"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
)

// Test-only credentials recognized exclusively by the loopback fake.
const Password = "fixture-password"
const AccessJWT = "fixture-access-token"

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if json.UnmarshalRead(r.Body, &in) != nil || in.Identifier != s.DID || in.Password != Password {
		xrpcError(w, 401, "AuthenticationRequired", "invalid login")
		return
	}
	writeJSON(w, map[string]string{"did": s.DID, "accessJwt": AccessJWT, "refreshJwt": "fixture-refresh-token"})
}

func (s *Server) uploadBlob(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+AccessJWT {
		xrpcError(w, 401, "AuthenticationRequired", "invalid token")
		return
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		xrpcError(w, 400, "InvalidRequest", "invalid upload")
		return
	}
	cid := fmt.Sprintf("bafkrei%x", sha256.Sum256(data))
	mime := r.Header.Get("Content-Type")
	s.PutBlob(cid, mime, data)
	writeJSON(w, map[string]any{"blob": map[string]any{"$type": "blob", "ref": map[string]string{"$link": cid}, "mimeType": mime, "size": len(data)}})
}

func (s *Server) createRecord(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+AccessJWT {
		xrpcError(w, 401, "AuthenticationRequired", "invalid token")
		return
	}
	var in struct {
		Repo       string         `json:"repo"`
		Collection string         `json:"collection"`
		RKey       string         `json:"rkey"`
		Record     jsontext.Value `json:"record"`
	}
	if json.UnmarshalRead(r.Body, &in) != nil || in.Repo != s.DID || in.Collection == "" || in.RKey == "" || in.Record.Kind() != '{' {
		xrpcError(w, 400, "InvalidRequest", "expected create")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[in.Collection][in.RKey]; exists {
		xrpcError(w, 400, "InvalidRequest", "record already exists")
		return
	}
	cid := s.putLocked(in.Collection, in.RKey, in.Record)
	writeJSON(w, map[string]string{"uri": s.uri(in.Collection, in.RKey), "cid": cid})
}

func (s *Server) putRecord(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+AccessJWT {
		xrpcError(w, 401, "AuthenticationRequired", "invalid token")
		return
	}
	var in struct {
		Repo       string         `json:"repo"`
		Collection string         `json:"collection"`
		RKey       string         `json:"rkey"`
		SwapRecord string         `json:"swapRecord"`
		Record     jsontext.Value `json:"record"`
	}
	if json.UnmarshalRead(r.Body, &in) != nil || in.Repo != s.DID || in.RKey == "" || in.Collection == "" || in.Record.Kind() != '{' || in.SwapRecord == "" {
		xrpcError(w, 400, "InvalidRequest", "expected conditional put")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records[in.Collection][in.RKey].cid != in.SwapRecord {
		xrpcError(w, 400, "InvalidSwap", "record changed")
		return
	}
	cid := s.putLocked(in.Collection, in.RKey, in.Record)
	writeJSON(w, map[string]string{"uri": s.uri(in.Collection, in.RKey), "cid": cid})
}
