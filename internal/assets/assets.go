package assets

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var Files embed.FS

func Web() fs.FS {
	f, err := fs.Sub(Files, "dist")
	if err != nil {
		panic(err)
	}
	return f
}
