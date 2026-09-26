// Package usb bindet einen USB-Stick an die Fotobox an.
//
// Hauptzweck ist der Weg der Fotos nach draußen: Nach der Feier liegen die im
// Kioskbetrieb entstandenen Bilder nur auf der Speicherkarte der Fotobox, und
// die hat weder Bildschirmfreigabe noch Cloud. Ein Stick ist das Einzige, was
// jeder Gastgeber dabeihat. Nebenbei lassen sich Bilder vom Stick durchsehen
// und drucken.
//
// Die Fotobox läuft als systemd-Dienst ohne Anmeldesitzung und ohne
// Desktop-Automounter (gvfs ist maskiert). Niemand hängt den Stick also von
// selbst ein; das geschieht hier ausdrücklich über udisksctl, und zwar erst
// dann, wenn er gebraucht wird. Dafür braucht der Dienstnutzer die
// polkit-Regel deploy/polkit/51-eventprint-udisks.rules.
package usb
