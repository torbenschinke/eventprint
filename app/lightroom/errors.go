package lightroom

import (
	"fmt"
	"strings"
)

// NotConnectedError meldet, dass keine gültige Verbindung zu Lightroom
// besteht und der Besitzer sich neu anmelden muss.
//
// Ein Typ und keine Paketvariable: Die Anwendungsfälle sollen keinen
// Paketzustand lesen. Die Oberfläche braucht den Unterschied, denn nur hier
// hilft der QR-Code – bei jedem anderen Fehler hilft Warten oder ein besseres
// Funknetz.
type NotConnectedError struct {
	// Reason erklärt, warum die Verbindung fehlt; leer, wenn nie verbunden.
	Reason string
}

func (e NotConnectedError) Error() string {
	if e.Reason == "" {
		return "Lightroom ist nicht verbunden – bitte über den QR-Code neu verbinden"
	}

	return "Lightroom ist nicht mehr verbunden (" + e.Reason + ") – bitte über den QR-Code neu verbinden"
}

// NotConfiguredError meldet fehlende Einstellungen.
type NotConfiguredError struct {
	Missing []string
}

func (e NotConfiguredError) Error() string {
	return "Lightroom ist nicht eingerichtet, es fehlt: " + strings.Join(e.Missing, ", ")
}

// HTTPError ist eine unerwartete Antwort eines entfernten Dienstes.
//
// Der Statuscode bleibt erhalten, weil die Aufrufer an ihm unterscheiden, ob
// eine Verbindung verloren ist (400/401 beim Token-Tausch) oder eine
// Vorschaustufe nur fehlt (404 bei einer Rendition).
type HTTPError struct {
	// Service benennt die Gegenstelle für die Meldung, z. B. "Adobe-Anmeldung".
	Service string
	Status  int
	// Detail ist ein gekürzter Auszug der Antwort, falls vorhanden.
	Detail string
}

func (e HTTPError) Error() string {
	msg := fmt.Sprintf("%s antwortet mit HTTP %d", e.Service, e.Status)

	switch e.Status {
	case 429:
		msg += " (zu viele Anfragen, bitte kurz warten)"
	case 500, 502, 503, 504:
		msg += " (Störung beim Dienst, bitte später erneut versuchen)"
	}

	if e.Detail != "" {
		msg += ": " + e.Detail
	}

	return msg
}
