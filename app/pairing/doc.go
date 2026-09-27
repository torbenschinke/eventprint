// Package pairing koppelt eine Fotobox mit dem Konto eines Nutzers des
// Upload-Dienstes.
//
// Vorher musste ein Administrator für jede Box ein Zugangstoken anlegen, und
// wer die Box einrichtete, tippte dessen 26 Zeichen auf dem Touchscreen ab.
// Jetzt koppelt jeder registrierte Nutzer seine Boxen selbst, wie man einen
// Fernseher mit einem Streamingkonto verbindet:
//
//  1. An der Box die eigene Mailadresse eintragen. Der Dienst antwortet
//     immer gleich und verrät nicht, ob es das Konto gibt.
//  2. Gibt es einen registrierten, bestätigten und aktiven Nutzer mit dieser
//     Adresse, bekommt er einen sechsstelligen Code, 30 Minuten gültig.
//  3. Den Code an der Box eintippen. Stimmt er, stellt der Dienst selbst ein
//     Zugangstoken mit der Rolle Fotobox-Relay aus und gibt es genau einmal
//     an die Box zurück. Der Nutzer erfährt per Mail, dass eine Box gekoppelt
//     wurde.
//
// Codes liegen nur im Speicher. Ein Neustart des Dienstes verwirft offene
// Kopplungen; wer gerade koppelt, fordert einen neuen Code an. Dafür gibt es
// nichts, was ein Angreifer von der Platte lesen könnte.
package pairing
