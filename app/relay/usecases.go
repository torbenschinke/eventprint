package relay

// UseCases bündelt die Anwendungsfälle des Upload-Dienstes auf dem Gerät.
type UseCases struct {
	UploadAddress   UploadAddress
	BeginPairing    BeginPairing
	CompletePairing CompletePairing
}

// NewUseCases verdrahtet die Anwendungsfälle mit dem Poller und dem
// Speicher der Zugangsdaten.
func NewUseCases(p *Poller, save Credentials) UseCases {
	return UseCases{
		UploadAddress:   NewUploadAddress(p),
		BeginPairing:    NewBeginPairing(),
		CompletePairing: NewCompletePairing(save),
	}
}
