package camera

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// complete meldet, ob die Datei fertig geschrieben ist.
//
// Die Prüfung sieht in das Bild selbst: Ein JPEG beginnt mit SOI (FFD8) und
// endet mit EOI (FFD9), ein PNG beginnt mit seiner Signatur und endet mit dem
// IEND-Chunk. Solange gphoto2 noch schreibt, fehlt das Ende – und genau das
// unterscheidet ein halbes Bild zuverlässig von einem ganzen.
//
// Vorher wurde stattdessen auf zwei Verzeichnisdurchläufe mit gleicher Größe
// gewartet. Das kostete immer mindestens einen zusätzlichen Takt und ließ
// trotzdem abgeschnittene Bilder durch, wenn der USB-Transfer kurz stockte.
func complete(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return false
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return endsWith(f, info.Size(), pngSignature, pngEnd)
	default:
		return endsWith(f, info.Size(), jpegStart, jpegEnd)
	}
}

var (
	jpegStart = []byte{0xFF, 0xD8}
	jpegEnd   = []byte{0xFF, 0xD9}

	pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	pngEnd       = []byte{'I', 'E', 'N', 'D', 0xAE, 0x42, 0x60, 0x82}
)

func endsWith(f *os.File, size int64, start, end []byte) bool {
	if size < int64(len(start)+len(end)) {
		return false
	}

	head := make([]byte, len(start))
	if _, err := f.ReadAt(head, 0); err != nil || !bytes.Equal(head, start) {
		return false
	}

	tail := make([]byte, len(end))
	if _, err := f.ReadAt(tail, size-int64(len(end))); err != nil {
		return false
	}
	return bytes.Equal(tail, end)
}
