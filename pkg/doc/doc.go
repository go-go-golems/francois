// Package doc holds francois's embedded help topics (francois help <slug>).
package doc

import (
	"embed"

	"github.com/go-go-golems/glazed/pkg/help"
)

//go:embed topics/*.md
var topics embed.FS

// AddDocToHelpSystem loads the embedded topics.
func AddDocToHelpSystem(hs *help.HelpSystem) error {
	return hs.LoadSectionsFromFS(topics, "topics")
}
