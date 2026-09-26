//go:build linux || darwin

package usb

import "syscall"

// StatfsFreeSpace liefert den für Nicht-Root-Nutzer freien Platz des
// Dateisystems, in dem path liegt.
//
// Bavail und nicht Bfree: Der Dienst läuft nicht als root, und bei ext4 hält
// das Dateisystem einen Teil für root zurück. Mit Bfree würde der Export
// beginnen und kurz vor Schluss an einem Platz scheitern, den es nur auf dem
// Papier gibt.
func StatfsFreeSpace(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}

	// Die Feldtypen unterscheiden sich zwischen Linux und macOS, deshalb die
	// ausdrückliche Umwandlung.
	return int64(st.Bavail) * int64(st.Bsize), nil
}
