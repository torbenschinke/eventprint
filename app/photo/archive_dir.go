package photo

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ArchiveUsage beschreibt, was im Archiv liegt.
type ArchiveUsage struct {
	// Files ist die Anzahl der Bilddateien.
	Files int

	// Bytes ist ihr Platzbedarf.
	Bytes int64
}

// partialPrefix kennzeichnet halb geschriebene Dateien.
//
// [NewDirArchive] schreibt erst unter diesem Namen und benennt danach um. Eine
// solche Datei ist kein Bild: Sie darf weder im Export landen noch als Bestand
// gezaehlt werden.
const partialPrefix = ".partial-"

// archiveEntries listet die Bilddateien eines Archivverzeichnisses.
//
// Bewusst nur die oberste Ebene und nur gewoehnliche Dateien. Wer hier
// rekursiv liefe oder Verweisen folgte, koennte spaeter beim Loeschen weit
// ausserhalb des Archivs landen - und das ist eine Funktion, die Dateien
// endgueltig entfernt.
func archiveEntries(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("cannot read photo archive: %w", err)
	}

	var names []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Type() liefert die Bits ohne Aufloesung von Verweisen. Ein Symlink
		// ist damit erkennbar und wird uebergangen.
		if entry.Type()&fs.ModeSymlink != 0 || !entry.Type().IsRegular() {
			continue
		}

		if strings.HasPrefix(entry.Name(), partialPrefix) {
			continue
		}

		names = append(names, entry.Name())
	}

	// Der Dateiname beginnt mit der Foto-ID und die traegt den Zeitstempel.
	// Alphabetisch ist damit chronologisch, und das ZIP ist geordnet.
	sort.Strings(names)

	return names, nil
}

// usageOf zaehlt Bestand und Platzbedarf.
func usageOf(fsys fs.FS) (ArchiveUsage, error) {
	names, err := archiveEntries(fsys)
	if err != nil {
		return ArchiveUsage{}, err
	}

	var usage ArchiveUsage

	for _, name := range names {
		info, err := fs.Stat(fsys, name)
		if err != nil {
			// Eine Datei, die zwischen Auflisten und Messen verschwindet, ist
			// kein Grund, die ganze Auskunft zu verweigern.
			continue
		}

		usage.Files++
		usage.Bytes += info.Size()
	}

	return usage, nil
}

// writeArchiveZip schreibt alle Bilder als ZIP nach dst.
//
// Strömend und nicht in den Speicher: Ein Fotoarchiv einer Feier wiegt
// mehrere Gigabyte, und die Fotobox hat vier davon insgesamt.
func writeArchiveZip(fsys fs.FS, dst io.Writer) (int, error) {
	names, err := archiveEntries(fsys)
	if err != nil {
		return 0, err
	}

	zw := zip.NewWriter(dst)

	written := 0

	for _, name := range names {
		if err := copyIntoZip(fsys, zw, name); err != nil {
			_ = zw.Close()

			return written, err
		}

		written++
	}

	if err := zw.Close(); err != nil {
		return written, fmt.Errorf("cannot finish zip: %w", err)
	}

	return written, nil
}

func copyIntoZip(fsys fs.FS, zw *zip.Writer, name string) error {
	src, err := fsys.Open(name)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", name, err)
	}

	defer src.Close()

	info, err := src.(fs.File).Stat()
	if err != nil {
		return fmt.Errorf("cannot stat %s: %w", name, err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("cannot describe %s: %w", name, err)
	}

	header.Name = name

	// Store statt Deflate: JPEG ist bereits komprimiert. Es erneut zu pressen
	// kostet auf einem Raspberry Pi spuerbar Rechenzeit und spart nichts.
	header.Method = zip.Store

	w, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("cannot add %s: %w", name, err)
	}

	if _, err := io.Copy(w, src); err != nil {
		return fmt.Errorf("cannot write %s: %w", name, err)
	}

	return nil
}

// purgeDir entfernt alle Bilddateien und meldet, was frei wurde.
func purgeDir(dir string) (ArchiveUsage, error) {
	fsys := os.DirFS(dir)

	names, err := archiveEntries(fsys)
	if err != nil {
		return ArchiveUsage{}, err
	}

	var freed ArchiveUsage

	for _, name := range names {
		info, err := fs.Stat(fsys, name)
		if err != nil {
			continue
		}

		// filepath.Join mit einem Namen aus ReadDir kann das Verzeichnis nicht
		// verlassen; ein Name enthaelt nie einen Trenner.
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return freed, fmt.Errorf("cannot delete %s: %w", name, err)
		}

		freed.Files++
		freed.Bytes += info.Size()
	}

	return freed, nil
}

// ArchiveZipName bildet den Dateinamen des Exports.
func ArchiveZipName(now time.Time) string {
	return "fotoarchiv-" + now.Format("2006-01-02") + ".zip"
}
