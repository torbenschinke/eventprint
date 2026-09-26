// Package device ist das Gerät selbst: Betriebsart, Einstellungen, Zugang.
//
// Die Box kennt keine Benutzerkonten. Wer vor dem Bildschirm steht, ergibt
// sich aus der Betriebsart: Im Heimbetrieb ist es der Besitzer, im Kiosk ein
// Gast, und nach Eingabe der PIN im Kiosk die Betreuung. Diese Rolle ist das
// Subjekt, gegen das jeder Anwendungsfall seine Berechtigung prüft. Die
// Schranke zwischen Feier und privater Mediathek hängt damit an denselben
// Prüfungen wie alles andere und nicht daran, welche Knöpfe eine Oberfläche
// gerade zeigt.
package device
