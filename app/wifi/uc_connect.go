package wifi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.wdy.de/nago/application/permission"
)

// Connect verbindet die Fotobox mit einem Funknetz.
//
// Ein leeres Kennwort ist gültig und bedeutet ein offenes Netz.
type Connect func(subject permission.Auditable, ctx context.Context, ssid, password string) error

// WrongPasswordError meldet ein abgelehntes Kennwort.
//
// Ein Typ und keine Paketvariable: Der Anwendungsfall soll keinen Paketzustand
// lesen, sonst haengt sein Verhalten an einer Initialisierung, die an der
// Aufrufstelle unsichtbar ist. Der Unterschied zum sonstigen Scheitern ist für
// die Bedienung wesentlich – neu eintippen oder näher ans Funknetz.
type WrongPasswordError struct {
	SSID string
}

func (e WrongPasswordError) Error() string {
	return "das Kennwort für " + e.SSID + " wurde nicht angenommen"
}

// NewConnect bindet den Verbindungsaufbau an NetworkManager.
//
// nmcli und nicht wpa_supplicant von Hand: Raspberry Pi OS verwaltet das Netz
// seit Bookworm über NetworkManager. Wer daran vorbei konfiguriert, bekommt
// zwei Stellen, die sich widersprechen, und beim nächsten Start gewinnt die
// falsche.
func NewConnect() Connect {
	return func(subject permission.Auditable, ctx context.Context, ssid, password string) error {
		if err := subject.Audit(PermConnect); err != nil {
			return err
		}

		if strings.TrimSpace(ssid) == "" {
			return errors.New("ohne Netzname kann keine Verbindung aufgebaut werden")
		}

		args := []string{"device", "wifi", "connect", ssid}
		if password != "" {
			args = append(args, "password", password)
		}

		out, err := runNmcli(ctx, connectTimeout, args...)
		if err == nil {
			return nil
		}

		// NetworkManager meldet ein falsches Kennwort nicht als eigenen Code,
		// sondern im Text. Der Unterschied ist für die Bedienung wesentlich:
		// Kennwort neu eingeben oder näher ans Funknetz.
		if IsWrongPassword(out) {
			return WrongPasswordError{SSID: ssid}
		}

		return fmt.Errorf("Verbindung mit %q fehlgeschlagen: %w", ssid, err)
	}
}

// IsWrongPassword erkennt ein abgelehntes Kennwort in der Ausgabe von nmcli.
func IsWrongPassword(out string) bool {
	low := strings.ToLower(out)

	for _, marker := range []string{
		"secrets were required",
		"invalid password",
		"802.1x supplicant",
	} {
		if strings.Contains(low, marker) {
			return true
		}
	}

	return false
}
