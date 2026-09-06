// Package webui embeds the built Vue SPA (copied into ./dist by the Makefile
// before `go build`), so the whole game ships as one binary.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the SPA file system rooted at dist/.
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
