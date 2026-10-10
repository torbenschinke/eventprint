#!/usr/bin/env bash
# Sitzung des Kiosk-Nutzers: Bildschirme einrichten, der Fotobox den
# Bildschirm ueberlassen.
#
# Die Oberflaeche ist ein eigenes Programm, das der Dienst eventprint.service
# als Nutzer eventprint startet. Diese Sitzung stellt nur den Bildschirm
# bereit: labwc, ohne Leiste, ohne Menue, ohne Dateimanager – nichts, worueber
# ein Gast die Fotobox verlassen koennte. labwc startet das Skript ueber
# ~/.config/labwc/autostart; die Fotobox zeichnet als X11-Programm in dessen
# Xwayland.
set -uo pipefail

# Alles ins Journal. Ein Kiosk, der unbeaufsichtigt laeuft, muss sagen koennen,
# was ihm fehlt.
if command -v logger >/dev/null 2>&1; then
  exec > >(logger -t eventprint-kiosk) 2>&1
fi

echo "Sitzung startet"

# Die Einstellungen der Box gelten auch hier, vor allem EVENTPRINT_ROTATE:
# Wer das Panel andersherum einbaut, stellt die Drehung dort um, wo auch alles
# andere steht, und startet neu. Der Touch zieht mit.
if [[ -r /etc/default/eventprint ]]; then
  set -a
  # shellcheck disable=SC1091
  . /etc/default/eventprint
  set +a
fi

# Erst einrichten – Drehung, Spiegelung –, dann im Hintergrund auf neue
# Bildschirme achten. Die Fotobox liest die Groesse beim Start; sie darf den
# Bildschirm deshalb erst danach bekommen.
/usr/local/bin/eventprint-displays --once

# Dem Dienstnutzer den Bildschirm freigeben – genau ihm und niemandem sonst.
# SI:localuser prueft die Kennung des verbindenden Prozesses ueber den lokalen
# Socket; ein Rechner im Netz kommt so nicht an den Bildschirm. Der Aufruf
# startet auch Xwayland, das labwc sonst erst beim ersten Fenster hochfaehrt.
if xhost "+SI:localuser:eventprint" >/dev/null; then
  echo "Bildschirm fuer eventprint freigegeben"
else
  echo "xhost fehlgeschlagen - die Fotobox kann nicht zeichnen"
fi

exec /usr/local/bin/eventprint-displays
