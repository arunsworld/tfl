package handlers

import (
	"io/fs"
	"net/http"
	"time"
)

func (h handlers) registerStatic(static fs.FS) {
	h.handler.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
		neuter(http.FileServer(http.FS(static)))))
}

var (
	cacheSince = time.Now().Format(http.TimeFormat)
	cacheUntil = time.Now().AddDate(0, 6, 0).Format(http.TimeFormat)
)

func neuter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
		// 	http.NotFound(w, r)
		// 	return
		// }

		w.Header().Set("Last-Modified", cacheSince)
		w.Header().Set("Expires", cacheUntil)

		next.ServeHTTP(w, r)
	})
}
