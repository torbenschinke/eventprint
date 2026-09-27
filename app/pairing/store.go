package pairing

import (
	"sync"
	"time"
)

// pending ist eine offene Kopplung.
type pending struct {
	account  Account
	mail     string
	known    bool // gibt es das Konto; sonst kann kein Code passen
	device   string
	code     string
	expires  time.Time
	attempts int
}

// store hält die offenen Kopplungen und die Versandzeiten je Adresse.
type store struct {
	mu      sync.Mutex
	pending map[ID]*pending
	sent    map[string][]time.Time

	// last ist die jüngste Kopplung je Adresse, für jede Adresse, ob es das
	// Konto gibt oder nicht; siehe [store.recent].
	last map[string]recentPairing
}

type recentPairing struct {
	id ID
	at time.Time
}

func newStore() *store {
	return &store{pending: map[ID]*pending{}, sent: map[string][]time.Time{}, last: map[string]recentPairing{}}
}

// recent liefert die Kopplung, die für diese Adresse vor weniger als
// [MailCooldown] begonnen wurde und noch offen ist.
//
// Ein zweiter Tipp auf "Code senden" bekommt so dieselbe Kennung und damit
// den Code aus der ersten Mail. Dass es für jede Adresse gilt und nicht nur
// für bekannte, ist Absicht: Sonst verriete "dieselbe Kennung wie eben", dass
// es das Konto gibt.
func (s *store) recent(mail string, now time.Time) (ID, bool) {
	r, ok := s.last[mail]
	if !ok || now.Sub(r.at) >= MailCooldown {
		return "", false
	}

	p, ok := s.pending[r.id]
	if !ok || p.attempts >= MaxAttempts {
		return "", false
	}

	return r.id, true
}

// sweep wirft Abgelaufenes weg. Es läuft bei jeder Anfrage mit; einen
// eigenen Hintergrundjob braucht es bei dieser Menge nicht.
func (s *store) sweep(now time.Time) {
	for id, p := range s.pending {
		if now.After(p.expires) {
			delete(s.pending, id)
		}
	}

	for mail, r := range s.last {
		if now.Sub(r.at) >= MailCooldown {
			delete(s.last, mail)
		}
	}

	for mail, times := range s.sent {
		kept := times[:0]
		for _, t := range times {
			if now.Sub(t) < time.Hour {
				kept = append(kept, t)
			}
		}

		if len(kept) == 0 {
			delete(s.sent, mail)
		} else {
			s.sent[mail] = kept
		}
	}
}

// mayMail meldet, ob an die Adresse jetzt ein Code gehen darf, und merkt
// sich den Versand.
func (s *store) mayMail(mail string, now time.Time) bool {
	times := s.sent[mail]
	if len(times) >= MailsPerHour {
		return false
	}

	if n := len(times); n > 0 && now.Sub(times[n-1]) < MailCooldown {
		return false
	}

	s.sent[mail] = append(times, now)

	return true
}
