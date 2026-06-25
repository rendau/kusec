package app

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var adminDistCandidates = []string{
	"./admin-dist",
	"./apps/admin/dist",
}

func NewAdminSPAHandler() http.Handler {
	distPath := resolveFirstExistingDir(adminDistCandidates...)
	if distPath == "" {
		return http.NotFoundHandler()
	}

	fileServer := http.FileServer(http.Dir(distPath))
	indexPath := filepath.Join(distPath, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Кэшируем статику админки в браузере не дольше 5 минут, чтобы новый
		// деплой подхватывался быстро (бандл не имеет content-hash в имени).
		w.Header().Set("Cache-Control", "max-age=300")

		requestPath := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")

		if requestPath == "." {
			http.ServeFile(w, r, indexPath)
			return
		}

		fullPath := filepath.Join(distPath, requestPath)
		if stat, err := os.Stat(fullPath); err == nil && !stat.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, indexPath)
	})
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
