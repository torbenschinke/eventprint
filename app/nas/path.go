package nas

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// cleanPath prüft einen Pfad relativ zur Freigabe. Die Wurzel ist ".".
//
// Pfade kommen aus der Oberfläche zurück, und nichts, was von dort kommt, soll
// aus der Freigabe herausführen oder in ihre Verwaltungsordner hinein.
func cleanPath(p string) (string, error) {
	p = strings.Trim(strings.ReplaceAll(p, `\`, "/"), "/")
	if p == "" || p == "." {
		return ".", nil
	}

	if !fs.ValidPath(p) {
		return "", fmt.Errorf("ungültiger Pfad %q", p)
	}

	for _, seg := range strings.Split(p, "/") {
		if hidden(seg) {
			return "", fmt.Errorf("ungültiger Pfad %q", p)
		}
	}

	return p, nil
}

// hidden erkennt Einträge, die niemand als Foto sucht.
//
// Synology legt zu jedem Ordner "@eaDir" mit Vorschaubildern und Metadaten an,
// dazu "#recycle" und "#snapshot"; die Bilder darin sind Kopien oder
// Gelöschtes. Windows und macOS hinterlassen ihre eigenen Verwaltungsordner.
func hidden(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") || strings.HasPrefix(name, "#") {
		return true
	}

	switch name {
	case "$RECYCLE.BIN", "System Volume Information", "lost+found":
		return true
	default:
		return false
	}
}

// isImage erkennt die druckbaren Formate an der Endung, unabhängig von der
// Schreibweise.
func isImage(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".heic", ".heif":
		return true
	default:
		return false
	}
}
