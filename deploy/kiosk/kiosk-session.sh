#!/usr/bin/env bash
# Sitzung des Kiosk-Nutzers: Bildschirme spiegeln, Browser im Vollbild starten.
#
# Openbox statt eines vollstaendigen Desktops: Es hat keine Leiste, kein Menue,
# keinen Dateimanager - nichts, worueber ein Gast die Fotobox verlassen koennte.
# Die Fenstersteuerung wird ohnehin nie gebraucht, weil genau ein Fenster
# laeuft.
set -uo pipefail

# Alles ins Journal.
#
# Vorher ging die Ausgabe nach ~/.xsession-errors, im Heimverzeichnis des
# Kiosk-Nutzers und damit fuer niemanden lesbar ausser root. Als der Browser
# beim ersten Kaltstart wortlos verschwand, war die Ursache nicht auffindbar.
# Ein Kiosk, der unbeaufsichtigt laeuft, muss sagen koennen, was ihm fehlt.
if command -v logger >/dev/null 2>&1; then
  exec > >(logger -t eventprint-kiosk) 2>&1
fi

URL="${EVENTPRINT_KIOSK_URL:-http://localhost:3000}"

echo "Sitzung startet, Ziel ${URL}"

# Kein Bildschirmschoner, kein Abschalten. Die Fotobox steht den Abend ueber
# ungenutzt herum, und ein schwarzer Bildschirm sieht aus wie ein Defekt.
xset s off
xset s noblank
xset -dpms

# Den Mauszeiger verstecken, falls das Werkzeug da ist. Auf einem Touchscreen
# ist er nur ein Fleck, den niemand wegbekommt.
if command -v unclutter >/dev/null 2>&1; then
  unclutter -idle 1 -root &
fi

/usr/local/bin/eventprint-mirror-displays &

# Warten, bis die Fotobox antwortet. Ohne das zeigt Chromium die Fehlerseite
# "Website nicht erreichbar" und bleibt darauf stehen, weil der Dienst ein paar
# Sekunden spaeter als die Sitzung bereit ist.
for _ in $(seq 1 120); do
  if curl --silent --fail --max-time 2 --output /dev/null "${URL}"; then
    echo "Fotobox antwortet"
    break
  fi

  sleep 1
done

# Der Browser laeuft unter Aufsicht.
#
# Vorher stand hier exec: Beendete sich Chromium - Absturz, zu wenig Speicher,
# ein Fehlgriff auf dem Touchscreen -, blieb der Bildschirm bis zum naechsten
# Neustart leer. Auf einer Feier merkt das niemand rechtzeitig. Jetzt startet
# er neu.
backoff=2
while true; do
  # Chromium meldet nach einem harten Ausschalten "Wiederherstellen?" und
  # bleibt mit einem Dialog stehen, den vor Ort niemand wegklickt. Die Fotobox
  # wird aber genau so ausgeschaltet: am Stecker. Deshalb werden die
  # Absturzspuren vor jedem Start entfernt.
  profile="${HOME}/.config/chromium/Default/Preferences"
  if [[ -f "${profile}" ]]; then
    sed -i 's/"exit_type":"Crashed"/"exit_type":"Normal"/; s/"exited_cleanly":false/"exited_cleanly":true/' "${profile}"
  fi

  echo "starte Chromium"

  chromium \
    --kiosk \
    --app="${URL}" \
    --start-fullscreen \
    --noerrdialogs \
    --disable-infobars \
    --no-first-run \
    --disable-session-crashed-bubble \
    --disable-features=Translate,TranslateUI \
    --disable-pinch \
    --overscroll-history-navigation=0 \
    --touch-events=enabled \
    --check-for-update-interval=31536000 \
    --password-store=basic \
    --autoplay-policy=no-user-gesture-required

  status=$?
  echo "Chromium beendet mit ${status}, Neustart in ${backoff}s"

  sleep "${backoff}"

  # Sanft ansteigen, damit ein dauerhaft kaputter Browser nicht die Maschine
  # mit Startversuchen belegt. Gedeckelt, damit die Box wieder hochkommt, wenn
  # die Ursache voruebergehend war.
  backoff=$(( backoff * 2 ))
  if [[ ${backoff} -gt 30 ]]; then
    backoff=30
  fi
done
