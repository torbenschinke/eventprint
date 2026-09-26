package usb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Drive ist ein Dateisystem auf einem USB-Datenträger, das die Fotobox
// anbieten darf.
//
// Gemeint ist die Partition und nicht der Stick: Nur eine Partition hat ein
// Dateisystem, das sich einhängen lässt. Ein Stick ohne Partitionstabelle ist
// selbst diese Partition.
type Drive struct {
	// Path ist die Gerätedatei, z. B. "/dev/sda1". Sie ist die Kennung, mit
	// der die Oberfläche einen Stick an die Anwendungsfälle zurückgibt.
	Path string

	// Disk ist die Gerätedatei des ganzen Datenträgers, z. B. "/dev/sda".
	// Abschalten lässt sich nur der ganze Stick, nicht eine Partition.
	Disk string

	Label  string
	Vendor string
	Model  string

	// Size in Byte.
	Size   int64
	FSType string

	// MountPoint ist leer, solange der Stick nicht eingehängt ist.
	MountPoint string
}

// Title ist der Name, unter dem der Gastgeber seinen Stick wiedererkennt.
//
// Die Bezeichnung des Dateisystems zuerst, denn die hat er womöglich selbst
// vergeben. Hersteller und Modell sind der Rückfall; ein leerer Titel wäre in
// der Liste nicht anklickbar.
func (d Drive) Title() string {
	if d.Label != "" {
		return d.Label
	}

	if name := strings.TrimSpace(d.Vendor + " " + d.Model); name != "" {
		return name
	}

	return "USB-Stick"
}

// SizeText ist die Größe zum Anzeigen.
func (d Drive) SizeText() string {
	return FormatSize(d.Size)
}

// FormatSize formatiert eine Byteanzahl in deutscher Schreibweise.
//
// Dezimale Einheiten, weil die Hersteller so rechnen: Auf dem Stick steht
// "16 GB", und so soll er auch in der Liste heißen, nicht "14,9 GB".
func FormatSize(b int64) string {
	units := []string{"B", "kB", "MB", "GB", "TB"}

	v := float64(b)
	i := 0

	for v >= 1000 && i < len(units)-1 {
		v /= 1000
		i++
	}

	if i == 0 {
		return strconv.FormatInt(b, 10) + " B"
	}

	prec := 1
	if v >= 100 {
		prec = 0
	}

	return strings.Replace(strconv.FormatFloat(v, 'f', prec, 64), ".", ",", 1) + " " + units[i]
}

// DriveNotFoundError meldet, dass die Gerätedatei kein angebotener Stick ist.
//
// Ein Typ und keine Paketvariable, wie im restlichen Code: Die Oberfläche soll
// den Fall erkennen können (Stick abgezogen, Liste neu laden), ohne Paketzustand
// zu lesen.
type DriveNotFoundError struct {
	Path string
}

func (e DriveNotFoundError) Error() string {
	return "der USB-Stick " + e.Path + " ist nicht (mehr) angeschlossen"
}

// lsblkArgs sind die Argumente für lsblk.
//
// MOUNTPOINT und nicht MOUNTPOINTS: Die Einzahl verstehen auch ältere
// Versionen. Die Auswertung verträgt trotzdem beide Schreibweisen, denn neuere
// Versionen antworten je nach Spaltenwahl mit der einen oder anderen.
//
// Eine Funktion und keine Paketvariable, damit niemand die Liste zur Laufzeit
// verändern kann.
func lsblkArgs() []string {
	return []string{
		"--json", "--bytes",
		"--output", "NAME,PATH,RM,HOTPLUG,TRAN,TYPE,FSTYPE,LABEL,SIZE,MOUNTPOINT,MODEL,VENDOR",
	}
}

// isSupportedFS nennt die Dateisysteme, die der Kernel von Raspberry Pi OS
// schreiben kann. vfat und exfat sind das, was auf gekauften Sticks steht;
// ntfs3 ist seit Kernel 5.15 dabei, und die ext-Familie und btrfs kommen vor,
// wenn jemand eine USB-SSD von seinem Linux-Rechner mitbringt.
func isSupportedFS(fs string) bool {
	switch strings.ToLower(fs) {
	case "vfat", "exfat", "ntfs", "ntfs3", "ext4", "ext3", "ext2", "btrfs":
		return true
	default:
		return false
	}
}

// isSystemMountPoint erkennt Einhängepunkte, die verraten, dass ein
// Datenträger das System trägt.
//
// Ein Raspberry Pi 4 oder 5 bootet gern von einer USB-SSD. Die ist dann
// TRAN=usb und sähe aus wie ein Stick. Wer sie anbietet, lädt dazu ein, die
// Fotos auf die Systemplatte zu kopieren oder sie im laufenden Betrieb
// abzuschalten.
func isSystemMountPoint(mp string) bool {
	switch mp {
	case "":
		return false
	case "/", "/boot", "/boot/firmware", "/boot/efi", "/usr", "/var", "/home",
		"/etc", "/opt", "/srv", "/tmp", "/root", "[SWAP]":
		return true
	default:
		return strings.HasPrefix(mp, "/boot/")
	}
}

// lsblkOutput spiegelt die JSON-Ausgabe von lsblk.
type lsblkOutput struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Name        string        `json:"name"`
	Path        string        `json:"path"`
	RM          flexBool      `json:"rm"`
	Hotplug     flexBool      `json:"hotplug"`
	Tran        string        `json:"tran"`
	Type        string        `json:"type"`
	FSType      string        `json:"fstype"`
	Label       string        `json:"label"`
	Size        flexInt       `json:"size"`
	MountPoint  string        `json:"mountpoint"`
	MountPoints []string      `json:"mountpoints"`
	Model       string        `json:"model"`
	Vendor      string        `json:"vendor"`
	Children    []lsblkDevice `json:"children"`
}

// devPath liefert die Gerätedatei. Sehr alte lsblk kennen die Spalte PATH
// nicht; dort ergibt sie sich aus dem Namen.
func (d lsblkDevice) devPath() string {
	if d.Path != "" {
		return d.Path
	}

	return "/dev/" + d.Name
}

// mountPoint liefert den ersten Einhängepunkt, egal in welcher Schreibweise
// lsblk ihn meldet. Neuere Versionen melden ein Feld mit null-Einträgen.
func (d lsblkDevice) mountPoint() string {
	if d.MountPoint != "" {
		return d.MountPoint
	}

	for _, mp := range d.MountPoints {
		if mp != "" {
			return mp
		}
	}

	return ""
}

// carriesSystem prüft ein Gerät samt Kindern auf Systemeinhängepunkte.
func (d lsblkDevice) carriesSystem() bool {
	if isSystemMountPoint(d.MountPoint) {
		return true
	}

	for _, mp := range d.MountPoints {
		if isSystemMountPoint(mp) {
			return true
		}
	}

	for _, c := range d.Children {
		if c.carriesSystem() {
			return true
		}
	}

	return false
}

// flexBool liest true/false ebenso wie "1"/"0": Ältere lsblk geben alle Werte
// als Zeichenketten aus.
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(bytes.TrimSpace(data)), `"`)

	switch strings.ToLower(s) {
	case "true", "1":
		*b = true
	case "false", "0", "", "null":
		*b = false
	default:
		return fmt.Errorf("unerwarteter Wahrheitswert %s", data)
	}

	return nil
}

// flexInt liest Zahlen ebenso wie Zahlen in Anführungszeichen, aus demselben
// Grund wie flexBool.
type flexInt int64

func (n *flexInt) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(bytes.TrimSpace(data)), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("unerwartete Größe %s: %w", data, err)
	}

	*n = flexInt(v)

	return nil
}

// parseLsblk wählt aus der Ausgabe von lsblk die Laufwerke aus, die die
// Fotobox anbieten darf.
//
// Die Regeln sind bewusst streng. Ein fälschlich fehlender Stick fällt sofort
// auf und ist harmlos; eine fälschlich angebotene Systemplatte kann die
// Fotobox mitten auf der Feier unbrauchbar machen.
func parseLsblk(data []byte) ([]Drive, error) {
	var out lsblkOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("die Laufwerksliste von lsblk ist unlesbar: %w", err)
	}

	var drives []Drive

	for _, disk := range out.BlockDevices {
		if disk.Type != "disk" {
			continue
		}

		// Die Speicherkarte des Pi ist nie ein USB-Stick, auch wenn ein
		// Kartenleser sie als wechselbar meldet.
		if strings.HasPrefix(disk.Name, "mmcblk") || strings.HasPrefix(filepath.Base(disk.devPath()), "mmcblk") {
			continue
		}

		// Ist irgendein Teil des Datenträgers als System eingehängt, ist der
		// ganze Datenträger tabu, auch seine übrigen Partitionen: Abschalten
		// ließe sich nur der ganze Datenträger.
		if disk.carriesSystem() {
			continue
		}

		usbDisk := strings.EqualFold(disk.Tran, "usb") || bool(disk.RM) || bool(disk.Hotplug)

		parts := make([]lsblkDevice, 0, len(disk.Children))
		for _, c := range disk.Children {
			if c.Type == "part" {
				parts = append(parts, c)
			}
		}

		// Ohne Partitionstabelle trägt der Datenträger selbst das
		// Dateisystem. Das kommt bei Sticks vor, die jemand unter Linux mit
		// mkfs direkt auf /dev/sdX formatiert hat.
		if len(disk.Children) == 0 {
			parts = append(parts, disk)
		}

		for _, p := range parts {
			if !usbDisk && !bool(p.RM) && !bool(p.Hotplug) {
				continue
			}

			if !isSupportedFS(p.FSType) {
				continue
			}

			if strings.HasPrefix(p.Name, "mmcblk") {
				continue
			}

			drives = append(drives, Drive{
				Path:       p.devPath(),
				Disk:       disk.devPath(),
				Label:      strings.TrimSpace(p.Label),
				Vendor:     strings.TrimSpace(disk.Vendor),
				Model:      strings.TrimSpace(disk.Model),
				Size:       int64(p.Size),
				FSType:     strings.ToLower(p.FSType),
				MountPoint: p.mountPoint(),
			})
		}
	}

	return drives, nil
}
