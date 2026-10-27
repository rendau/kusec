package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminSPAHandler_Routing(t *testing.T) {
	dist := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dist, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dist, "index.html"), []byte("<html>index</html>"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dist, "assets", "index-abc.js"), []byte("console.log(1)"), 0o644))

	handler := newAdminSPAHandler(dist)

	tests := []struct {
		path     string
		wantBody string
	}{
		{path: "/", wantBody: "<html>index</html>"},
		{path: "/secret", wantBody: "<html>index</html>"},
		{path: "/app/123", wantBody: "<html>index</html>"},
		{path: "/assets/index-abc.js", wantBody: "console.log(1)"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.wantBody, rec.Body.String())
			assert.Equal(t, adminCacheControl, rec.Header().Get("Cache-Control"))

			lastModified := rec.Header().Get("Last-Modified")
			require.NotEmpty(t, lastModified)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("If-Modified-Since", lastModified)
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusNotModified, rec.Code)
		})
	}
}

func TestAdminSPAHandler_Precompressed(t *testing.T) {
	dist := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dist, "assets"), 0o755))
	files := map[string]string{
		"index.html":             "<html>index</html>",
		"index.html.br":          "index-br",
		"index.html.gz":          "index-gz",
		"assets/index-abc.js":    "console.log(1)",
		"assets/index-abc.js.br": "js-br",
		"assets/index-abc.js.gz": "js-gz",
		"assets/style-abc.css":   "body{}",
	}
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dist, name), []byte(body), 0o644))
	}

	handler := newAdminSPAHandler(dist)

	tests := []struct {
		name           string
		path           string
		acceptEncoding string
		wantBody       string
		wantEncoding   string
		wantType       string
	}{
		{name: "br preferred", path: "/assets/index-abc.js", acceptEncoding: "gzip, deflate, br, zstd", wantBody: "js-br", wantEncoding: "br", wantType: "text/javascript"},
		{name: "gzip only", path: "/assets/index-abc.js", acceptEncoding: "gzip", wantBody: "js-gz", wantEncoding: "gzip", wantType: "text/javascript"},
		{name: "br refused", path: "/assets/index-abc.js", acceptEncoding: "br;q=0, gzip;q=0.8", wantBody: "js-gz", wantEncoding: "gzip", wantType: "text/javascript"},
		{name: "no encoding", path: "/assets/index-abc.js", acceptEncoding: "", wantBody: "console.log(1)", wantType: "text/javascript"},
		{name: "unknown encoding", path: "/assets/index-abc.js", acceptEncoding: "zstd", wantBody: "console.log(1)", wantType: "text/javascript"},
		{name: "no copies", path: "/assets/style-abc.css", acceptEncoding: "br, gzip", wantBody: "body{}", wantType: "text/css"},
		{name: "index", path: "/", acceptEncoding: "br", wantBody: "index-br", wantEncoding: "br", wantType: "text/html"},
		{name: "spa route", path: "/app/123", acceptEncoding: "GZIP", wantBody: "index-gz", wantEncoding: "gzip", wantType: "text/html"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.wantBody, rec.Body.String())
			assert.Equal(t, tt.wantEncoding, rec.Header().Get("Content-Encoding"))
			assert.True(t, strings.HasPrefix(rec.Header().Get("Content-Type"), tt.wantType), rec.Header().Get("Content-Type"))
			assert.Equal(t, "Accept-Encoding", rec.Header().Get("Vary"))
			assert.Equal(t, adminCacheControl, rec.Header().Get("Cache-Control"))

			lastModified := rec.Header().Get("Last-Modified")
			require.NotEmpty(t, lastModified)

			req = httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			req.Header.Set("If-Modified-Since", lastModified)
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusNotModified, rec.Code)
			assert.Empty(t, rec.Body.String())
		})
	}
}

func TestAcceptsEncoding(t *testing.T) {
	tests := []struct {
		header string
		name   string
		want   bool
	}{
		{header: "gzip, deflate, br", name: "br", want: true},
		{header: "gzip, deflate, br", name: "gzip", want: true},
		{header: "Gzip", name: "gzip", want: true},
		{header: "br;q=0.5", name: "br", want: true},
		{header: "br; q=0", name: "br", want: false},
		{header: "br;q=0.0, gzip", name: "br", want: false},
		{header: "br;q=bad", name: "br", want: false},
		{header: "x-br, gzip", name: "br", want: false},
		{header: "", name: "gzip", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.header+"/"+tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, acceptsEncoding(tt.header, tt.name))
		})
	}
}
