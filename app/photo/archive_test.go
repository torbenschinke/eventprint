package photo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirArchiveStoresBytesVerbatim(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "originals")

	archive, err := NewDirArchive(dir)
	if err != nil {
		t.Fatalf("NewDirArchive: %v", err)
	}

	raw := []byte{0xff, 0xd8, 0xff, 0xe1, 0x00, 0x42}

	if err := archive("1788201767405-176029a2a2b47578", "DSC02301.JPG", raw); err != nil {
		t.Fatalf("archive: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Dateien = %d, erwartet 1", len(entries))
	}

	name := entries[0].Name()
	if name != "1788201767405-176029a2a2b47578_DSC02301.jpg" {
		t.Fatalf("Dateiname = %q", name)
	}

	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Byte für Byte: Das Archiv ist nur dann etwas wert, wenn es die Datei
	// der Kamera enthält und nicht eine neu kodierte Fassung davon.
	if string(got) != string(raw) {
		t.Fatalf("Inhalt = % x, erwartet % x", got, raw)
	}
}

// TestDirArchiveLeavesNoPartialFiles sichert ab, dass ein unterbrochener
// Schreibvorgang keine halbe Datei hinterlässt, die später als heiles Bild
// verteilt würde.
func TestDirArchiveLeavesNoPartialFiles(t *testing.T) {
	dir := t.TempDir()

	archive, err := NewDirArchive(dir)
	if err != nil {
		t.Fatalf("NewDirArchive: %v", err)
	}

	if err := archive("id", "a.jpg", []byte("x")); err != nil {
		t.Fatalf("archive: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".partial-") {
			t.Fatalf("Zwischendatei blieb liegen: %s", e.Name())
		}
	}
}

func TestArchiveFilename(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"gewöhnlich", "IMG_1234.jpg", "id_IMG_1234.jpg"},
		{"Endung wird vereinheitlicht", "IMG_1234.JPEG", "id_IMG_1234.jpeg"},
		{"Pfad wird verworfen", "/tmp/incoming/foto.png", "id_foto.png"},
		{"Umlaute und Leerzeichen", "Hochzeit Käthe.jpg", "id_Hochzeit_K_the.jpg"},
		{"ohne Endung", "scan", "id_scan.jpg"},
		{"leerer Name", "", "id.jpg"},
		{"versteckte Datei", ".htaccess", "id_htaccess.jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := archiveFilename("id", tt.in); got != tt.want {
				t.Fatalf("archiveFilename(%q) = %q, erwartet %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestArchiveFilenameStaysSortable hält fest, dass die alphabetische
// Sortierung im Dateimanager der zeitlichen entspricht.
func TestArchiveFilenameStaysSortable(t *testing.T) {
	early := archiveFilename("1788201767405-aaaa", "z.jpg")
	late := archiveFilename("1788201768000-0000", "a.jpg")

	if !(early < late) {
		t.Fatalf("%q sortiert nicht vor %q", early, late)
	}
}
