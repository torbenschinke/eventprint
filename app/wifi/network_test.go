package wifi_test

import (
	"testing"

	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/app/wifi"
	"github.com/torbenschinke/eventprint/requirements/fun/netz"
)

// echte Ausgabe der Fotobox, aufgezeichnet mit
//
//	nmcli -t -f IN-USE,SSID,SIGNAL,SECURITY device wifi list
//
// Sie zeigt den Regelfall, der die naive Auswertung erledigt: EIN Funknetz
// erscheint sechsmal, einmal je Zugangspunkt und Frequenzband.
const echteAusgabe = ` :Schinke:82:WPA2 WPA3
 :Schinke:69:WPA2 WPA3
 :Schinke:64:WPA2 WPA3
*:Schinke:62:WPA2 WPA3
 :Schinke:55:WPA2 WPA3
 :Schinke:37:WPA2 WPA3
`

// TestOneNetworkAppearsOnce ist der Grund fuer die Zusammenfassung. Ohne sie
// stuenden sechs gleichnamige Zeilen zur Auswahl, und niemand wuesste, welche.
func TestOneNetworkAppearsOnce(t *testing.T) {
	nets := wifi.ParseNetworks(echteAusgabe)

	if len(nets) != 1 {
		t.Fatalf("%d Netze, erwartet 1: %+v", len(nets), nets)
	}

	got := nets[0]

	if got.SSID != "Schinke" {
		t.Fatalf("SSID = %q", got.SSID)
	}

	// Die beste Feldstaerke, nicht die des aktiven Zugangspunkts: Das ist die,
	// die die Fotobox an diesem Ort bekommen kann.
	if got.Signal != 82 {
		t.Fatalf("Signal = %d, erwartet 82 (die beste der sechs)", got.Signal)
	}

	// Der aktive Zugangspunkt war der mit 62. Die Verbindung haengt aber am
	// Namen: Ist irgendeiner aktiv, ist es dieses Netz.
	if !got.Active {
		t.Fatal("das verbundene Netz ist nicht als aktiv erkannt")
	}

	if !got.Secured {
		t.Fatal("ein WPA2/WPA3-Netz gilt als offen")
	}

	spec.Verified(t, netz.RNetzSuche)
}

// TestColonInSsidSurvives deckt die Maskierung ab. nmcli trennt Felder mit
// Doppelpunkten und maskiert deshalb Doppelpunkte im Namen; ohne Behandlung
// zerfiele so ein Name in zwei Felder und die Liste zeigte Unsinn.
func TestColonInSsidSurvives(t *testing.T) {
	out := ` :Bar\:Foo:70:WPA2
 :Backslash\\Netz:60:WPA2
`

	nets := wifi.ParseNetworks(out)
	if len(nets) != 2 {
		t.Fatalf("%d Netze, erwartet 2: %+v", len(nets), nets)
	}

	namen := map[string]bool{}
	for _, n := range nets {
		namen[n.SSID] = true
	}

	for _, want := range []string{"Bar:Foo", `Backslash\Netz`} {
		if !namen[want] {
			t.Fatalf("Name %q fehlt, gefunden: %+v", want, namen)
		}
	}
}

// TestOpenNetworkIsRecognised trennt offene von gesicherten Netzen. Ein
// falsches Ergebnis hiesse: Kennwortabfrage fuer ein offenes Netz oder ein
// Verbindungsversuch ohne Kennwort.
func TestOpenNetworkIsRecognised(t *testing.T) {
	nets := wifi.ParseNetworks(" :Gastnetz:55:\n :Sicher:55:WPA2\n :Strich:55:--\n")

	secured := map[string]bool{}
	for _, n := range nets {
		secured[n.SSID] = n.Secured
	}

	if secured["Gastnetz"] {
		t.Fatal("ein offenes Netz gilt als gesichert")
	}

	if secured["Strich"] {
		t.Fatal("-- bedeutet offen, wurde aber als gesichert gewertet")
	}

	if !secured["Sicher"] {
		t.Fatal("ein WPA2-Netz gilt als offen")
	}
}

// TestHiddenAndBrokenLinesAreSkipped haelt die Auswertung robust. Ein
// versteckter Name liesse sich ohnehin nicht anwaehlen, und eine unvollstaendige
// Zeile darf die ganze Liste nicht umbringen.
func TestHiddenAndBrokenLinesAreSkipped(t *testing.T) {
	out := ` ::70:WPA2
 :Kaputt
 :OhneSignal:abc:WPA2

 :Gut:70:WPA2
`

	nets := wifi.ParseNetworks(out)
	if len(nets) != 1 || nets[0].SSID != "Gut" {
		t.Fatalf("erwartet nur das gueltige Netz, bekam: %+v", nets)
	}
}

// TestActiveNetworkComesFirst haelt die Sortierung fest: Wer die Seite
// oeffnet, will zuerst wissen, woran er ist.
func TestActiveNetworkComesFirst(t *testing.T) {
	out := ` :Stark:95:WPA2
*:Verbunden:40:WPA2
 :Mittel:70:WPA2
`

	nets := wifi.ParseNetworks(out)

	if nets[0].SSID != "Verbunden" {
		t.Fatalf("erstes Netz ist %q, erwartet das verbundene", nets[0].SSID)
	}

	// Danach nach Feldstaerke.
	if nets[1].SSID != "Stark" || nets[2].SSID != "Mittel" {
		t.Fatalf("Reihenfolge nach der Feldstaerke stimmt nicht: %+v", nets)
	}
}

// TestBarsAnswerTheOnlyQuestionThatMatters: Prozentwerte sind vor Ort
// unbrauchbar, weil niemand weiss, ob 47 gut ist.
func TestBarsAnswerTheOnlyQuestionThatMatters(t *testing.T) {
	tests := []struct {
		signal int
		bars   int
	}{
		{signal: 0, bars: 0},
		{signal: 1, bars: 1},
		{signal: 24, bars: 1},
		{signal: 25, bars: 2},
		{signal: 49, bars: 2},
		{signal: 50, bars: 3},
		{signal: 74, bars: 3},
		{signal: 75, bars: 4},
		{signal: 100, bars: 4},
	}

	for _, tt := range tests {
		if got := (wifi.Network{Signal: tt.signal}).Bars(); got != tt.bars {
			t.Errorf("Signal %d = %d Balken, erwartet %d", tt.signal, got, tt.bars)
		}
	}
}

// TestStatusJoinsDeviceAndSignal: Die Feldstaerke steht nur in der Netzliste,
// der Verbindungszustand nur in der Geraeteliste. Erst beides zusammen ergibt
// die Anzeige, die vor Ort gebraucht wird.
func TestStatusJoinsDeviceAndSignal(t *testing.T) {
	devices := "wlan0:wifi:connected:Schinke\nlo:loopback:connected (externally):\neth0:ethernet:unavailable:\n"

	status := wifi.ParseStatus(devices, wifi.ParseNetworks(echteAusgabe))

	if !status.Connected {
		t.Fatal("die bestehende Verbindung wurde nicht erkannt")
	}

	if status.SSID != "Schinke" {
		t.Fatalf("SSID = %q", status.SSID)
	}

	if status.Device != "wlan0" {
		t.Fatalf("Device = %q", status.Device)
	}

	if status.Signal != 82 {
		t.Fatalf("Signal = %d, erwartet die Feldstaerke aus der Netzliste", status.Signal)
	}

	if status.Bars() != 4 {
		t.Fatalf("Bars = %d", status.Bars())
	}

	spec.Verified(t, netz.RNetzZustand)
}

// TestStatusWithoutConnection ist der Zustand beim Aufbau an einem neuen Ort.
func TestStatusWithoutConnection(t *testing.T) {
	devices := "wlan0:wifi:disconnected:\neth0:ethernet:unavailable:\n"

	status := wifi.ParseStatus(devices, nil)

	if status.Connected {
		t.Fatalf("ohne Verbindung gilt die Fotobox als verbunden: %+v", status)
	}

	if status.SSID != "" {
		t.Fatalf("SSID = %q, erwartet leer", status.SSID)
	}
}
