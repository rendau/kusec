package app

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var adminDistCandidates = []string{
	"./admin-dist",
	"./apps/admin/dist",
}

// adminCacheControl: кэшируем статику админки в браузере не дольше 5 минут,
// чтобы новый деплой подхватывался быстро (бандл не имеет content-hash в имени).
const adminCacheControl = "max-age=300"

func NewAdminSPAHandler() http.Handler {
	distPath := resolveFirstExistingDir(adminDistCandidates...)
	if distPath == "" {
		return http.NotFoundHandler()
	}

	return newAdminSPAHandler(distPath)
}

func newAdminSPAHandler(distPath string) http.Handler {
	indexPath := filepath.Join(distPath, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", adminCacheControl)

		requestPath := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")

		filePath := indexPath
		if requestPath != "." {
			fullPath := filepath.Join(distPath, requestPath)
			if stat, err := os.Stat(fullPath); err == nil && !stat.IsDir() {
				filePath = fullPath
			}
		}

		serveAdminFile(w, r, filePath)
	})
}

type precompressedEncoding struct {
	name string
	ext  string
}

// precompressedEncodings — копии, которые apps/admin/scripts/compress.mjs
// пишет рядом с файлами сборки; предпочтительная первой.
var precompressedEncodings = []precompressedEncoding{
	{name: "br", ext: ".br"},
	{name: "gzip", ext: ".gz"},
}

func serveAdminFile(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Add("Vary", "Accept-Encoding")

	acceptEncoding := r.Header.Get("Accept-Encoding")
	for _, enc := range precompressedEncodings {
		if acceptsEncoding(acceptEncoding, enc.name) && serveEncodedFile(w, r, path, enc) {
			return
		}
	}

	http.ServeFile(w, r, path)
}

// serveEncodedFile отдаёт enc-копию файла path, если она есть. Content-Type и
// Last-Modified берутся от оригинала, чтобы закэшированная копия
// ревалидировалась одинаково независимо от полученной браузером кодировки.
func serveEncodedFile(w http.ResponseWriter, r *http.Request, path string, enc precompressedEncoding) bool {
	f, err := os.Open(path + enc.ext)
	if err != nil {
		return false
	}
	defer f.Close()

	stat, err := os.Stat(path)
	if err != nil {
		return false
	}

	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Encoding", enc.name)
	http.ServeContent(w, r, path, stat.ModTime(), f)

	return true
}

func acceptsEncoding(header, name string) bool {
	for part := range strings.SplitSeq(header, ",") {
		coding, params, _ := strings.Cut(part, ";")
		if !strings.EqualFold(strings.TrimSpace(coding), name) {
			continue
		}

		q, found := strings.CutPrefix(strings.TrimSpace(params), "q=")
		if !found {
			return true
		}
		weight, err := strconv.ParseFloat(q, 64)

		return err == nil && weight > 0
	}

	return false
}

func resolveFirstExistingDir(paths ...string) string {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			return path
		}
	}

	return ""
}
