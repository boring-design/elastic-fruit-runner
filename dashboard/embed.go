package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"net/url"
)

//go:embed all:dist
var distFS embed.FS

// Handler returns an http.Handler that serves the embedded dashboard SPA.
// It serves static files from the Vite build output and falls back to
// index.html for any path that does not match a real file (SPA routing).
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("dashboard: embedded dist/ subtree not found: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "" || path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if the requested file exists in the embedded FS.
		f, err := sub.Open(path[1:])
		if err != nil {
			// Static assets (hashed filenames under /assets/, favicon, etc.)
			// should return 404 when missing, not the SPA shell.
			if len(path) > 8 && path[:8] == "/assets/" {
				http.NotFound(w, r)
				return
			}
			// SPA client-side route — serve index.html.
			// Shallow-copy the request to avoid mutating the original.
			r2 := new(http.Request)
			*r2 = *r
			r2.URL = new(url.URL)
			*r2.URL = *r.URL
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		_ = f.Close()

		fileServer.ServeHTTP(w, r)
	})
}
