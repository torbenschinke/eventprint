package relay

// UseCases bündelt die Anwendungsfälle des Upload-Dienstes auf dem Gerät.
type UseCases struct {
	UploadAddress UploadAddress
}

// NewUseCases verdrahtet die Anwendungsfälle mit dem Poller.
func NewUseCases(p *Poller) UseCases {
	return UseCases{UploadAddress: NewUploadAddress(p)}
}
