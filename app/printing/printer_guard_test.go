package printing

import (
	"context"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestPrintRejectsWrongSizeBeforeLP(t *testing.T) {
	dir := t.TempDir()
	ppdDir := filepath.Join(dir, "ppd")
	if err := os.Mkdir(ppdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ppd := "*ImageableArea w288h432/4x6: \"17.040 0.000 320.880 440.640\"\n*Resolution 300dpi/300x300 DPI: \"\"\n"
	if err := os.WriteFile(filepath.Join(ppdDir, "CZ01.ppd"), []byte(ppd), 0o600); err != nil {
		t.Fatal(err)
	}
	lpMarker := filepath.Join(dir, "lp-called")
	lp := writeGuardCommand(t, dir, "lp", "touch \"$LP_MARKER\"\n")

	oldLP, oldPPD := lpExecutable, cupsPPDDirectory
	lpExecutable, cupsPPDDirectory = lp, ppdDir
	t.Cleanup(func() { lpExecutable, cupsPPDDirectory = oldLP, oldPPD })
	t.Setenv("LP_MARKER", lpMarker)

	p := CUPSPrinter{Queue: "CZ01"}
	if _, err := p.Print(context.Background(), solidJPEG(t, 1224, 1836, color.Black), "wrong.jpg"); err == nil {
		t.Fatal("falsche JPEG-Größe wurde akzeptiert")
	}
	if _, err := os.Stat(lpMarker); !os.IsNotExist(err) {
		t.Fatal("lp wurde trotz falscher JPEG-Größe aufgerufen")
	}
}

func writeGuardCommand(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
