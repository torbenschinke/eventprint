package wifi

// UseCases bündelt die Anwendungsfälle der Funkverbindung.
type UseCases struct {
	Scan    Scan
	Current Current
	Connect Connect
}

// NewUseCases bindet alle Anwendungsfälle an NetworkManager.
func NewUseCases() UseCases {
	return UseCases{
		Scan:    NewScan(),
		Current: NewCurrent(),
		Connect: NewConnect(),
	}
}
