# Citizen CZ-01 unter Debian 13 (trixie) einrichten

Dokumentation der Einrichtung des Citizen CZ-01 Fotodruckers (10x15 cm / 4x6") via USB
unter Debian 13 mit CUPS + Gutenprint, inkl. der Stolperfallen, die dabei auftraten.

Getestet auf: Debian GNU/Linux 13 (trixie), arm64, CUPS 2.4.10, Gutenprint 5.3.4.

---

## 1. Hardware prüfen

```bash
lsusb | grep -i citizen
# Bus 001 Device 005: ID 1343:000c Citizen Systems Photo Printer
```

Vendor/Product-ID: `1343:000c`

## 2. Pakete installieren

```bash
sudo apt-get install -y printer-driver-gutenprint imagemagick
```

* `printer-driver-gutenprint` – enthält den PPD-Treiber **und** den speziellen
  Dyesub-Backend `gutenprint53+usb`.
* `imagemagick` – wird vom Druckscript für den Auto-Crop benötigt.

Prüfen, ob der Treiber erkannt wird:

```bash
lpinfo -m | grep -i cz-01
# gutenprint.5.3://citizen-cz-01/expert   Citizen CZ-01 - CUPS+Gutenprint v5.3.4
```

## 3. Geräte-URI ermitteln — **wichtig**

Der Gutenprint-Dyesub-Backend muss direkt aufgerufen werden, um die korrekte URI
zu bekommen:

```bash
sudo /usr/lib/cups/backend/gutenprint53+usb
# direct gutenprint53+usb://citizen-cz-01/RZ2C57003197 "CITIZEN SYSTEMS CZ-01" ...
```

Der Teil nach dem letzten `/` ist die Seriennummer des Geräts. Diese ist
gerätespezifisch — bei einem anderen Drucker den obigen Befehl erneut ausführen.

> **Nicht** die URI aus `lpinfo -v` (`usb://CITIZEN%20SYSTEMS/CZ-01?serial=...`)
> verwenden. Siehe Abschnitt „Stolperfallen".

## 4. Druckwarteschlange anlegen

```bash
sudo lpadmin -p CZ01 -E \
  -v "gutenprint53+usb://citizen-cz-01/RZ2C57003197" \
  -m "gutenprint.5.3://citizen-cz-01/expert" \
  -o printer-is-shared=false

sudo lpadmin -d CZ01     # als Standarddrucker setzen
lpstat -p -d
```

Die Warnung `lpadmin: Druckertreiber sind veraltet ...` ist unkritisch (CUPS
deprecated klassische PPD-Treiber, funktioniert in 2.4.x aber weiterhin).

## 5. Verfügbare Optionen ansehen

```bash
lpoptions -p CZ01 -l
```

Relevante Werte:

| Option | Wert | Bedeutung |
|---|---|---|
| `PageSize` | `w288h432` | 4x6" = 10x15 cm (288/432 Punkte à 1/72") |
| `Resolution` | `300dpi` | einzige unterstützte Auflösung |
| `StpImageType` | `Photo` | Farbaufbereitung für Fotos |

Weitere Formate des CZ-01: `w288h288` (4x4"), `w288h216` (4x3"),
`w288h432-div2` (2x 4x3" Split), `w324h432` (4.5x6") usw.

---

## Drucken

### Korrigierten 4x6-Filter installieren

```bash
./scripts/install-gutenprint-cz01.sh
```

Der Filter wird isoliert unter `/opt/eventprint/gutenprint` installiert. Die
Fotobox verwendet ihn automatisch und fällt mit einer Warnung im Log auf den
Systemtreiber zurück, falls er nicht verfügbar ist.

### Warum der Crop nötig ist

Der CZ-01 druckt physikalisch randlos, aber CUPS skaliert ein Bild mit
abweichendem Seitenverhältnis **einpassend** (Letterbox) → weiße Balken.
Das Testbild war 3063x5445 px (16:9), 4x6" benötigt aber 3:2. Deshalb wird
vorab serverseitig zugeschnitten, statt sich auf Druckeroptionen zu verlassen.
So ist das Ergebnis deterministisch und vorher prüfbar.

### Status prüfen

```bash
lpstat -o          # offene Jobs
lpstat -p CZ01     # Druckerstatus
cancel -a          # alle Jobs abbrechen
```

---

## Stolperfallen (aufgetretene Probleme)

### 1. Falscher CUPS-Backend → Job hängt endlos

**Symptom:**
```
Drucker CZ01 druckt jetzt CZ01-1.
	Warte darauf dass der Drucker verfügbar wird.
```
Der Job blieb dauerhaft in der Queue, ohne dass etwas gedruckt wurde.

**Ursache:** Die Queue wurde mit der von `lpinfo -v` gemeldeten URI
`usb://CITIZEN%20SYSTEMS/CZ-01?serial=...` angelegt. Der generische CUPS-`usb`-Backend
findet den CZ-01 jedoch nicht (`libusb_get_device_list=7`, danach Endlosschleife),
obwohl das Gerät sich als USB-Klasse 7 (Printer, bidirektional) meldet.

**Lösung:** Gutenprints eigenen Dyesub-Backend verwenden:
```bash
sudo lpadmin -p CZ01 -v "gutenprint53+usb://citizen-cz-01/RZ2C57003197"
sudo cupsenable CZ01
```

**Diagnose-Hinweis:** Solange ein Job hängt, hält dessen Backend-Prozess das
USB-Gerät belegt — `sudo /usr/lib/cups/backend/gutenprint53+usb` listet dann
nichts. Vorher immer `cancel -a` ausführen.

### 2. Script bricht still ab (Bash-Falle)

**Symptom:** `./print.sh` erzeugte keinerlei Ausgabe und keinen Druckjob.

**Ursache:**
```bash
read -r W H < <(magick identify -format "%w %h" "$IMAGE[0]")
```
`magick identify -format` gibt **kein abschließendes Newline** aus. `read` liefert
in diesem Fall Exitcode 1 (obwohl die Variablen korrekt gesetzt sind), und wegen
`set -e` beendete sich das Script kommentarlos an dieser Stelle.

**Lösung:** Newline im Format ergänzen:
```bash
read -r W H < <(magick identify -format "%w %h\n" "$IMAGE[0]")
```

**Diagnose:** `bash -x ./print.sh` zeigte, dass die Ausführung exakt nach dem
`magick identify` endete.

### 3. Logging aktivieren

`/var/log/cups/error_log` existiert bei Debian erst, wenn Debug-Logging aktiv ist:

```bash
sudo cupsctl --debug-logging
# ... reproduzieren ...
sudo grep -a "Job 3\]" /var/log/cups/error_log
sudo cupsctl --no-debug-logging     # danach wieder ausschalten
```

Erfolgreicher Druck sieht so aus:
```
[Job 3] Printing page 1 (1 copies)
[Job 3] PID ... (/usr/lib/cups/filter/rastertogutenprint.5.3) exited with no errors.
[Job 3] Print complete (0 copies remaining)
[Job 3] Job completed.
```

---

## Wartung

Wenn der Drucker nach einem Wechsel/Neuanschluss nicht mehr reagiert, hat sich
vermutlich die Seriennummer in der URI geändert:

```bash
cancel -a
sudo /usr/lib/cups/backend/gutenprint53+usb          # neue URI ablesen
sudo lpadmin -p CZ01 -v "<neue URI>"
```
