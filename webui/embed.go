// Package webui embeds the built console (Vite output of ../webui/dist) so the
// whole panel ships inside the single subrefine binary.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the embedded console tree rooted at dist/, or nil when the panel
// was not built (a `go build` without the webui step).
func Dist() fs.FS {
	out, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil
	}
	// An embed of an empty directory yields a tree with no entries.
	if entries, err := fs.ReadDir(out, "."); err != nil || len(entries) == 0 {
		return nil
	}
	return out
}
