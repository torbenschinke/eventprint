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
  | Passepartout | weißer Rahmen von 1 cm, ringsum exakt gleich breit |
  | Polaroid | Sofortbild-Look, breiter Steg unten |

  Bei „Formatfüllend“ und „Passepartout“ wird das Papier in die Richtung
  gedreht, in der das Motiv liegt – ein Querformatfoto ergibt also ein quer
  liegendes Bild. Das Polaroid bleibt immer hochkant, sonst säße der breite
  Steg an der falschen Kante.

  Beim Passepartout hat der Rahmen Vorrang vor dem Motiv: Passt das
  Seitenverhältnis nicht zur verbleibenden Fläche, wird das Bild an den Kanten
  beschnitten. Ein ungleichmäßiger Rand sieht auf dem Papier nach einem Fehler
  aus, ein knapperer Ausschnitt nicht. Bei einem 4:3-Foto gehen dabei rund
  18 % der Höhe verloren, bei 3:2 rund 8 %.

* **Historie** aller Fotos – Kameraaufnahmen und Gast-Uploads gleichermaßen –
  mit Nachdruck und Löschen.
* **Druckstatus** mit Warteschlange, Fehlerursache und Wiederholung, etwa nach
  einem Papierwechsel.
* **Archiv der Originale**: Jedes eingehende Bild wird zusätzlich unverändert
  als Datei abgelegt, für die digitale Weitergabe nach der Feier.

## Schnellstart

```bash
HOST=0.0.0.0 NAGO_COOKIES_INSECURE=true go run ./cmd/photobox
```

Danach unter <http://localhost:3000> als Betreuer anmelden
(`admin@localhost`, Kennwort siehe `cmd/photobox/main.go`) und einrichten.

## Bauen und prüfen

```bash
./scripts/build.sh
```

Das ist die vollständige Schleife und die einzige, deren Reihenfolge feststeht:

```
go build  ->  go test | speclink evidence  ->  speclink verify  ->  speclink generate
```

Der Nachweis läuft **vor** der Prüfung, weil `speclink verify` wissen will,
welche Tests tatsächlich durchgelaufen sind. Umgekehrt prüfte es einen Nachweis
von gestern. Am Ende stehen die Binärdateien in `dist/` und die abgeleitete
Spezifikation in [SPECIFICATION.md](SPECIFICATION.md).

| Variable | Wirkung |
|---|---|
| `FACECROP=0` | ohne Gesichtserkennung bauen, also ohne OpenCV und gocv |
| `SKIP_TESTS=1` | nur bauen; die Spezifikation entsteht dann nicht |
| `TARGETS` | z. B. `"linux/arm64 linux/amd64"` |

`photobox` braucht cgo und OpenCV, `photoupld` nicht. Für eine fremde
Architektur wird `photobox` deshalb übersprungen statt in einer Binärdatei zu
enden, die auf dem Zielgerät nicht startet.

### speclink

Die Anforderungen liegen als Go-Quelltext in `requirements/`, die
Quelldokumente darunter in `requirements/_sources/`. Jeder Anwendungsfall nennt
in einer `*.annotation.go` neben sich die Anforderung, für die er geschrieben
wurde; jeder Test endet mit `spec.Verified(t, …)`. Vier Zahlen müssen alle 100 %
erreichen, sonst schlägt der Bau fehl:

```
15 source segments (100% accounted), 30 constructs (100% bound),
15 normative requirements (100% covered, 100% verified), 0 findings
```

*accounted* fragt, ob jeder Abschnitt der Quelldokumente zu einer Anforderung
geworden ist. *bound* fragt, ob jeder Anwendungsfall eine Anforderung nennt.
*covered* fragt die Gegenrichtung. *verified* ist die einzige, die danach
fragt, ob überhaupt etwas gezeigt hat, dass der Quelltext tut, was die
Anforderung verlangt.

`speclink.lock` gehört ins Repository: Darin steht, welcher Test welche
Anforderung wann gezeigt hat und in welchem Wortlaut. Wird eine Anforderung
umformuliert, verfällt der Nachweis und muss neu erbracht werden.

## Installation auf einem Raspberry Pi

Auf einem frischen Raspberry Pi OS oder Debian, als root:

```bash
git clone https://github.com/torbenschinke/eventprint.git
cd eventprint
sudo ./scripts/install.sh --dry-run     # zeigt nur, was geschähe
sudo ./scripts/install.sh
```

Danach liegen dort:

| Ort | Inhalt |
|---|---|
| `/opt/eventprint` | Arbeitskopie des Repositories |
| `/var/lib/eventprint` | Daten: Fotos, Druckaufträge, Archiv der Originale |
| `/etc/default/eventprint` | Umgebung der Dienste, von Hand änderbar |
| `eventprint.service` | die Fotobox |
| `eventprint-update.service` | holt beim Hochfahren den aktuellen Stand |

Der Dienst läuft als eigener Nutzer `eventprint` in den Gruppen `lp`,
`lpadmin`, `plugdev` und `video`. `lpadmin` ist die SystemGroup von CUPS und
nicht verzichtbar: nur damit darf die Fotobox die Fehlerbehandlung ihrer
Warteschlange auf `abort-job` stellen. Seine Daten legt der Dienst unter
`StateDirectory` ab, das systemd auf `/var/lib/eventprint` setzt und das nago
von sich aus auswertet.

Das Einrichten des Druckers gehört **nicht** dazu — das hängt am Gerät und an
seiner Seriennummer, siehe [DRUCKER.md](DRUCKER.md).

Scheitert der Bau an gocv, ist fast immer die OpenCV-Fassung der Distribution
älter als die, gegen die gocv übersetzt. Dann hilft:

```bash
sudo ./scripts/install.sh --no-facecrop
```

Die Fotobox läuft damit ohne Gesichtserkennung; der Bildausschnitt ist dann
durchgehend mittig, also genau das Verhalten, das auch die Erkennung ohne
Treffer zeigt.

### Aktuell bleiben

`eventprint-update.service` läuft vor der Fotobox und holt den Stand von
`origin/<branch>`. Hat sich nichts geändert, endet er sofort; sonst baut er neu.

Zwei Regeln bestimmen das Verhalten:

* **Der Dienst startet immer.** `update.sh` endet grundsätzlich mit Erfolg, und
  die Unit ist über `Wants=` verknüpft, nicht über `Requires=`. Kein Netz auf
  einer Feier ist der Normalfall, kein Fehlerfall.
* **Es bleibt immer eine lauffähige Binärdatei da.** Gebaut wird neben den
  laufenden Stand und erst nach Erfolg umgehängt.

Geprüft wird beim Hochfahren nicht. Ein roter Testlauf um 18 Uhr auf einem
Raspberry Pi stünde niemandem zur Verfügung, und die Feier begänne trotzdem.
Dafür ist `scripts/build.sh` da.

Das Gerät ist ein Abbild des Repositories, keine Arbeitskopie: `update.sh`
setzt mit `git reset --hard` auf den Stand der Gegenseite. Lokale Änderungen
unter `/opt/eventprint` gehen dabei verloren.

Damit das nicht das falsche Verzeichnis trifft, bricht `update.sh` ab, sobald
das Arbeitsverzeichnis lokale Änderungen hat. Ein Gerätecheckout hat nie
welche; ein Entwicklungsverzeichnis immer. Das Skript bezieht sein Ziel aus
seinem eigenen Ort, nicht aus dem Arbeitsverzeichnis des Aufrufers — ein Aufruf
mit absolutem Pfad aus einem anderen Verzeichnis heraus meint also weiterhin
den Checkout, in dem das Skript liegt.

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

## Uploads aus dem Internet

`photoupld` ist eine zweite Nago-Anwendung für Installationen, bei denen die
Fotobox hinter NAT in einem privaten Gastnetz steht. Nur `photoupld` muss aus
dem Internet erreichbar sein; die Fotobox baut ausschließlich ausgehende
HTTPS-Verbindungen auf.

1. `go run ./cmd/photoupld` auf dem öffentlichen Server starten.
2. Als `admin@localhost` anmelden und unter **Einstellungen → Foto-Upload** die
   öffentliche Basis-URL eintragen.
3. Im Admin-Center einen Access Token ohne Impersonation erstellen und ihm die
   Rolle **Fotobox-Relay** zuweisen. Den Klartext-Token sofort sicher ablegen.
4. In der Fotobox unter **Einstellungen → Fotobox** die Felder
   **Upload-Service** und **Upload-Token** ausfüllen. Die Verbindung wird ohne
   Neustart aufgebaut; der Token wird in der Oberfläche als Geheimnisfeld
   behandelt.

Die Fotobox fordert beim Start eine zufällige Upload-ID an und setzt den
QR-Code automatisch auf die zurückgelieferte URL. Sie fragt alle zehn Sekunden
nach neuen Aufträgen. Nach einem Neustart einer der Anwendungen wird eine neue
ID erzeugt; alte Links zeigen Gästen ausdrücklich, dass sie den QR-Code erneut
scannen müssen.

Upload-IDs und Warteschlangen existieren nur im Arbeitsspeicher. Bilder liegen
für Vorschau und Abholung kurzfristig im Nago-Image-Store: nach erfolgreicher
Übernahme werden Original und Bildpyramide gelöscht, unabgeholte Daten nach 30
Minuten. Beim Neustart entfernt `photoupld` verbliebene Relay-Bilder.

## Kamera anschließen

Die Fotobox erkennt unterstützte USB/PTP-Kameras mit `gphoto2` automatisch.
Für den automatischen Polaroid-Bildausschnitt verwendet sie außerdem GoCV mit
OpenCV 4.x und dem eingebetteten YuNet-Modell. Die Systemabhängigkeiten müssen
vor dem Bauen und Starten der Fotobox einmalig installiert sein:

```bash
sudo apt install gphoto2 libopencv-dev pkg-config
```

Der Fotobox-Build benötigt deshalb aktiviertes CGO und eine über
`pkg-config --modversion opencv4` auffindbare OpenCV-Installation. Der separat
gebaute Upload-Service benötigt OpenCV nicht.

Die Kamera kann jederzeit an- oder abgesteckt werden. Spätestens nach zehn
Sekunden startet die Fotobox den Tethering-Betrieb; nach einer Trennung sucht
sie automatisch erneut. Beim Auslösen lädt `gphoto2` die Aufnahme herunter,
belässt das Original auf der Speicherkarte und die Fotobox übernimmt sie in
Historie und Galerie.

Unter **Einstellungen → Fotobox → Kamera** lässt sich der automatische Druck
abschalten und das Standardlayout wählen. Standardmäßig wird jede Aufnahme
sofort als **Polaroid** gedruckt. Heruntergeladene Dateien werden erst nach
erfolgreichem Import und gegebenenfalls erfolgreichem Einreihen des
Druckauftrags entfernt.

Unter **Einstellungen → Fotobox → Bildausschnitt** kann der automatische
Polaroid-Bildausschnitt deaktiviert werden. Er ist standardmäßig aktiv und
richtet Gruppen sowie Einzelpersonen anhand erkannter Gesichter aus. Werden
keine Gesichter erkannt, bleibt es beim mittigen Standardausschnitt.

## Archiv der Originale

Jedes Bild – von der Kamera, vom Gast-Upload und aus dem Internet – wird beim
Import zusätzlich unverändert in einem gewöhnlichen Ordner gesichert:

```
<Datenverzeichnis>/photos/originals/
```

Das Datenverzeichnis nennt die Anwendung beim Start (`photo archive ready`);
die Historie zeigt den Pfad nach der Anmeldung als Betreuer ebenfalls an.

Gesichert wird die Datei **vor** jeder Verarbeitung, also mit EXIF-Block,
Aufnahmezeit und ursprünglicher Kompression. Das unterscheidet den Ordner vom
internen Bildspeicher, in dem gedrehte Aufnahmen aufgerichtet und dabei neu
kodiert werden.

Der Dateiname beginnt mit der Foto-ID, die den Zeitstempel in Millisekunden
enthält:

```
1788201767405-176029a2a2b47578_DSC02301.jpg
```

Damit entspricht die alphabetische Sortierung im Dateimanager der zeitlichen,
und der ursprüngliche Name bleibt zur Wiedererkennung erhalten. Nach der Feier
genügt es, den Ordner zu kopieren.

Zwei Eigenschaften sind bewusst so gewählt:

* **Der Ordner wird nur beschrieben.** Wird ein Foto in der Historie gelöscht,
  verschwindet es aus der Fotobox, die Datei im Archiv bleibt. Soll ein Bild
  wirklich verschwinden, muss es dort von Hand entfernt werden.
* **Ein Fehler beim Sichern bricht den Import nicht ab.** Eine volle Platte
  darf nicht dazu führen, dass auf einer Feier nichts mehr gedruckt wird. Der
  Fehler steht im Protokoll (`cannot archive original photo`).

## Aufbau

Die Anwendung folgt dem Layout, das speclink unter dem Profil `go_nago_ddd1`
prüft: Fachlichkeit unter `app/<kontext>/`, ein Anwendungsfall je Datei mit dem
Namen des Anwendungsfalls, Verdrahtung an genau einer Stelle.

```
app/photo/                  Domäne der Fotos (Import, Historie, Originaldaten)
app/printing/               Layouts, Rendering, Druckaufträge, CUPS-Anbindung
app/upld/                   Transiente Sitzungen und Upload-Warteschlangen

app/photobox/cfg/           Enable() – verdrahtet die Fotobox
app/photobox/cfg/camera/    Übernahme der Kamerabilder aus dem Tethering-Ordner
app/photobox/cfg/remote/    Ausgehender photoupld-Client und Abfrage
app/photobox/ui/            Oberfläche der Fotobox (Paket uiphotobox)

app/photoupld/cfg/          Enable() und REST-API des Upload-Relais
app/photoupld/ui/           Upload-Seite für das Smartphone (Paket uiphotoupld)

pkg/orient/                 EXIF-Ausrichtung, ohne Bezug zur Domäne
pkg/facecrop/               Gesichtserkennung, ohne Bezug zur Domäne
pkg/permtext/               Übersetzbare Texte für Berechtigungen

requirements/               Anforderungen und ihre Quelldokumente
cmd/photobox/               Startpunkt inkl. Scaffold-Menü
cmd/photoupld/              Startpunkt des öffentlichen Upload-Relais
```

Drei Regeln daraus, die beim Lesen sonst überraschen:

* **Ein Anwendungsfall je Datei, benannt nach ihm.** `FindAllJobs` steht in
  `uc_find_all_jobs.go`, samt Typ, Konstruktor und der Anmerkung
  `uc_find_all_jobs.annotation.go`, die ihn an eine Anforderung bindet.
* **Je Anwendungsfall genau eine Berechtigung**, geprüft in seiner
  Umsetzung. Das macht Rechte zuteilbar, statt sie in der Oberfläche zu
  verstecken.
* **`camera` und `remote` liegen unter `cfg/`**, nicht unter `pkg/`. Sie rufen
  Anwendungsfälle auf und sind damit Verdrahtung, keine Infrastruktur. Was
  unter `pkg/` liegt, kennt die Domäne nicht – das ist prüfbar und wird
  geprüft.

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

### Exakter CZ-01-Rastervertrag

Gutenprints PPD deklariert für 4x6 eine Bildfläche von **1266x1836 Pixeln bei
300 dpi**. Sie enthält den randlosen Überstand um das sichtbare
1200x1800-Pixel-Papier. Die Anwendung erzeugt dieses CUPS-Raster selbst,
validiert JPEG, PPD, Rasterheader, Dateilänge und abschließend alle drei
1408x1836-Druckerebenen. Erst danach wird der fertige Strom raw an CUPS
übergeben. Eine falsche Größe erreicht den Drucker nicht.

Rahmen und Polaroid-Layout werden auf der sichtbaren 1200x1800-Fläche
berechnet; Full-Bleed belegt einschließlich Überstand die gesamte
1266x1836-Fläche. Die gemessene Tonwertkurve wird vor der Rastererzeugung
angewendet und genau einmal an Gutenprint übergeben.

**Auflösung.** Der CZ-01 beherrscht ausschließlich 300x300 dpi; eine höhere
Stufe gibt es nicht. Das ist keine Einschränkung des Treibers: Gutenprint
bietet für die verwandten Modelle CW-02, CX-02 und DNP DS620 sehr wohl
zusätzlich 300x600 dpi an, für den CZ-01 dagegen nur einen einzigen Eintrag.

```bash
/usr/lib/cups/driver/gutenprint.5.3 cat gutenprint.5.3://citizen-cz-01/expert | grep "^\*Resolution "
```

**Was sonst noch geprüft wurde:**

* `StpImageType=Photo` – gesetzt, aktiviert die Farbaufbereitung für Fotos
  statt der Vorgabe `TextGraphics`.
* `StpColorPrecision=Best` – **wirkungslos, Ursache geklärt.** Das PPD setzt
  für *beide* Stufen `cupsBitsPerColor 8`; bei `Best` kommt lediglich der
  Hinweis `cupsPreferredBitsPerColor 16` hinzu. Die kontrollierte
  Rasterpipeline liefert RGB mit 8 Bit pro Kanal; da die Vorlagen ohnehin
  8-Bit-JPEGs von Kameras und Smartphones sind, wäre nichts zu gewinnen.
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

## Betrieb vor Ort

### Kiosk

`install.sh` richtet ein eigenes, eingeschränktes Konto `fotobox` ein: keine
Shell, gesperrtes Passwort, kein sudo. Es meldet sich automatisch an und
startet Chromium im Vollbild auf `http://localhost:3000`.

**X11 statt Wayland, und das ist keine Geschmacksfrage.** labwc und wlroots
kennen keinen Clone-Modus; zwei Ausgänge zeigen dort zwangsläufig
verschiedene Ausschnitte. Der Fernseher als gespiegelter zweiter Bildschirm
verlangt `xrandr --same-as` und damit X11. Als Fenstersteuerung dient openbox
mit einer `rc.xml` **ohne jede Tastenbindung** – der Raspberry Pi 400 ist
selbst eine Tastatur, ein Gast hat sie also immer in der Hand.

`eventprint-mirror-displays` läuft als Schleife und sucht die beste Auflösung,
die *beide* Geräte können. Ein Fernseher mit 4K würde sonst das Layout des
Touchscreens verschieben und die Bedienflächen unerreichbar machen. Er darf
beim Hochfahren fehlen und später dazukommen.

### Betreuer-PIN

Die Fotobox hat ab Werk **keine** PIN. Wer davorsteht, vergibt sie:
fünfmal zügig den QR-Code antippen, dann das Ziffernfeld. Ab der ersten
Vergabe kommt nur noch hinein, wer die bisherige PIN kennt.

> Das gehört an den Aufbau, nicht auf den Abend. Bis die PIN vergeben ist,
> könnte sie jeder vergeben, der davorsteht – anders ginge es nicht, denn ein
> Geheimnis, das niemand kennt, sperrt auch den Aufbauenden aus.

Die PIN liegt als Argon2-Ableitung in den Einstellungen, nie im Klartext. Nach
drei Fehlversuchen sperrt die Eingabe für wachsende Zeit, gedeckelt bei 15
Minuten; der Zähler gilt anwendungsweit, damit ein privates Fenster ihn nicht
umgeht. Eine Freischaltung verfällt nach 30 Minuten, weil die Box
unbeaufsichtigt steht.

### Drucker

Die CUPS-Warteschlange richtet `install.sh` selbst ein: `lpinfo` liefert das
USB-Ziel, daraus folgt die Gutenprint-PPD. Sie wird gegen das Format
`w288h432` geprüft, für das die Anwendung ihren Raster baut – sonst fiele der
Fehler erst beim ersten Druckversuch auf. Der Name der Warteschlange landet als
`EVENTPRINT_PRINTER` in `/etc/default/eventprint`.

Findet das Skript keinen oder mehrere Drucker, richtet es **nichts** ein und
sagt das. Raten wäre hier schlimmer als nichts zu tun.

## Tests

Die Aufteilung folgt einer Regel: **Im Browser steht nur, was ein Browser
beweisen muss.** Alles, was eine Aussage über Go-Werte ist, gehört in
`go test` – dort kostet es Millisekunden statt einen Anwendungsstart.

| Frage | Wo sie beantwortet wird |
|---|---|
| Erscheint die Oberfläche überhaupt? | Browser |
| Sind alle Seiten über das Menü erreichbar? | Browser |
| Funktioniert der Ablauf eines Abends im Zusammenspiel? | Browser |
| Kommt eine Vorbelegung aus der Umgebung an? | Go |
| Ist jede Berechtigung einer Rolle zugeteilt? | Go |
| Was meldet eine unbekannte CUPS-Warteschlange? | Go |
| Wird aus einem Import ein fertiger Druckauftrag? | Go |
| Wie sieht ein Layout auf dem Papier aus? | Go, am Pixel |

### Fachlichkeit (Go)

```bash
go test ./...
```

Die Tests des Renderers prüfen die Kernanforderung direkt am Pixel: exakte
Papiergeometrie (1200x1800 @ 300 dpi), keine weißen Ecken beim formatfüllenden
Layout, ringsum exakt gleich breiter Rahmen beim Passepartout, breiterer Steg
unten beim Polaroid.

Dazu kommen die Prüfungen, die früher nur der Browser abdeckte:

* `app/photobox/cfg/perm_test.go` – jede deklarierte Berechtigung erreicht eine
  Rolle. Ohne das bleibt eine Seite im Betrieb leer, und zwar erst dann, wenn
  ein Gast davorsteht.
* `app/photobox/cfg/defaults_test.go` – eine Vorbelegung aus der Umgebung
  überschreibt nie eine getroffene Wahl, und eine abgeschaltete Automatik
  springt beim nächsten Start nicht wieder an.
* `app/photobox/cfg/flow_test.go` – der Weg vom Import über den Druck bis in
  die Historie, beide Kontexte verdrahtet wie im Betrieb.
* `app/printing/cups_printer_status_test.go` – was eine fehlende oder
  angehaltene Warteschlange meldet, gegen ein vorgetäuschtes `lpstat`.

Zum Ansehen der gerenderten Layouts:

```bash
EVENTPRINT_TEST_OUTPUT=/tmp/tpl go test ./app/printing/
```

### Oberfläche (Playwright)

Nago ist eine Single-Page-Anwendung, die ihren gesamten Zustand über eine
WebSocket-Verbindung bezieht. Ein HTTP-Abruf sagt darüber nichts – diese Tests
steuern einen echten Browser.

```bash
python3 -m venv .venv
.venv/bin/pip install playwright pytest
.venv/bin/playwright install chromium

.venv/bin/python -m pytest
```

Es sind bewusst nur vier Tests. Jeder startet die Anwendung in einem leeren
Datenverzeichnis auf einem freien Port, denn Fotos und Druckaufträge bleiben
gespeichert und die Tests hingen sonst voneinander ab. Ein Test kostet damit
rund acht Sekunden, und das ist der Grund, warum hier nichts steht, was auch
in Go stehen könnte.

> **Stolperfalle:** Nago mountet die SPA nicht, wenn der User-Agent als Crawler
> erkannt wird – der Bundle prüft ihn gegen eine Bot-Liste und ruft
> `mount("#app")` gar nicht erst auf. Playwrights Standard-User-Agent enthält
> `HeadlessChrome` und fällt in genau dieses Raster; die Seite bleibt dann
> dauerhaft leer, ohne jede Fehlermeldung. `e2e/conftest.py` setzt deshalb
> einen realistischen User-Agent.
>
> Ebenfalls beachten: Die Menüeinträge des Scaffolds sind keine `<a>`-Elemente,
> sondern klickbare Container. `get_by_role("link", …)` findet sie nicht.
