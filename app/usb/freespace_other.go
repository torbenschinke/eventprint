//go:build !linux && !darwin

package usb

import "errors"

// StatfsFreeSpace gibt es nur auf Linux und macOS. Die Fotobox läuft auf
// Linux; macOS braucht es für die Entwicklung. Anderswo meldet der Export
// lieber einen Fehler, als ohne Platzprüfung loszukopieren.
func StatfsFreeSpace(path string) (int64, error) {
	return 0, errors.New("freier Speicherplatz lässt sich auf diesem System nicht ermitteln")
}
