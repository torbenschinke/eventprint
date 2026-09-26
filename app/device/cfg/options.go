// Package cfgdevice verdrahtet alle Kontexte zu einem lauffähigen Gerät.
//
// Hier und nur hier werden Adapter gewählt: Dateiablage, CUPS, udisks,
// NetworkManager. Die Kontexte selbst kennen nur ihre Ports.
package cfgdevice

import (
	"os"
	"path/filepath"
)

// Options sind die Betriebsparameter, die nicht in die Einstellungen gehören,
// weil sie das Gerät selbst beschreiben und nicht seine Bedienung.
type Options struct {
	// DataDir hält Fotos, Aufträge und Einstellungen. Unter systemd ist das
	// STATE_DIRECTORY, also /var/lib/eventprint.
	DataDir string

	// RuntimeDir hält den Kiosk-Zustand. Unter systemd ist das
	// RUNTIME_DIRECTORY, also /run/eventprint: flüchtig über einen Neustart
	// des Geräts, beständig über einen Neustart des Dienstes.
	RuntimeDir string

	// CacheDir hält die Vorschaubilder der Galerien.
	CacheDir string

	// CameraDir ist das Ziel des Kamera-Tetherings. Leer schaltet die
	// Kamera ab.
	CameraDir string

	// Vorgaben für ein frisch eingerichtetes Gerät. Sie werden nur
	// übernommen, solange in den Einstellungen nichts steht, damit sich eine
	// Box über /etc/default/eventprint vorbereiten lässt, ohne Tokens auf
	// dem Touchscreen zu tippen.
	PrinterQueue string
	RelayURL     string
	RelayToken   string
}

// OptionsFromEnv liest die Parameter aus der Umgebung.
//
//	EVENTPRINT_DATA_DIR         Datenverzeichnis (Vorgabe: STATE_DIRECTORY)
//	EVENTPRINT_RUNTIME_DIR      Laufzeitverzeichnis (Vorgabe: RUNTIME_DIRECTORY)
//	EVENTPRINT_CAMERA_DIR       Tethering-Ziel; "off" schaltet die Kamera ab
//	EVENTPRINT_PRINTER          CUPS-Warteschlange für ein frisches Gerät
//	EVENTPRINT_RELAY_URL        Upload-Dienst für ein frisches Gerät
//	EVENTPRINT_RELAY_TOKEN      Token des Geräts beim Upload-Dienst
func OptionsFromEnv() Options {
	data := firstNonEmpty(os.Getenv("EVENTPRINT_DATA_DIR"), os.Getenv("STATE_DIRECTORY"))
	if data == "" {
		home, _ := os.UserHomeDir()
		data = filepath.Join(home, ".local", "share", "eventprint")
	}

	runtime := firstNonEmpty(os.Getenv("EVENTPRINT_RUNTIME_DIR"), os.Getenv("RUNTIME_DIRECTORY"))
	if runtime == "" {
		runtime = filepath.Join(os.TempDir(), "eventprint-run")
	}

	camera := os.Getenv("EVENTPRINT_CAMERA_DIR")
	switch camera {
	case "":
		camera = filepath.Join(data, "camera")
	case "off":
		camera = ""
	}

	return Options{
		DataDir:      data,
		RuntimeDir:   runtime,
		CacheDir:     filepath.Join(data, "cache"),
		CameraDir:    camera,
		PrinterQueue: os.Getenv("EVENTPRINT_PRINTER"),
		RelayURL:     os.Getenv("EVENTPRINT_RELAY_URL"),
		RelayToken:   os.Getenv("EVENTPRINT_RELAY_TOKEN"),
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}

	return ""
}
