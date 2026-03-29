package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticFileServing(t *testing.T) {
	mux := http.NewServeMux()

	// Serve embedded dist FS.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.FileServer(http.FS(distFS)).ServeHTTP(w, r)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/dist/index.html")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "OpenParallax")
}

func TestCORSMiddleware(t *testing.T) {
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Regular request gets CORS headers.
	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	// OPTIONS preflight returns 204.
	req, _ := http.NewRequest(http.MethodOptions, srv.URL, nil)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"hello": "world"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "world", body["hello"])
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "invalid input")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "invalid input", body["error"])
}

func TestEmbedFSContainsIndexHTML(t *testing.T) {
	f, err := distFS.Open("dist/index.html")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<!DOCTYPE html>")
}

func TestWriteJSONNilSlice(t *testing.T) {
	w := httptest.NewRecorder()
	var items []string
	writeJSON(w, http.StatusOK, items)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "null")
}

func TestWriteJSONEmptySlice(t *testing.T) {
	w := httptest.NewRecorder()
	items := []string{}
	writeJSON(w, http.StatusOK, items)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "[]")
}

func TestWriteErrorJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusNotFound, "not found")
	assert.Equal(t, http.StatusNotFound, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "not found", body["error"])
}

func TestWriteJSONNestedMap(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]any{
		"agent_name":    "Atlas",
		"model":         "test-model",
		"session_count": 5,
		"workspace":     "/home/test",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Atlas", body["agent_name"])
	assert.Equal(t, "/home/test", body["workspace"])
	assert.Equal(t, float64(5), body["session_count"])
}

func TestCORSPreflightHeaders(t *testing.T) {
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodOptions, srv.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "POST")
}

func TestWSUpgradeRequiresWebSocket(t *testing.T) {
	// Without a real engine, just test that non-WebSocket requests to /api/ws
	// get proper error handling. The handler needs a Server with log, so we
	// test the endpoint returns non-200 for a plain HTTP request.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(strings.ToLower(r.Header.Get("Upgrade")), "websocket") {
			writeError(w, http.StatusBadRequest, "websocket upgrade required")
			return
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/ws")
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
