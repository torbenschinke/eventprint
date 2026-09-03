// Package printing enthält die Domäne rund um das Rendern und Ausdrucken von
// Fotos auf einem 10x15 cm Fotodrucker (z. B. Citizen CZ-01).
//
// Der Druck läuft asynchron: Die Oberfläche legt über [Print] lediglich einen
// [Job] an, der von einem Worker abgearbeitet wird. Dadurch blockiert die
// Fotobox nicht, während der Drucker seine ~15 Sekunden pro Bild braucht, und
// der Fortschritt ist auf der Druckstatus-Seite sichtbar.
package printing
