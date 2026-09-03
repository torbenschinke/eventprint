package cfgphotobox

import (
	"net"
	"strings"
)

// localHosts sind die Namen, unter denen die Fotobox nur der eigene Rechner
// erreicht.
var localHosts = []string{"localhost", "127.0.0.1", "[::1]", "0.0.0.0"}

// isLocalOnly meldet Adressen, die nur der Rechner selbst erreicht.
//
// Sie entstehen von selbst und ohne Zutun: Nago leitet die öffentliche Adresse
// aus der ersten Verbindung ab, und auf dem Kiosk kommt die erste Verbindung
// vom Rechner selbst. Im QR-Code steht dann eine Adresse, die kein Gast
// erreicht – und der Code sieht dabei aus wie jeder andere.
func isLocalOnly(rawURL string) bool {
	for _, host := range localHosts {
		if strings.Contains(rawURL, "//"+host) {
			return true
		}
	}

	return false
}

// netIface ist die Naht zum Betriebssystem.
//
// Ohne sie fragte der Test die echten Schnittstellen des Rechners ab, auf dem
// er gerade laeuft - und waere damit an jedem Ort ein anderer Test.
type netIface struct {
	Flags net.Flags
	Addrs []net.Addr
}

// lanAddress liefert die Adresse, unter der die Fotobox im örtlichen Netz
// erreichbar ist.
//
// Damit repariert sich der häufigste Fall von selbst: Die Fotobox steht mit
// den Gästen im selben WLAN, und deren Handys erreichen sie unter genau dieser
// Adresse. Sie von Hand einzutragen wäre ein Schritt, den beim Aufbau niemand
// mehr weiß.
func lanAddress(list func() ([]net.Interface, error)) string {
	ifaces, err := list()
	if err != nil {
		return ""
	}

	out := make([]netIface, 0, len(ifaces))
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		out = append(out, netIface{Flags: iface.Flags, Addrs: addrs})
	}

	return lanAddressFrom(out)
}

// lanAddressFrom waehlt die brauchbare Adresse aus.
//
// Nur IPv4: Eine IPv6-Adresse im QR-Code ist lang, fehleranfällig und im
// Heimnetz selten nötig.
func lanAddressFrom(ifaces []netIface) string {
	for _, iface := range ifaces {
		// Nur was oben ist und kein Rückkopplungsgerät: Über lo erreicht
		// niemand die Fotobox, und eine abgeschaltete Schnittstelle hat keine
		// gültige Adresse.
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		for _, addr := range iface.Addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Eine Adresse aus 169.254.0.0/16 vergibt sich der Rechner selbst,
			// wenn kein DHCP antwortet. Sie sieht brauchbar aus und ist es
			// nicht: Sie bedeutet, dass gar keine Netzverbindung besteht.
			if ip.IsLinkLocalUnicast() {
				continue
			}

			return ip.String()
		}
	}

	return ""
}

// withLANHost ersetzt einen nur örtlich gültigen Rechnernamen durch die
// Adresse im Netz und lässt Schema, Port und Pfad unangetastet.
func withLANHost(rawURL string, lan string) string {
	if lan == "" || !isLocalOnly(rawURL) {
		return rawURL
	}

	for _, host := range localHosts {
		if strings.Contains(rawURL, "//"+host) {
			return strings.Replace(rawURL, "//"+host, "//"+lan, 1)
		}
	}

	return rawURL
}
