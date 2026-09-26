# Entscheidungen

<!-- speclink:informative -->

Festlegungen, die nicht aus einer Anforderung folgen, sondern gewählt wurden,
und deren Preis irgendwann jemand kennen muss.

## Form der Ablage

Fotos und Druckaufträge werden als **aktueller Zustand** abgelegt, nicht als
Folge von Ereignissen. Eine Fotobox läuft an einem Abend; danach interessiert
niemanden, in welcher Reihenfolge ein Auftrag seine Zustände durchlaufen hat,
sondern nur, ob das Bild auf Papier ist.

## Oberfläche ohne Browser

Die Oberfläche am Gerät wird mit gift gezeichnet, direkt auf die GPU, und
nicht mehr als Weboberfläche in einem Browser im Vollbild. Das Gerät braucht
dadurch weder Chromium noch einen lokalen Webserver, startet schneller und
kann nicht versehentlich eine Webseite verlassen. Der Preis: Die Oberfläche
lässt sich nicht mehr aus der Ferne im Browser öffnen, und Oberflächentests
laufen nicht mehr mit Playwright, sondern mit dem Test-Harness von gift.
