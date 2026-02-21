package static

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed css js fonts
var staticFiles embed.FS

// HTTPFileSystem returns an http.FileSystem rooted at the static directory.
// Use with gin's StaticFS: router.StaticFS("/gui/static", static.HTTPFileSystem())
func HTTPFileSystem() http.FileSystem {
	return http.FS(staticFiles)
}

// FS returns the raw embed.FS for direct file access if needed.
func FS() fs.FS {
	return staticFiles
}
