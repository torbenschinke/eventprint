package device

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

const (
	idLoadSettings permission.ID = "de.torbenschinke.eventprint.device.load_settings"
	idSaveSettings permission.ID = "de.torbenschinke.eventprint.device.save_settings"
	idSetPin       permission.ID = "de.torbenschinke.eventprint.device.set_pin"
	idStartKiosk   permission.ID = "de.torbenschinke.eventprint.device.start_kiosk"
	idStopKiosk    permission.ID = "de.torbenschinke.eventprint.device.stop_kiosk"
	idCurrentKiosk permission.ID = "de.torbenschinke.eventprint.device.current_kiosk"
	idUnlock       permission.ID = "de.torbenschinke.eventprint.device.unlock"
	idPreflight    permission.ID = "de.torbenschinke.eventprint.device.preflight"
	idRefillPaper  permission.ID = "de.torbenschinke.eventprint.device.refill_paper"
	idConsumePaper permission.ID = "de.torbenschinke.eventprint.device.consume_paper"
	idIntake       permission.ID = "de.torbenschinke.eventprint.device.intake"
)

var (
	PermLoadSettings = permission.Declare[LoadSettings](idLoadSettings,
		permtext.Name(idLoadSettings, "Einstellungen lesen", "Read settings"),
		permtext.Description(idLoadSettings,
			"Träger dieser Berechtigung sehen die Einstellungen des Geräts.",
			"Holders of this authorisation can see the device settings."),
	)

	PermSaveSettings = permission.Declare[SaveSettings](idSaveSettings,
		permtext.Name(idSaveSettings, "Einstellungen ändern", "Change settings"),
		permtext.Description(idSaveSettings,
			"Träger dieser Berechtigung ändern die Einstellungen des Geräts.",
			"Holders of this authorisation can change the device settings."),
	)

	PermSetPin = permission.Declare[SetPin](idSetPin,
		permtext.Name(idSetPin, "Betreuer-PIN festlegen", "Set the operator PIN"),
		permtext.Description(idSetPin,
			"Träger dieser Berechtigung legen die PIN fest, mit der sich der Kiosk betreuen lässt.",
			"Holders of this authorisation set the PIN that unlocks kiosk administration."),
	)

	PermStartKiosk = permission.Declare[StartKiosk](idStartKiosk,
		permtext.Name(idStartKiosk, "Kiosk starten", "Start the kiosk"),
		permtext.Description(idStartKiosk,
			"Träger dieser Berechtigung versetzen das Gerät bis zum nächsten Neustart in den Kiosk-Modus.",
			"Holders of this authorisation put the device into kiosk mode until the next restart."),
	)

	PermStopKiosk = permission.Declare[StopKiosk](idStopKiosk,
		permtext.Name(idStopKiosk, "Kiosk beenden", "Stop the kiosk"),
		permtext.Description(idStopKiosk,
			"Träger dieser Berechtigung beenden den Kiosk ohne Neustart.",
			"Holders of this authorisation end the kiosk without a restart."),
	)

	PermCurrentKiosk = permission.Declare[CurrentKiosk](idCurrentKiosk,
		permtext.Name(idCurrentKiosk, "Betriebsart abfragen", "Query the operating mode"),
		permtext.Description(idCurrentKiosk,
			"Träger dieser Berechtigung erfahren, ob und welche Feier gerade läuft.",
			"Holders of this authorisation learn whether and which event is running."),
	)

	PermUnlock = permission.Declare[Unlock](idUnlock,
		permtext.Name(idUnlock, "Betreuung freischalten", "Unlock administration"),
		permtext.Description(idUnlock,
			"Träger dieser Berechtigung dürfen die PIN eingeben, um den Kiosk zu betreuen.",
			"Holders of this authorisation may enter the PIN to administer the kiosk."),
	)

	PermPreflight = permission.Declare[Preflight](idPreflight,
		permtext.Name(idPreflight, "Bereitschaft prüfen", "Check readiness"),
		permtext.Description(idPreflight,
			"Träger dieser Berechtigung prüfen vor einer Feier Drucker, Papier, Upload und Kamera.",
			"Holders of this authorisation check printer, paper, upload and camera before an event."),
	)

	PermRefillPaper = permission.Declare[RefillPaper](idRefillPaper,
		permtext.Name(idRefillPaper, "Papier nachgelegt", "Paper refilled"),
		permtext.Description(idRefillPaper,
			"Träger dieser Berechtigung melden ein neu eingelegtes Papierset.",
			"Holders of this authorisation report a freshly loaded paper set."),
	)

	PermConsumePaper = permission.Declare[ConsumePaper](idConsumePaper,
		permtext.Name(idConsumePaper, "Papierverbrauch zählen", "Count paper usage"),
		permtext.Description(idConsumePaper,
			"Träger dieser Berechtigung ziehen gedruckte Blätter vom Vorrat ab.",
			"Holders of this authorisation deduct printed sheets from the stock."),
	)
)

var PermIntake = permission.Declare[Intake](idIntake,
	permtext.Name(idIntake, "Bilder annehmen", "Accept incoming photos"),
	permtext.Description(idIntake,
		"Träger dieser Berechtigung nehmen Bilder von Handy und Kamera an und ordnen sie nach Betriebsart zu.",
		"Holders of this authorisation accept photos from phones and the camera and file them according to the mode."),
)

// Permissions liefert alle Berechtigungen dieses Kontexts.
func Permissions() []permission.ID {
	return []permission.ID{
		PermLoadSettings, PermSaveSettings, PermSetPin, PermStartKiosk, PermStopKiosk,
		PermCurrentKiosk, PermUnlock, PermPreflight, PermRefillPaper, PermConsumePaper, PermIntake,
	}
}
