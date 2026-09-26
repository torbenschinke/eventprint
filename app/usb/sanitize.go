package usb

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// maxSegment begrenzt die Länge eines Ordner- oder Dateinamens.
//
// FAT erlaubt 255 Zeichen je Name, Windows aber nur 260 für den ganzen Pfad.
// Ein Stick, dessen Ordner sich am Rechner des Gastgebers nicht öffnen lässt,
// ist so gut wie kein Stick.
const maxSegment = 120

// SanitizeFolder macht aus einem Ordnernamen wie "eventprint/2026-09-26
// Hochzeit Anna & Ben" einen Pfad, den FAT und Windows annehmen.
//
// "/" trennt Unterordner. Alles andere, was FAT verbietet, wird ersetzt statt
// abgelehnt: Der Name kommt aus dem Titel der Feier, und an einem
// Doppelpunkt in "Sommerfest 2026: Team Nord" soll der Export nicht
// scheitern. ".." verschwindet dabei von selbst, denn Punkte am Rand werden
// abgeschnitten; so kann der Name nie aus dem Stick hinausführen.
func SanitizeFolder(folder string) string {
	var segs []string

	for _, seg := range strings.Split(folder, "/") {
		if s := truncateRunes(sanitizeSegment(seg), maxSegment); s != "" {
			segs = append(segs, s)
		}
	}

	if len(segs) == 0 {
		return "eventprint"
	}

	return strings.Join(segs, "/")
}

// sanitizeFileName macht einen Dateinamen FAT-tauglich und behält dabei die
// Endung, denn ohne ".jpg" öffnet kein Rechner das Foto.
func sanitizeFileName(name string) string {
	name = sanitizeSegment(name)

	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)

	if utf8.RuneCountInString(ext) > 10 {
		// Keine Endung, sondern ein Punkt mitten im Namen.
		stem, ext = name, ""
	}

	stem = truncateRunes(stem, maxSegment-utf8.RuneCountInString(ext))

	return reserveSafe(stem + ext)
}

// sanitizeSegment ersetzt, was FAT oder Windows in einem Namen verbieten.
//
// Trennähnliche Zeichen werden zum Bindestrich, weil "12:30" als "12-30"
// lesbar bleibt; der Rest wird zum Unterstrich. Steuerzeichen fallen weg.
func sanitizeSegment(s string) string {
	var b strings.Builder

	for _, r := range s {
		switch {
		case r < 0x20 || r == 0x7f:
			continue
		case r == '/' || r == '\\' || r == ':' || r == '|':
			b.WriteRune('-')
		case r == '*' || r == '?' || r == '"' || r == '<' || r == '>':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}

	return reserveSafe(strings.Trim(b.String(), " ."))
}

// truncateRunes kürzt auf n Zeichen, nicht Bytes, damit kein Umlaut
// halbiert wird. Danach werden Ränder erneut bereinigt, weil der Schnitt
// genau hinter einem Leerzeichen liegen kann.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}

	if utf8.RuneCountInString(s) <= n {
		return s
	}

	return strings.Trim(string([]rune(s)[:n]), " .")
}

// reserveSafe entschärft die Gerätenamen von Windows. Eine Datei "CON.jpg"
// legt Linux klaglos an, am Rechner des Gastgebers ließe sie sich aber weder
// öffnen noch löschen.
func reserveSafe(name string) string {
	stem, _, _ := strings.Cut(name, ".")

	switch strings.ToUpper(strings.TrimSpace(stem)) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return "_" + name
	default:
		return name
	}
}
