// Package wifi verwaltet die Funkverbindung der Fotobox.
//
// Die Fotobox wird an fremden Orten aufgebaut - in einem Saal, einem Zelt, bei
// jemandem zu Hause. Das WLAN ist dort jedes Mal ein anderes, und wer sie
// aufbaut, hat weder Tastatur am Server noch Lust auf eine SSH-Sitzung. Die
// Verbindung gehoert deshalb in die Oberflaeche.
package wifi

import (
	"sort"
	"strconv"
	"strings"
)

// Network ist ein empfangenes Funknetz.
type Network struct {
	// SSID ist der Name des Netzes.
	SSID string

	// Signal ist die Feldstaerke in Prozent, 0 bis 100.
	Signal int

	// Secured meldet, ob das Netz ein Kennwort verlangt.
	Secured bool

	// Active meldet, ob die Fotobox gerade mit diesem Netz verbunden ist.
	Active bool
}

// Bars bildet die Feldstaerke auf vier Stufen ab.
//
// Prozentwerte sind fuer eine Entscheidung unbrauchbar: Niemand weiss, ob 47
// gut ist. Vier Stufen beantworten die einzige Frage, die sich vor Ort stellt -
// reicht es hier, oder muss die Box naeher ans Funknetz.
func (n Network) Bars() int {
	switch {
	case n.Signal >= 75:
		return 4
	case n.Signal >= 50:
		return 3
	case n.Signal >= 25:
		return 2
	case n.Signal > 0:
		return 1
	default:
		return 0
	}
}

// Status ist die aktuelle Verbindung.
type Status struct {
	// Connected meldet, ob eine Funkverbindung besteht.
	Connected bool

	// SSID ist das Netz, mit dem die Fotobox verbunden ist.
	SSID string

	// Signal ist die Feldstaerke in Prozent.
	Signal int

	// Device ist die Schnittstelle, etwa wlan0.
	Device string
}

// Bars bildet die Feldstaerke auf vier Stufen ab. Siehe [Network.Bars].
func (s Status) Bars() int {
	return Network{Signal: s.Signal}.Bars()
}

// unescapeTerse loest die Maskierung der Terse-Ausgabe von nmcli auf.
//
// nmcli trennt Felder mit Doppelpunkten und maskiert deshalb Doppelpunkte
// innerhalb eines Wertes als \: - und den Gegenschraegstrich selbst als \\.
// Eine SSID darf beides enthalten. Ohne diese Behandlung zerfiele der Name
// eines solchen Netzes in zwei Felder, und die Liste zeigte Unsinn.
func unescapeTerse(s string) string {
	var sb strings.Builder

	escaped := false
	for _, r := range s {
		if escaped {
			sb.WriteRune(r)
			escaped = false

			continue
		}

		if r == '\\' {
			escaped = true
			continue
		}

		sb.WriteRune(r)
	}

	return sb.String()
}

// splitTerse zerlegt eine Zeile der Terse-Ausgabe in ihre Felder.
//
// strings.Split waere falsch: Es zerschnitte auch maskierte Doppelpunkte.
func splitTerse(line string) []string {
	var (
		fields  []string
		current strings.Builder
		escaped bool
	)

	for _, r := range line {
		switch {
		case escaped:
			current.WriteRune('\\')
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == ':':
			fields = append(fields, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}

	fields = append(fields, current.String())

	return fields
}

// ParseNetworks wertet die Ausgabe von
//
//	nmcli -t -f IN-USE,SSID,SIGNAL,SECURITY device wifi list
//
// aus.
func ParseNetworks(out string) []Network {
	// Ein Funknetz erscheint einmal je Zugangspunkt und Frequenzband. Ein
	// Heimnetz mit zwei Baendern und einem Verstaerker steht so sechsmal in
	// der Liste. Vor Ort waere das unbrauchbar, deshalb wird nach Namen
	// zusammengefasst und die beste Feldstaerke behalten - das ist die, die
	// die Fotobox tatsaechlich bekommt.
	best := map[string]Network{}

	for line := range strings.Lines(out) {
		line = strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := splitTerse(line)
		if len(fields) < 4 {
			continue
		}

		ssid := unescapeTerse(fields[1])

		// Versteckte Netze melden einen leeren Namen. Sie liessen sich ohne
		// Eingabe des Namens ohnehin nicht waehlen.
		if strings.TrimSpace(ssid) == "" {
			continue
		}

		signal, err := strconv.Atoi(strings.TrimSpace(unescapeTerse(fields[2])))
		if err != nil {
			continue
		}

		security := strings.TrimSpace(unescapeTerse(fields[3]))

		net := Network{
			SSID:   ssid,
			Signal: signal,

			// Ein leeres Feld bedeutet offen; nmcli schreibt sonst etwa
			// "WPA2 WPA3" oder "WEP".
			Secured: security != "" && security != "--",
			Active:  strings.TrimSpace(unescapeTerse(fields[0])) == "*",
		}

		prev, seen := best[ssid]
		if !seen || net.Signal > prev.Signal {
			// Die Verbindung haengt am Namen, nicht am Zugangspunkt: Ist
			// irgendeiner davon der aktive, ist es dieses Netz.
			net.Active = net.Active || prev.Active
			best[ssid] = net

			continue
		}

		if net.Active && !prev.Active {
			prev.Active = true
			best[ssid] = prev
		}
	}

	nets := make([]Network, 0, len(best))
	for _, n := range best {
		nets = append(nets, n)
	}

	// Das verbundene Netz zuoberst, danach nach Feldstaerke. Wer die Seite
	// oeffnet, will zuerst wissen, woran er ist.
	sort.Slice(nets, func(i, j int) bool {
		if nets[i].Active != nets[j].Active {
			return nets[i].Active
		}

		if nets[i].Signal != nets[j].Signal {
			return nets[i].Signal > nets[j].Signal
		}

		return nets[i].SSID < nets[j].SSID
	})

	return nets
}

// ParseStatus wertet die Ausgabe von
//
//	nmcli -t -f DEVICE,TYPE,STATE,CONNECTION device status
//
// aus und ergaenzt die Feldstaerke aus der Netzliste.
func ParseStatus(deviceOut string, networks []Network) Status {
	for line := range strings.Lines(deviceOut) {
		fields := splitTerse(strings.TrimRight(line, "\r\n"))
		if len(fields) < 4 {
			continue
		}

		if unescapeTerse(fields[1]) != "wifi" {
			continue
		}

		state := unescapeTerse(fields[2])
		if !strings.HasPrefix(state, "connected") {
			continue
		}

		ssid := unescapeTerse(fields[3])
		status := Status{
			Connected: true,
			SSID:      ssid,
			Device:    unescapeTerse(fields[0]),
		}

		// Die Feldstaerke steht nicht in der Geraeteliste, sondern nur in der
		// Netzliste. Beides zusammenzufuehren ist der einzige Weg, sie ohne
		// einen zweiten Aufruf zu bekommen.
		for _, n := range networks {
			if n.SSID == ssid {
				status.Signal = n.Signal
				break
			}
		}

		return status
	}

	return Status{}
}
