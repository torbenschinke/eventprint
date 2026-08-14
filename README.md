# eventprint – Fotobox für Veranstaltungen

Eine Nago-Anwendung für Hochzeiten, Jubiläen und ähnliche Feiern. Sie läuft auf
dem Rechner, an dem Fotodrucker und Kamera per USB hängen, und wird von den
Gästen selbst bedient.

Die Einrichtung des Druckers ist getrennt dokumentiert: **[DRUCKER.md](DRUCKER.md)**.

---

## Was die Anwendung kann

* **Fotobox-Startbildschirm** mit den zuletzt entstandenen Bildern. Ein Tippen
  auf ein Bild druckt es erneut.
* **QR-Code** auf dem Startbildschirm. Gäste scannen ihn und laden vom
  Smartphone eigene Bilder hoch – ohne Anmeldung, ohne App.
* **Drei Layouts**, vor jedem Druck in einem Dialog wählbar:

  | Layout | Verhalten |
  |---|---|
  | Formatfüllend | randlos, das Motiv wird mittig auf 3:2 beschnitten |
  | Mit Rand | vollständiges Motiv, gleichmäßiger weißer Rand |
  | Polaroid | Sofortbild-Look, breiter Steg unten |

* **Historie** aller Fotos – Kameraaufnahmen und Gast-Uploads gleichermaßen –
  mit Nachdruck und Löschen.
* **Druckstatus** mit Warteschlange, Fehlerursache und Wiederholung, etwa nach
  einem Papierwechsel.

## Schnellstart

```bash
HOST=0.0.0.0 NAGO_COOKIES_INSECURE=true go run ./cmd/photobox
```

Danach unter <http://localhost:3000> als Betreuer anmelden
(`admin@localhost`, Kennwort siehe `cmd/photobox/main.go`) und einrichten.

## Einrichten

Beides geschieht in der Oberfläche, wirkt sofort und überlebt einen Neustart.
Der Weg dorthin führt über das Nutzermenü → Admin-Center → Einstellungen oder
direkt über die Schaltflächen auf den betroffenen Seiten.

### Drucker

**Einstellungen → Fotodrucker.** Dort werden auch Oberflächenfinish
(glänzend, matt, seidenmatt) und die Rastergröße gepflegt – zu letzterer siehe
[Druckqualität](#druckqualität). Die Warteschlange wird aus `lpstat -a`
angeboten, es lässt sich also nichts vertippen. Solange nichts gewählt ist,
läuft die Fotobox im Testmodus: Aufträge durchlaufen dieselbe Zustandsmaschine
inklusive Rendering, das Ergebnis wird aber verworfen. So lässt sich die
Fotobox ohne Hardware aufbauen und vorführen – die Druckstatus-Seite weist
deutlich darauf hin.

### Öffentliche Adresse für den QR-Code

**Einstellungen → Fotobox → Öffentliche Adresse.** Ohne Angabe leitet Nago die
Adresse aus der ersten Verbindung ab. Hinter einem Reverse Proxy stimmt die
nicht, und wer die Fotobox zuerst lokal öffnet, bekommt `localhost` in den
QR-Code – kein Gast kommt dann auf die Upload-Seite. Deshalb hier die von
außen erreichbare Adresse vollständig eintragen, z. B.
`https://fotobox.example.de`. Steht eine lokale Adresse im QR-Code, warnt der
Startbildschirm den angemeldeten Betreuer.

### Umgebungsvariablen

Sie sind nur noch Vorbelegung für den unbeaufsichtigten Betrieb (systemd,
Container) und greifen ausschließlich, solange in den Einstellungen nichts
steht. Danach hat die Oberfläche Vorrang.

| Variable | Bedeutung |
|---|---|
| `EVENTPRINT_TITLE` | Überschrift auf dem Startbildschirm |
| `EVENTPRINT_PRINTER` | CUPS-Warteschlange, z. B. `CZ01` |
| `EVENTPRINT_CAMERA_DIR` | Tethering-Verzeichnis der Kamera. Leer = deaktiviert |
| `EVENTPRINT_CAMERA_AUTOPRINT` | `true` druckt jede Aufnahme sofort |
| `EVENTPRINT_CAMERA_DELETE` | `true` löscht die Datei nach der Übernahme |
| `HOST` | Bind-Adresse. Für Gäste im WLAN `0.0.0.0` |
| `NAGO_COOKIES_INSECURE` | `true`, solange ohne HTTPS betrieben |

## Rollen

| Rolle | Rechte |
|---|---|
| `Fotobox-Gast` | Hochladen, Ansehen, Drucken – automatisch für jeden nicht angemeldeten Besucher |
| `Fotobox-Betreuer` | zusätzlich Löschen, Druckaufträge wiederholen, Einrichten |

Die Betreuer-Rolle wird beim Start automatisch dem Bootstrap-Administrator
zugewiesen. Das ist notwendig, weil Nago diesem Konto absichtlich nur
`nago.*`-Berechtigungen gibt: Ohne die Zuweisung zeigte die Anwendung nach dem
Anmelden auf jeder Seite „Zugriff verweigert". Weitere Betreuer bekommen die
Rolle über die Nutzerverwaltung.

## Kamera anschließen

Die Kamera wird per PTP/MTP angesprochen. Statt das Protokoll selbst zu
implementieren, nutzt die Anwendung ein Übergabeverzeichnis – das entkoppelt
sie vollständig vom Kameramodell:

```bash
mkdir -p /var/lib/photobox/incoming
gphoto2 --capture-tethered --filename '/var/lib/photobox/incoming/%Y%m%d-%H%M%S.jpg'
```

Mit `EVENTPRINT_CAMERA_DIR=/var/lib/photobox/incoming` übernimmt die Fotobox
jede Aufnahme automatisch. Eine Datei wird erst eingelesen, wenn ihre Größe
zwischen zwei Durchläufen konstant bleibt – sonst würde ein noch übertragenes,
halbes JPEG importiert.

## Aufbau

Die Anwendung folgt dem Nago-Use-Case-Muster: Fachlichkeit in eigenständigen
Paketen, ein Anwendungsfall je Datei, Verdrahtung an genau einer Stelle.

```
photo/       Domäne der Fotos (Import, Historie, Originaldaten)
printing/    Layouts, Rendering, Druckaufträge, CUPS-Anbindung
camera/      Übernahme der Kamerabilder aus dem Tethering-Verzeichnis
ui/          Oberfläche (Fotobox, Upload, Galerie, Druckstatus)
cfg/         Enable() – verdrahtet alles mit dem Nago-Configurator
cmd/photobox Startpunkt inkl. Scaffold-Menü
```

Einige Entwurfsentscheidungen, die beim Lesen sonst überraschen:

* **IDs sind zeitlich sortierbar** (`<unix-millis>-<zufall>`). Dadurch liefert
  die lexikographische Iteration des Repositories die chronologische
  Reihenfolge – ohne zusätzlichen Index.
* **Gedruckt wird aus dem Original**, nicht aus einer der verkleinerten
  Varianten des Nago-Image-Subsystems. Nur so werden die 300 dpi des
  Dye-Sublimation-Druckers ausgenutzt.
* **Der Zuschnitt passiert in der Anwendung**, nicht im Druckertreiber. CUPS
  würde ein abweichendes Seitenverhältnis einpassen und weiße Balken erzeugen;
  siehe [DRUCKER.md](DRUCKER.md).
* **Das gerenderte JPEG bekommt ein JFIF-Segment.** Gos `image/jpeg` schreibt
  hinter den Startmarker direkt die Quantisierungstabelle; die Datei beginnt
  also mit `FF D8 FF DB`. CUPS erkennt `image/jpeg` aber nur, wenn das vierte
  Byte ein Anwendungsmarker aus `0xE0`–`0xEF` ist
  (`/usr/share/cups/mime/mime.types`). Ohne das Segment scheitert die
  Typerkennung, CUPS meldet „The print file could not be opened" und der
  Auftrag verschwindet.
* **Die EXIF-Ausrichtung wird beim Import aufgelöst**, nicht erst beim Druck.
  Gos `image/jpeg` ignoriert den EXIF-Block, Nagos Bild-Subsystem ebenfalls –
  ein hochkant gehaltenes Smartphone speichert aber quer und vermerkt die Lage
  nur als Zahl. Ohne Korrektur läge ein Hochformat auf der langen Papierkante,
  würde dort formatfüllend beschnitten und käme gedreht sowie stark vergrößert
  aus dem Drucker. Durch die Normalisierung beim Import zeigen Galerie,
  Vorschau und Ausdruck dieselbe Lage. `printing.Render` prüft zusätzlich, weil
  vorher gespeicherte Fotos sonst weiterhin falsch gedruckt würden.
* **Der Druck läuft asynchron** über einen einzelnen Worker. Der Klick kehrt
  sofort zurück, der Fortschritt ist auf der Druckstatus-Seite sichtbar. Nach
  einem Neustart werden wartende Aufträge erneut eingereiht, unterbrochene als
  fehlgeschlagen markiert – ob das Papier verbraucht wurde, ist nicht bekannt.
* **Gäste arbeiten anonym.** Beim Start legt die Anwendung die Rolle
  `Fotobox-Gast` an und weist sie allen nicht angemeldeten Besuchern zu. Sie
  enthält gezielt nur Hochladen, Ansehen und Drucken – das Löschen bleibt dem
  angemeldeten Betreuer vorbehalten.
* **Der Drucker wird pro Auftrag aus den Einstellungen gelesen**, nicht beim
  Start zwischengespeichert. Wer die Fotobox mitten auf der Feier
  umkonfiguriert, sieht das Ergebnis sofort. Am Auftrag selbst bleibt das
  damals verwendete Ziel gespeichert – die Historie bleibt dadurch wahr.
* **Jeder Einstellungstyp braucht `enum.Rename`.** Nago serialisiert die
  globalen Einstellungen als offenen Summentyp und nutzt dabei standardmäßig
  den bloßen Go-Typnamen als Diskriminator. Da mehrere Pakete ihren Typ
  `Settings` nennen, überschreiben sie sich sonst gegenseitig; beim Lesen
  entsteht dann ein `interface conversion`-Panic. `cfg/settings_enum_test.go`
  prüft das für alle registrierten Varianten.

## Druckqualität

### CZ-01-Treiber installieren

Gutenprint vergrößert die 1224 Pixel breite 4x6-Bildfläche standardmäßig per
Point-Sampling auf 1266 Pixel. Das erzeugt sichtbare Treppen an diagonalen
Kanten. Der isolierte Custom-Build korrigiert nur diese Geometrie und ersetzt
keine Paketdateien:

```bash
./scripts/install-gutenprint-cz01.sh
```

Die Anwendung bevorzugt danach automatisch
`/opt/eventprint/gutenprint/bin/rastertocz01`. Fehlt der Filter, protokolliert
sie eine Warnung und verwendet weiterhin den normalen Gutenprint-Treiber der
CUPS-Warteschlange.

Die am Gerät gemessene Tonwertkurve wird bereits beim Rendern angewendet und
gilt für beide Wege. Beim Custom-Weg wird der fertige Druckstrom raw an CUPS
übergeben, sodass keine zweite Farb- oder Geometrieverarbeitung stattfindet.

**Auflösung.** Der CZ-01 beherrscht ausschließlich 300x300 dpi; eine höhere
Stufe gibt es nicht. Das ist keine Einschränkung des Treibers: Gutenprint
bietet für die verwandten Modelle CW-02, CX-02 und DNP DS620 sehr wohl
zusätzlich 300x600 dpi an, für den CZ-01 dagegen nur einen einzigen Eintrag.

```bash
/usr/lib/cups/driver/gutenprint.5.3 cat gutenprint.5.3://citizen-cz-01/expert | grep "^\*Resolution "
```

**Rastergröße – hier lag Qualität brach.** Der Drucker erwartet für 10x15 cm
nicht die rechnerischen 1200x1800 Pixel (4x6 Zoll mal 300 dpi), sondern
**1224x1836**: Randloses Drucken braucht einen Überstand von gut zwei Prozent.
Wurde in 1200x1800 gerendert, skalierte die CUPS-Filterkette das Bild hoch und
weichte es dabei auf. Da beide Größen dasselbe Seitenverhältnis von 2:3 haben,
fällt das ohne direkten Vergleich nicht auf – der Ausdruck ist nur unnötig
weich. Die Anwendung rendert deshalb direkt in der nativen Rastergröße, es
findet keine Skalierung mehr statt.

Für ein anderes Modell oder Papierformat lässt sich der richtige Wert ablesen:

```bash
sudo cupsctl --debug-logging
lp -d CZ01 -o PageSize=w288h432 bild.jpg
grep -a "cupsWidth\|cupsHeight" /var/log/cups/error_log
# cupsWidth = 1224
# cupsHeight = 1836
sudo cupsctl --no-debug-logging
```

Danach in **Einstellungen → Fotodrucker → Rasterbreite/Rasterhöhe** eintragen.

**Was sonst noch geprüft wurde:**

* `StpImageType=Photo` – gesetzt, aktiviert die Farbaufbereitung für Fotos
  statt der Vorgabe `TextGraphics`.
* `StpColorPrecision=Best` – **wirkungslos, Ursache geklärt.** Das PPD setzt
  für *beide* Stufen `cupsBitsPerColor 8`; bei `Best` kommt lediglich der
  Hinweis `cupsPreferredBitsPerColor 16` hinzu. Die Filterkette lautet hier
  `imagetoraster → rastertogutenprint` – ohne PDF-Umweg –, und
  `imagetoraster` wertet nur `cupsBitsPerColor` aus. Im Protokoll blieb es
  entsprechend bei `cupsBitsPerColor = 8`. Da die Vorlagen ohnehin 8-Bit-JPEGs
  von Kameras und Smartphones sind, wäre auch nichts zu gewinnen.
* `StpPrintSpeed` – **standardmäßig `LowSpeed`**. Bei normaler
  Geschwindigkeit verweilt der Thermokopf kürzer auf jeder Zeile und überträgt
  weniger Farbe; die Ausdrucke wirken dann verwaschen. Für eine Fotobox zählt
  das Ergebnis mehr als der Durchsatz. Umstellbar unter
  **Einstellungen → Fotodrucker → Druckgeschwindigkeit**, falls der Durchsatz
  wichtiger ist.
* Das gerenderte JPEG wird mit Qualität 95 und nur ein einziges Mal
  komprimiert; gedruckt wird stets aus dem Original, nie aus einem
  Vorschaubild.

## Fehlersuche beim Drucken

Die Druckstatus-Seite beantwortet die Frage „warum kommt nichts?" ohne
Terminal:

* **Zustand des Druckers** – nicht eingerichtete Warteschlange, angehaltener
  Drucker, gestoppte Annahme sowie die Meldung des Geräts, etwa „Out of paper".
* **Je Auftrag** die Kennung der Druckerwarteschlange (z. B. `CZ01-12`), den
  IPP-Grund bei Fehlschlägen (z. B. `canceled-at-device`) und die
  Klartextursache von CUPS.

Der entscheidende Punkt: Ein Auftrag gilt **nicht** als fertig, sobald `lp` ihn
angenommen hat. Die Anwendung verfolgt ihn danach über
`lpstat -l -W completed` weiter und meldet erst dann Erfolg, wenn CUPS
`job-completed-successfully` bestätigt. Genau daran scheiterte die erste
Fassung: `lp` quittierte den Auftrag, CUPS verwarf ihn anschließend still, und
die Oberfläche behauptete „Fertig", während nichts gedruckt wurde.

Zum Nachschauen im Terminal, mit der Kennung von der Statusseite:

```bash
lpstat -l -W completed -o CZ01     # Ausgang der letzten Aufträge
lpstat -p CZ01                     # Zustand des Druckers
sudo cupsenable CZ01               # angehaltenen Drucker freigeben
```

## Tests

### Fachlichkeit (Go)

```bash
go test ./...
```

Die Tests des Renderers prüfen die Kernanforderung direkt am Pixel: exakte
Papiergeometrie (1200x1800 @ 300 dpi), keine weißen Ecken beim formatfüllenden
Layout, weißer Rand beim Layout „Mit Rand", breiterer Steg unten beim Polaroid.
Dazu kommen Tests für die Druckerauswahl, das Auswerten von `lpstat -a`, den
Aufbau der öffentlichen Adresse und die eindeutigen Einstellungs-Diskriminatoren.

Zum Ansehen der Ergebnisse:

```bash
EVENTPRINT_TEST_OUTPUT=/tmp/tpl go test ./printing/
```

### Oberfläche (Playwright)

Die Nago-Oberfläche ist eine Single-Page-Anwendung, die ihren gesamten Zustand
über eine WebSocket-Verbindung bezieht. Ein HTTP-Abruf sagt daher nichts über
die Funktion aus – die Tests steuern einen echten Browser.

```bash
python3 -m venv .venv
.venv/bin/pip install playwright pytest
.venv/bin/playwright install chromium

.venv/bin/python -m pytest
```

Die Tests starten die Anwendung selbst in einem leeren Datenverzeichnis auf
einem freien Port – je Test eine eigene Instanz, weil Fotos und Druckaufträge
dauerhaft gespeichert werden und die Tests sonst voneinander abhingen. Geprüft
wird der echte Ablauf: Upload über die QR-Code-Seite, Layout-Auswahl im Dialog,
Druck, Druckstatus, Historie und Startbildschirm, dazu das Einrichten als
Betreuer.

> **Stolperfalle:** Nago mountet die SPA nicht, wenn der User-Agent als Crawler
> erkannt wird – der Bundle prüft ihn gegen eine Bot-Liste und ruft
> `mount("#app")` gar nicht erst auf. Playwrights Standard-User-Agent enthält
> `HeadlessChrome` und fällt in genau dieses Raster; die Seite bleibt dann
> dauerhaft leer, ohne jede Fehlermeldung. `e2e/conftest.py` setzt deshalb
> einen realistischen User-Agent.
>
> Ebenfalls beachten: Die Menüeinträge des Scaffolds sind keine `<a>`-Elemente,
> sondern klickbare Container. `get_by_role("link", …)` findet sie nicht.
