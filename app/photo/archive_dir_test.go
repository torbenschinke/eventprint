package photo

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/archiv"
)

func archiveFS() fstest.MapFS {
	return fstest.MapFS{
		"1700000001_DSC1.jpg":     {Data: bytes.Repeat([]byte("a"), 100)},
		"1700000002_DSC2.jpg":     {Data: bytes.Repeat([]byte("b"), 200)},
		".partial-halbfertig":     {Data: bytes.Repeat([]byte("x"), 999)},
		"unterordner/verirrt.jpg": {Data: bytes.Repeat([]byte("c"), 50)},
	}
}

// TestPartialFilesAreNeverCounted haelt die Zusage der Ablage ein: Eine halb
// geschriebene Datei ist kein Bild. Sie zu zaehlen taeuschte Bestand vor, sie
// zu exportieren lieferte ein kaputtes Bild an die Gaeste.
func TestPartialFilesAreNeverCounted(t *testing.T) {
	usage, err := usageOf(archiveFS())
	if err != nil {
		t.Fatalf("usageOf: %v", err)
	}

	if usage.Files != 2 {
		t.Fatalf("Files = %d, erwartet 2 (die halbe Datei zaehlt nicht)", usage.Files)
	}

	if usage.Bytes != 300 {
		t.Fatalf("Bytes = %d, erwartet 300", usage.Bytes)
	}

	spec.Verified(t, archiv.RArchivPlatz)
}

// TestOnlyTheTopLevelIsTheArchive begruendet, warum nicht rekursiv gesucht
// wird: purgeDir loescht endgueltig, und was gelistet wird, wird geloescht.
func TestOnlyTheTopLevelIsTheArchive(t *testing.T) {
	names, err := archiveEntries(archiveFS())
	if err != nil {
		t.Fatalf("archiveEntries: %v", err)
	}

	for _, name := range names {
		if name == "unterordner/verirrt.jpg" || name == "unterordner" {
			t.Fatalf("die Suche verlaesst die oberste Ebene: %v", names)
		}
	}

	if len(names) != 2 {
		t.Fatalf("%d Eintraege, erwartet 2: %v", len(names), names)
	}
}

// TestZipContainsEveryPhotoOnce ist die Zusage des Exports.
func TestZipContainsEveryPhotoOnce(t *testing.T) {
	var buf bytes.Buffer

	n, err := writeArchiveZip(archiveFS(), &buf)
	if err != nil {
		t.Fatalf("writeArchiveZip: %v", err)
	}

	if n != 2 {
		t.Fatalf("%d Dateien geschrieben, erwartet 2", n)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("das erzeugte ZIP ist nicht lesbar: %v", err)
	}

	if len(zr.File) != 2 {
		t.Fatalf("%d Eintraege im ZIP, erwartet 2", len(zr.File))
	}

	// Die Reihenfolge ist chronologisch, weil der Dateiname mit der Foto-ID
	// beginnt und die den Zeitstempel traegt.
	if zr.File[0].Name != "1700000001_DSC1.jpg" || zr.File[1].Name != "1700000002_DSC2.jpg" {
		t.Fatalf("Reihenfolge stimmt nicht: %s, %s", zr.File[0].Name, zr.File[1].Name)
	}

	// Der Inhalt muss unveraendert ankommen; das Archiv ist das, was die Gaeste
	// bekommen.
	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatalf("cannot open entry: %v", err)
	}

	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("cannot read entry: %v", err)
	}

	if len(data) != 100 || data[0] != 'a' {
		t.Fatalf("der Inhalt kam veraendert an: %d Bytes", len(data))
	}

	spec.Verified(t, archiv.RArchivExport)
}

// TestEmptyArchiveYieldsAValidZip: Ein leeres Archiv darf keinen Fehler und
// keine kaputte Datei erzeugen.
func TestEmptyArchiveYieldsAValidZip(t *testing.T) {
	var buf bytes.Buffer

	n, err := writeArchiveZip(fstest.MapFS{}, &buf)
	if err != nil {
		t.Fatalf("writeArchiveZip: %v", err)
	}

	if n != 0 {
		t.Fatalf("%d Dateien, erwartet 0", n)
	}

	if _, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len())); err != nil {
		t.Fatalf("das leere ZIP ist nicht lesbar: %v", err)
	}
}

// TestPurgeRemovesOnlyTheArchive ist der gefaehrlichste Test im Projekt: Die
// Funktion loescht endgueltig. Geprueft wird deshalb ausdruecklich, was sie
// NICHT anfasst.
func TestPurgeRemovesOnlyTheArchive(t *testing.T) {
	dir := t.TempDir()

	schreibe := func(name string, size int) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(dir, name), bytes.Repeat([]byte("z"), size), 0o644); err != nil {
			t.Fatalf("cannot write %s: %v", name, err)
		}
	}

	schreibe("1700000001_a.jpg", 100)
	schreibe("1700000002_b.jpg", 200)
	schreibe(".partial-abc", 300)

	// Ein Unterordner mit Inhalt: Er darf unangetastet bleiben.
	unter := filepath.Join(dir, "wichtig")
	if err := os.MkdirAll(unter, 0o755); err != nil {
		t.Fatalf("cannot create dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(unter, "bleibt.jpg"), []byte("bleibt"), 0o644); err != nil {
		t.Fatalf("cannot write: %v", err)
	}

	freed, err := purgeDir(dir)
	if err != nil {
		t.Fatalf("purgeDir: %v", err)
	}

	if freed.Files != 2 || freed.Bytes != 300 {
		t.Fatalf("freigegeben %d Dateien / %d Bytes, erwartet 2 / 300", freed.Files, freed.Bytes)
	}

	rest, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read dir: %v", err)
	}

	uebrig := map[string]bool{}
	for _, e := range rest {
		uebrig[e.Name()] = true
	}

	if !uebrig["wichtig"] {
		t.Fatal("der Unterordner wurde geloescht")
	}

	if !uebrig[".partial-abc"] {
		t.Fatal("eine gerade entstehende Datei wurde geloescht; sie gehoert einem laufenden Import")
	}

	if uebrig["1700000001_a.jpg"] || uebrig["1700000002_b.jpg"] {
		t.Fatal("ein Bild ist nicht geloescht worden")
	}

	if _, err := os.Stat(filepath.Join(unter, "bleibt.jpg")); err != nil {
		t.Fatalf("der Inhalt des Unterordners wurde angetastet: %v", err)
	}
}

// TestPurgeIgnoresSymlinks: Ein Verweis im Archiv duerfte sonst dazu fuehren,
// dass weit ausserhalb geloescht wird.
func TestPurgeIgnoresSymlinks(t *testing.T) {
	dir := t.TempDir()
	fremd := t.TempDir()

	ziel := filepath.Join(fremd, "fremde-datei.jpg")
	if err := os.WriteFile(ziel, []byte("fremd"), 0o644); err != nil {
		t.Fatalf("cannot write: %v", err)
	}

	if err := os.Symlink(ziel, filepath.Join(dir, "1700000003_verweis.jpg")); err != nil {
		t.Skipf("Symlinks nicht verfuegbar: %v", err)
	}

	if _, err := purgeDir(dir); err != nil {
		t.Fatalf("purgeDir: %v", err)
	}

	if _, err := os.Stat(ziel); err != nil {
		t.Fatalf("die Datei hinter dem Verweis wurde geloescht: %v", err)
	}
}

// TestPurgeOnAnEmptyDirIsHarmless deckt den Fall ab, dass zweimal geloescht
// wird.
func TestPurgeOnAnEmptyDirIsHarmless(t *testing.T) {
	dir := t.TempDir()

	freed, err := purgeDir(dir)
	if err != nil {
		t.Fatalf("purgeDir: %v", err)
	}

	if freed.Files != 0 || freed.Bytes != 0 {
		t.Fatalf("freigegeben %+v, erwartet nichts", freed)
	}
}

func TestArchiveZipNameCarriesTheDate(t *testing.T) {
	got := ArchiveZipName(time.Date(2026, 9, 3, 20, 0, 0, 0, time.UTC))

	if want := "fotoarchiv-2026-09-03.zip"; got != want {
		t.Fatalf("= %q, erwartet %q", got, want)
	}
}

var _ fs.FS = fstest.MapFS{}
