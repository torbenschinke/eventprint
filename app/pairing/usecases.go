package pairing

import "time"

// UseCases bündelt die Kopplung.
type UseCases struct {
	RequestPairing RequestPairing
	ConfirmPairing ConfirmPairing
}

// Options sind die Anschlüsse an den Dienst.
type Options struct {
	Accounts Accounts
	Mailer   Mailer
	Issuer   Issuer

	// Now ist die Uhr; leer heißt time.Now.
	Now Clock
}

// NewUseCases verdrahtet die Kopplung mit einem gemeinsamen Speicher.
func NewUseCases(opts Options) UseCases {
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	s := newStore()

	return UseCases{
		RequestPairing: NewRequestPairing(s, opts.Accounts, opts.Mailer, now),
		ConfirmPairing: NewConfirmPairing(s, opts.Issuer, opts.Mailer, now),
	}
}
