package theme

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/egoist/mygo/ui"
)

// Inter and Geist Mono, both under the SIL Open Font License.
//
//go:embed fonts/*.ttf
var fontFiles embed.FS

// families names the bundled families by their files' prefixes.
var families = map[string]string{
	"Inter": "Inter", "GeistMono": "Geist Mono",
}

// RegisterFonts makes the bundled families available to the views. Chinese,
// Japanese and Korean text falls back to the system's sans (FontUI).
func RegisterFonts() error {
	entries, err := fs.ReadDir(fontFiles, "fonts")
	if err != nil {
		return err
	}
	for _, e := range entries {
		data, err := fontFiles.ReadFile("fonts/" + e.Name())
		if err != nil {
			return err
		}
		var family string
		for prefix, f := range families {
			if len(e.Name()) > len(prefix) && e.Name()[:len(prefix)+1] == prefix+"-" {
				family = f
			}
		}
		if err := ui.RegisterFont(data, family); err != nil {
			return fmt.Errorf("register %s: %w", e.Name(), err)
		}
	}
	return nil
}
