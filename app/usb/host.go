package usb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// host bündelt die Handgriffe am Betriebssystem, die mehrere Anwendungsfälle
// brauchen. Er hält selbst keinen Zustand: Ob ein Stick steckt und wo er
// eingehängt ist, fragt er jedes Mal neu, denn der Stick kann jederzeit
// gezogen werden.
type host struct {
	runner Runner
}

// drives liest die angebotenen Laufwerke.
func (h host) drives(ctx context.Context) ([]Drive, error) {
	out, err := run(ctx, h.runner, lsblkTimeout, "lsblk", lsblkArgs()...)
	if err != nil {
		return nil, fmt.Errorf("die angeschlossenen Laufwerke lassen sich nicht auflisten: %w", err)
	}

	return parseLsblk(out)
}

// find sucht ein angebotenes Laufwerk.
//
// Jeder Anwendungsfall, der eine Gerätedatei von außen bekommt, geht hier
// durch. So kann auch eine manipulierte Anfrage nie /dev/mmcblk0p2 einhängen
// oder beschreiben: Was lsblk nicht als Stick durchlässt, gibt es hier nicht.
func (h host) find(ctx context.Context, path string) (Drive, error) {
	if strings.TrimSpace(path) == "" {
		return Drive{}, errors.New("es wurde kein USB-Stick ausgewählt")
	}

	drives, err := h.drives(ctx)
	if err != nil {
		return Drive{}, err
	}

	for _, d := range drives {
		if d.Path == path {
			return d, nil
		}
	}

	return Drive{}, DriveNotFoundError{Path: path}
}

// mount hängt das Laufwerk ein, falls nötig, und liefert den Einhängepunkt.
//
// Erst bei Bedarf und nicht schon beim Einstecken: Ein eingehängter Stick,
// den jemand ohne Auswerfen abzieht, kann beschädigt werden. Je kürzer er
// eingehängt ist, desto kleiner das Zeitfenster.
func (h host) mount(ctx context.Context, d Drive) (string, error) {
	if d.MountPoint != "" {
		return d.MountPoint, nil
	}

	out, err := run(ctx, h.runner, mountTimeout, "udisksctl", "mount", "--no-user-interaction", "-b", d.Path)
	if err == nil {
		if mp := parseMountOutput(string(out), exists); mp != "" {
			return mp, nil
		}
	}

	// Ob udisksctl gescheitert ist oder nur anders formuliert hat: Ist der
	// Stick jetzt eingehängt, etwa weil ein zweiter Aufruf schneller war
	// (AlreadyMounted), zählt nur das Ergebnis. lsblk ist dafür die
	// verlässlichere Quelle als der Meldungstext.
	if again, findErr := h.find(ctx, d.Path); findErr == nil && again.MountPoint != "" {
		return again.MountPoint, nil
	}

	if err == nil {
		return "", fmt.Errorf("der USB-Stick %s wurde eingehängt, aber sein Einhängepunkt ist unbekannt: %s", d.Title(), strings.TrimSpace(string(out)))
	}

	// Ohne die polkit-Regel meldet udisks NotAuthorized. Den Hinweis braucht
	// der Betreiber, nicht der Gast; er steht deshalb nur in der Meldung und
	// führt direkt zur Lösung.
	if strings.Contains(string(out), "NotAuthorized") || strings.Contains(err.Error(), "NotAuthorized") {
		return "", fmt.Errorf("der USB-Stick %s darf nicht eingehängt werden; fehlt die polkit-Regel 51-eventprint-udisks.rules? %w", d.Title(), err)
	}

	return "", fmt.Errorf("der USB-Stick %s lässt sich nicht einhängen: %w", d.Title(), err)
}

// exists prüft, ob ein Pfad vorhanden ist.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseMountOutput liest den Einhängepunkt aus der Meldung von udisksctl.
//
// Ältere udisks schreiben "Mounted /dev/sda1 at /media/eventprint/STICK.",
// neuere lassen den Punkt weg. Ein Punkt am Ende kann aber auch zum Namen des
// Sticks gehören. Deshalb wird nachgesehen, welche der beiden Lesarten es im
// Dateisystem gibt; gibt es keine, gilt die ohne Punkt.
func parseMountOutput(out string, exists func(string) bool) string {
	for _, line := range strings.Split(out, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "Mounted ")
		if !ok {
			continue
		}

		// Die Gerätedatei enthält keine Leerzeichen, das erste " at " ist
		// also die Trennstelle, auch wenn der Name des Sticks "at" enthält.
		_, mp, ok := strings.Cut(rest, " at ")
		if !ok {
			continue
		}

		mp = strings.TrimSpace(mp)
		if !strings.HasPrefix(mp, "/") {
			continue
		}

		trimmed := strings.TrimSuffix(mp, ".")
		if trimmed != mp && !exists(trimmed) && exists(mp) {
			return filepath.Clean(mp)
		}

		return filepath.Clean(trimmed)
	}

	return ""
}
