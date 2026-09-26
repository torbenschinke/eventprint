package uidevice

import "go.wdy.de/nago/application/permission"

// sys ist das Subjekt für Anzeigen, die für jeden bestimmt sind, etwa die
// Statusleiste. Es wird bewusst nur dort verwendet: Alles, was jemand am
// Bildschirm auslöst, läuft mit dessen Rolle.
func sys() permission.Auditable { return permission.SU() }
