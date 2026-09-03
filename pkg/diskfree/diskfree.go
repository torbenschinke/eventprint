// Package diskfree beantwortet, wie viel Platz auf dem Datenträger liegt.
//
// Die Fotobox läuft auf einer SD-Karte. Läuft die voll, nimmt sie keine Fotos
// mehr an – und zwar mitten auf der Feier, ohne dass jemand vorgewarnt wäre.
package diskfree

import (
	"fmt"
	"syscall"
)

// Usage ist die Belegung eines Dateisystems.
type Usage struct {
	// TotalBytes ist die Größe des Dateisystems.
	TotalBytes int64

	// FreeBytes ist der für nicht-privilegierte Nutzer verfügbare Platz.
	FreeBytes int64
}

// UsedBytes ist der belegte Platz.
func (u Usage) UsedBytes() int64 { return u.TotalBytes - u.FreeBytes }

// Of misst das Dateisystem, auf dem path liegt.
//
// Bavail statt Bfree: Ein Teil des Dateisystems ist für root reserviert. Diese
// Reserve als frei zu melden, verspräche Platz, den die Fotobox als
// gewöhnlicher Dienst nie bekommt.
func Of(path string) (Usage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return Usage{}, fmt.Errorf("cannot measure %q: %w", path, err)
	}

	blockSize := int64(stat.Bsize)

	return Usage{
		TotalBytes: int64(stat.Blocks) * blockSize,
		FreeBytes:  int64(stat.Bavail) * blockSize,
	}, nil
}

// GiB formatiert eine Byte-Zahl als Gibibyte.
//
// Gibibyte und nicht Gigabyte, weil Dateisysteme und Betriebssystem in
// Zweierpotenzen rechnen: Eine Zahl, die von der im Dateimanager abweicht,
// stiftet nur Zweifel.
func GiB(bytes int64) string {
	const gib = 1024 * 1024 * 1024

	return fmt.Sprintf("%.2f GiB", float64(bytes)/gib)
}
