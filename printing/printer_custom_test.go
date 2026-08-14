package printing

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomFilterPipelineSubmitsRawStream(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "commands.log")
	ppdDir := filepath.Join(dir, "ppd")
	if err := os.Mkdir(ppdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ppdDir, "CZ01.ppd"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	cupsfilter := writeTestCommand(t, dir, "cupsfilter", `
printf 'cupsfilter %s\n' "$*" >> "$EVENTPRINT_TEST_LOG"
printf 'raster-data'
`)
	filter := writeTestCommand(t, dir, "rastertocz01", `
printf 'filter %s PPD=%s\n' "$*" "$PPD" >> "$EVENTPRINT_TEST_LOG"
printf 'print-stream'
`)
	lp := writeTestCommand(t, dir, "lp", `
printf 'lp %s\n' "$*" >> "$EVENTPRINT_TEST_LOG"
printf 'request id is CZ01-42 (1 file(s))\n'
`)

	oldLP, oldCupsfilter, oldPPD := lpExecutable, cupsfilterExecutable, cupsPPDDirectory
	lpExecutable, cupsfilterExecutable, cupsPPDDirectory = lp, cupsfilter, ppdDir
	t.Cleanup(func() {
		lpExecutable, cupsfilterExecutable, cupsPPDDirectory = oldLP, oldCupsfilter, oldPPD
	})
	t.Setenv("EVENTPRINT_TEST_LOG", logFile)

	p := CUPSPrinter{Queue: "CZ01", CustomFilter: filter}
	result, err := p.Print(context.Background(), []byte("jpeg-data"), "photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if result.JobID != "CZ01-42" {
		t.Fatalf("JobID = %q, erwartet CZ01-42", result.JobID)
	}

	commands, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	log := string(commands)
	for _, expected := range []string{
		"PageSize=w288h432",
		"StpImageType=Photo",
		"StpPrintSpeed=LowSpeed",
		"PPD=" + filepath.Join(ppdDir, "CZ01.ppd"),
		"lp -d CZ01 -t photo -o raw",
	} {
		if !strings.Contains(log, expected) {
			t.Errorf("%q fehlt in:\n%s", expected, log)
		}
	}
}

func writeTestCommand(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
