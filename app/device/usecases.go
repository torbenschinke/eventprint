package device

import (
	"sync"
	"time"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
)

// UseCases bündelt die Anwendungsfälle des Geräts.
type UseCases struct {
	LoadSettings LoadSettings
	SaveSettings SaveSettings
	SetPin       SetPin
	StartKiosk   StartKiosk
	StopKiosk    StopKiosk
	CurrentKiosk CurrentKiosk
	Unlock       Unlock
	Preflight    Preflight
	RefillPaper  RefillPaper
	ConsumePaper ConsumePaper
	Intake       Intake
}

// NewUseCases verdrahtet die Anwendungsfälle.
func NewUseCases(settings SettingsStore, kiosk KioskStore, lock *Lock, probes []Probe, importPhoto photo.Import, print printing.PrintSimple) UseCases {
	var mutex sync.Mutex

	return UseCases{
		LoadSettings: NewLoadSettings(settings),
		SaveSettings: NewSaveSettings(&mutex, settings),
		SetPin:       NewSetPin(&mutex, settings),
		StartKiosk:   NewStartKiosk(&mutex, settings, kiosk, time.Now),
		StopKiosk:    NewStopKiosk(kiosk, lock),
		CurrentKiosk: NewCurrentKiosk(kiosk),
		Unlock:       NewUnlock(settings, lock),
		Preflight:    NewPreflight(settings, probes),
		RefillPaper:  NewRefillPaper(&mutex, settings),
		ConsumePaper: NewConsumePaper(&mutex, settings),
		Intake:       NewIntake(settings, kiosk, importPhoto, print),
	}
}
