# Uploads aus dem Internet

<!-- speclink:informative -->

Gäste und Besitzer sollen Bilder vom eigenen Smartphone beisteuern können, auch ohne im
Netz der Fotobox zu sein. Dafür steht ein kleiner Dienst im Internet, den die
Fotobox regelmäßig abfragt. Die Fotobox selbst bleibt von außen unerreichbar.

## Upload-Sitzung

Eine Fotobox MUSS sich am Dienst anmelden und eine kurzlebige Upload-Adresse
erhalten. Je Fotobox darf höchstens eine Adresse gültig sein, denn der QR-Code
auf dem Startbildschirm zeigt genau eine.

## Wartende Aufträge abholen

Die Fotobox MUSS die für sie hinterlegten Aufträge abrufen können. Eine
Fotobox darf dabei ausschließlich ihre eigenen Aufträge sehen.

## Bild eines Auftrags laden

Zu einem wartenden Auftrag MUSS das Originalbild abrufbar sein.

## Übernahme bestätigen

Ein Auftrag MUSS erst dann beim Dienst verschwinden, wenn die Fotobox seine
Übernahme bestätigt hat. Ohne Bestätigung geht ein Bild verloren, sobald die
Übertragung scheitert.

## Upload in den Eingang

Im Heimbetrieb MUSS die Upload-Seite mehrere Bilder auf einmal annehmen und
dabei keine Gestaltung abfragen: Die Bilder landen im Eingang, gestaltet wird
am Gerät. Welche Art von Upload gemeint ist, MUSS die Adresse bestimmen, die
das Gerät als QR-Code zeigt.

## Fotobox mit dem Konto koppeln

Jeder registrierte Nutzer des Upload-Dienstes MUSS seine Fotoboxen selbst
koppeln können, ohne dass ein Administrator Zugangsdaten verteilt. An der Box
wird dafür nur die eigene Mailadresse eingegeben. Gibt es dazu einen
registrierten, bestätigten und aktiven Nutzer, bekommt er einen
sechsstelligen Code, der 30 Minuten gilt; die Box DARF dabei nicht erfahren,
ob es das Konto gibt. Wird der Code an der Box eingetippt, MÜSSEN Dienst und
Box das Zugangstoken selbst austauschen. Falsche Codes MÜSSEN nach wenigen
Versuchen zur Sperre führen, und der Nutzer SOLL per Mail erfahren, dass eine
Box gekoppelt wurde.
