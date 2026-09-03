package cfgphotobox

import (
	"errors"
	"net"
	"testing"
)

func ipNet(cidr string) net.Addr {
	ip, n, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}

	n.IP = ip

	return n
}

// TestIsLocalOnlyCatchesTheAddressNagoDerivesItself ist der Fall, der im
// Betrieb auftritt: Der Kiosk-Browser verbindet sich als Erster, und Nago
// leitet daraus die oeffentliche Adresse ab.
func TestIsLocalOnlyCatchesTheAddressNagoDerivesItself(t *testing.T) {
	tests := []struct {
		url   string
		local bool
	}{
		{url: "http://localhost:3000/upload", local: true},
		{url: "http://127.0.0.1:3000/upload", local: true},
		{url: "http://[::1]:3000/upload", local: true},
		{url: "http://0.0.0.0:3000/upload", local: true},
		{url: "http://192.168.20.36:3000/upload"},
		{url: "https://fotobox.example.de/upload"},
	}

	for _, tt := range tests {
		if got := isLocalOnly(tt.url); got != tt.local {
			t.Errorf("isLocalOnly(%q) = %v, erwartet %v", tt.url, got, tt.local)
		}
	}
}

// TestWithLanHostKeepsEverythingButTheHost: Schema, Port und Pfad muessen
// stehen bleiben, sonst zeigt der QR-Code auf die falsche Seite.
func TestWithLanHostKeepsEverythingButTheHost(t *testing.T) {
	got := withLANHost("http://localhost:3000/upload", "192.168.20.36")

	if want := "http://192.168.20.36:3000/upload"; got != want {
		t.Fatalf("= %q, erwartet %q", got, want)
	}

	// Eine bereits brauchbare Adresse darf nicht angefasst werden. Sonst
	// ueberschriebe die Erkennung eine von Hand gesetzte oeffentliche Adresse.
	fest := "https://fotobox.example.de/upload"
	if got := withLANHost(fest, "192.168.20.36"); got != fest {
		t.Fatalf("eine gesetzte Adresse wurde veraendert: %q", got)
	}

	// Ohne erkannte Adresse bleibt alles, wie es war.
	if got := withLANHost("http://localhost:3000/upload", ""); got != "http://localhost:3000/upload" {
		t.Fatalf("ohne LAN-Adresse wurde veraendert: %q", got)
	}
}

// TestLanAddressPicksAUsableInterface haelt die Auswahlregeln fest.
func TestLanAddressPicksAUsableInterface(t *testing.T) {
	tests := []struct {
		name  string
		ifs   []netIface
		want  string
		grund string
	}{
		{
			name: "gewoehnlicher Fall",
			ifs: []netIface{
				{Flags: net.FlagUp | net.FlagLoopback, Addrs: []net.Addr{ipNet("127.0.0.1/8")}},
				{Flags: net.FlagUp, Addrs: []net.Addr{ipNet("192.168.20.36/24")}},
			},
			want: "192.168.20.36",
		},
		{
			name:  "abgeschaltete Schnittstelle zaehlt nicht",
			ifs:   []netIface{{Addrs: []net.Addr{ipNet("192.168.1.5/24")}}},
			want:  "",
			grund: "eine Schnittstelle ohne FlagUp hat keine gueltige Verbindung",
		},
		{
			name: "selbstvergebene Adresse zaehlt nicht",
			ifs: []netIface{
				{Flags: net.FlagUp, Addrs: []net.Addr{ipNet("169.254.10.3/16")}},
			},
			want:  "",
			grund: "169.254.0.0/16 bedeutet, dass gar kein DHCP geantwortet hat",
		},
		{
			name: "selbstvergebene Adresse wird uebersprungen, echte gewinnt",
			ifs: []netIface{
				{Flags: net.FlagUp, Addrs: []net.Addr{ipNet("169.254.10.3/16")}},
				{Flags: net.FlagUp, Addrs: []net.Addr{ipNet("10.0.0.7/24")}},
			},
			want: "10.0.0.7",
		},
		{
			name: "nur Rueckkopplung",
			ifs:  []netIface{{Flags: net.FlagUp | net.FlagLoopback, Addrs: []net.Addr{ipNet("127.0.0.1/8")}}},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lanAddressFrom(tt.ifs)
			if got != tt.want {
				t.Fatalf("= %q, erwartet %q. %s", got, tt.want, tt.grund)
			}
		})
	}
}

// TestLanAddressSurvivesABrokenSystem: Ein Fehler beim Abfragen der
// Schnittstellen darf die Fotobox nicht umbringen.
func TestLanAddressSurvivesABrokenSystem(t *testing.T) {
	if got := lanAddress(func() ([]net.Interface, error) { return nil, errors.New("kaputt") }); got != "" {
		t.Fatalf("= %q, erwartet leer", got)
	}
}
