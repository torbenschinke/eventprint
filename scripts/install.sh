#!/usr/bin/env bash
#
# Richtet eventprint auf einer frischen Maschine ein (Raspberry Pi OS oder
# Debian). Erwartet root, ist wiederholbar und ändert nichts, was schon stimmt.
#
#   sudo ./scripts/install.sh --dry-run     zeigt nur, was geschähe
#   sudo ./scripts/install.sh               führt es aus
#
# Was danach da ist:
#   /opt/eventprint          Arbeitskopie des Repositories
#   /var/lib/eventprint      Daten: Fotos, Druckaufträge, Archiv der Originale
#   eventprint.service       die Fotobox
#   eventprint-update.service  holt beim Hochfahren den aktuellen Stand
#
# Ausdrücklich nicht enthalten ist das Einrichten des Druckers. Das hängt am
# Gerät und an dessen Seriennummer und steht in DRUCKER.md.

set -euo pipefail

REPO_URL="${EVENTPRINT_REPO:-https://github.com/torbenschinke/eventprint.git}"
BRANCH="${EVENTPRINT_BRANCH:-master}"
PREFIX="/opt/eventprint"
STATE_DIR="/var/lib/eventprint"
SERVICE_USER="eventprint"
DRY_RUN=0
FACECROP=1

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m!!\033[0m %s\n' "$*" >&2; exit 1; }

# run führt aus oder zeigt nur. Jede Änderung am System geht hier durch, damit
# --dry-run eine belastbare Aussage ist und keine Behauptung.
run() {
  if [[ ${DRY_RUN} -eq 1 ]]; then
    printf '   \033[2m[dry-run]\033[0m %s\n' "$*"
    return 0
  fi
  "$@"
}

usage() {
  sed -n '2,20p' "${BASH_SOURCE[0]}" | sed 's/^#\s\?//'
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)      DRY_RUN=1 ;;
    --no-facecrop)  FACECROP=0 ;;
    --repo)         REPO_URL="${2:?--repo braucht eine URL}"; shift ;;
    --branch)       BRANCH="${2:?--branch braucht einen Namen}"; shift ;;
    --prefix)       PREFIX="${2:?--prefix braucht einen Pfad}"; shift ;;
    -h|--help)      usage ;;
    *)              die "Unbekannte Option: $1" ;;
  esac
  shift
done

if [[ ${DRY_RUN} -eq 0 && "${EUID}" -ne 0 ]]; then
  die "Bitte als root ausführen (sudo). Zum Ansehen: --dry-run"
fi

[[ -r /etc/os-release ]] || die "/etc/os-release fehlt; dies ist kein Debian-artiges System."
# shellcheck disable=SC1091
. /etc/os-release
command -v apt-get >/dev/null 2>&1 || die "apt-get fehlt; dieses Skript setzt Debian oder Raspberry Pi OS voraus."
log "System: ${PRETTY_NAME:-unbekannt}"

# ------------------------------------------------------------------ Pakete ---
# Jede Zeile mit dem Grund, sonst weiß in einem Jahr niemand mehr, wozu.
PACKAGES=(
  git ca-certificates             # Quelltext holen und TLS
  golang-go                       # nur als Bootstrap; go.mod verlangt mehr und holt es selbst
  build-essential pkg-config      # cgo braucht einen C-Übersetzer
  cups cups-client                # Druckdienst und lp/lpstat/lpadmin
  printer-driver-gutenprint       # Treiber und Backend für den CZ-01
  gphoto2                         # Tethering der Kamera

  # Kioskbetrieb auf dem Touchscreen
  xserver-xorg xinit              # X11; Wayland kann den Fernseher nicht spiegeln
  openbox                         # Fenstersteuerung ohne Menue und ohne Leiste
  lightdm                         # automatische Anmeldung
  chromium                        # die Oberflaeche selbst
  x11-xserver-utils               # xrandr und xset
  unclutter                       # versteckt den Mauszeiger
  curl                            # wartet beim Start auf den Dienst
)

if [[ ${FACECROP} -eq 1 ]]; then
  # gocv übersetzt gegen OpenCV. Ist die Fassung der Distribution zu alt,
  # scheitert der Bau – dafür gibt es weiter unten den Rückfall.
  PACKAGES+=(libopencv-dev)
fi

log "Pakete installieren"
run apt-get update -qq
run env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "${PACKAGES[@]}"

# --------------------------------------------------------------------- Go ---
# Es wird nur ein Bootstrap-Compiler gebraucht, kein aktueller.
#
# Seit Go 1.21 holt sich der Werkzeugkasten die in go.mod verlangte Fassung
# selbst (GOTOOLCHAIN=auto, die Voreinstellung). Das Paket der Distribution
# genügt daher, solange es diese Grenze erreicht – eine zweite Go-Installation
# neben der des Systems wäre nur eine weitere Sache, die altert.
readonly GO_BOOTSTRAP_MIN="1.21"

required_go="$(sed -n 's/^go \([0-9][0-9.]*\).*/\1/p' "${SCRIPT_DIR}/../go.mod" 2>/dev/null | head -1)"
required_go="${required_go:-1.27.0}"

version_ge() {
  # sort -V ist hier genau richtig: Es vergleicht 1.10 korrekt gegen 1.9.
  [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -1)" == "$2" ]]
}

go_version_of() {
  command -v "$1" >/dev/null 2>&1 || return 1
  "$1" env GOVERSION 2>/dev/null | sed 's/^go//'
}

installed_go="$(go_version_of go || true)"

if [[ -z "${installed_go}" ]]; then
  log "Go aus der Distribution installieren"
  run env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends golang-go
  installed_go="$(go_version_of go || true)"

  if [[ ${DRY_RUN} -eq 1 && -z "${installed_go}" ]]; then
    installed_go="${GO_BOOTSTRAP_MIN}"
  fi
fi

if [[ -z "${installed_go}" ]]; then
  die "Nach der Installation von golang-go ist kein go auffindbar."
fi

if version_ge "${installed_go}" "${GO_BOOTSTRAP_MIN}"; then
  log "Go ${installed_go} genügt als Bootstrap; go.mod verlangt ${required_go} und der Werkzeugkasten holt das selbst"
else
  warn "Go ${installed_go} aus der Distribution ist zu alt."
  warn "Erst ab ${GO_BOOTSTRAP_MIN} holt sich der Werkzeugkasten die in go.mod"
  warn "verlangte Fassung (${required_go}) selbstständig nach."
  warn ""
  warn "Zwei Wege:"
  warn "  - eine neuere Fassung des Betriebssystems verwenden, oder"
  warn "  - Go von https://go.dev/dl/ nach /usr/local/go entpacken und"
  warn "    /usr/local/go/bin in PATH aufnehmen"
  die "Kein brauchbarer Bootstrap-Compiler."
fi

# ------------------------------------------------------------------ Nutzer ---
if id -u "${SERVICE_USER}" >/dev/null 2>&1; then
  log "Nutzer ${SERVICE_USER} existiert bereits"
else
  log "Nutzer ${SERVICE_USER} anlegen"
  run useradd --system --create-home --home-dir "${STATE_DIR}" \
      --shell /usr/sbin/nologin "${SERVICE_USER}"
fi

# lpadmin ist die SystemGroup von CUPS: nur damit darf der Dienst die
# Fehlerbehandlung der Warteschlange auf abort-job stellen. Ohne das wiederholt
# CUPS gescheiterte Aufträge selbsttätig und druckt Bilder ein zweites Mal.
for group in lp lpadmin plugdev video; do
  if getent group "${group}" >/dev/null 2>&1; then
    run usermod -aG "${group}" "${SERVICE_USER}"
  else
    warn "Gruppe ${group} existiert nicht und wurde übersprungen."
  fi
done

# ------------------------------------------------------------- Quelltext ---
if [[ -d "${PREFIX}/.git" ]]; then
  log "Repository in ${PREFIX} aktualisieren"
  run git -C "${PREFIX}" remote set-url origin "${REPO_URL}"
  run git -C "${PREFIX}" fetch --quiet origin "${BRANCH}"
  run git -C "${PREFIX}" checkout --quiet -B "${BRANCH}" "origin/${BRANCH}"
else
  log "Repository nach ${PREFIX} klonen"
  run mkdir -p "$(dirname "${PREFIX}")"
  run git clone --quiet --branch "${BRANCH}" "${REPO_URL}" "${PREFIX}"
fi

run chown -R "${SERVICE_USER}:${SERVICE_USER}" "${PREFIX}"
run install -d -o "${SERVICE_USER}" -g "${SERVICE_USER}" -m 0750 "${STATE_DIR}"

# Git weigert sich, in einem Verzeichnis zu arbeiten, das jemand anderem
# gehört. Beim Hochfahren läuft update.sh als ${SERVICE_USER} über /opt.
run git config --system --add safe.directory "${PREFIX}"

# ------------------------------------------------------------------- Bauen ---
log "Anwendungen bauen (das dauert auf einem Raspberry Pi einige Minuten)"

build_env=(
  "HOME=${STATE_DIR}"
  "PATH=/usr/bin:/bin"
  "GOCACHE=${STATE_DIR}/.cache/go-build"
  "GOMODCACHE=${STATE_DIR}/go/pkg/mod"
  "GOTOOLCHAIN=auto"
  "SKIP_TESTS=1"
  "FACECROP=${FACECROP}"
)

if [[ ${DRY_RUN} -eq 1 ]]; then
  printf '   \033[2m[dry-run]\033[0m %s\n' "sudo -u ${SERVICE_USER} env ${build_env[*]} ${PREFIX}/scripts/build.sh"
elif ! sudo -u "${SERVICE_USER}" env "${build_env[@]}" "${PREFIX}/scripts/build.sh"; then
  if [[ ${FACECROP} -eq 1 ]]; then
    warn "Der Bau ist fehlgeschlagen. Häufigste Ursache: Die OpenCV-Fassung der"
    warn "Distribution ist älter als die, gegen die gocv übersetzt."
    warn "Erneut versuchen mit:  sudo $0 --no-facecrop"
    warn "Die Fotobox läuft dann ohne Gesichtserkennung; der Bildausschnitt ist"
    warn "durchgehend mittig."
  fi
  die "Bau fehlgeschlagen."
fi

# ------------------------------------------------------------------ Drucker ---
# Die Warteschlange richtet sich selbst ein. Bisher stand hier nur ein
# Merkzettel, den ein Mensch abtippen sollte - und genau der wird beim Aufbau
# vergessen. Alles, was dafuer noetig ist, steht maschinell fest: lpinfo kennt
# das angeschlossene Geraet und die passende PPD.

# PRINT_QUEUE ist der Name der Warteschlange in CUPS.
PRINT_QUEUE="${PRINT_QUEUE:-CZ01}"

# REQUIRED_PAGE_SIZE ist das Papierformat, fuer das die Anwendung ihren Raster
# baut (10x15 cm). Fehlt es in der PPD, scheitert jeder Druck - siehe
# validateCZ01PPD in app/printing/cups_raster.go.
REQUIRED_PAGE_SIZE="w288h432"

# detect_printer_uri liefert das USB-Ziel des angeschlossenen Druckers.
#
# Es gibt zwei Schreibweisen, und welche erscheint, haengt davon ab, ob der
# Gutenprint-Treiber schon installiert ist:
#
#   usb://CITIZEN%20SYSTEMS/CZ-01?serial=RZ2C57003197   ohne Treiber
#   gutenprint53+usb://citizen-cz-01/RZ2C57003197       mit Treiber
#
# Dieses Skript installiert den Treiber selbst, die zweite Form ist also der
# Normalfall. Die erste bleibt trotzdem gueltig, etwa wenn jemand das Skript
# auf einem System mit vorhandener Einrichtung laufen laesst.
detect_printer_uri() {
  lpinfo -v 2>/dev/null | awk '
    $1 == "direct" && ($2 ~ /^usb:\/\// || $2 ~ /^gutenprint[0-9]*\+usb:\/\//) { print $2 }
  '
}

# model_key liefert die Gutenprint-Kennung des Modells aus dem Geraete-URI.
model_key() {
  local uri="$1" rest model

  rest="${uri#*://}"

  if [[ "${uri}" == gutenprint*+usb://* ]]; then
    # Die Kennung steht bereits im URI: citizen-cz-01/RZ2C57003197
    printf '%s' "${rest%%/*}"
    return 0
  fi

  # Sonst ist das Modell der letzte Pfadbestandteil vor dem Fragezeichen.
  model="${uri%%\?*}"
  model="${model##*/}"
  model="$(printf '%b' "${model//%/\\x}")"        # %20 und Verwandte aufloesen
  model="$(printf '%s' "${model}" | tr 'A-Z ' 'a-z-')"

  [[ -n "${model}" ]] || return 1

  printf '%s' "${model}"
}

# ppd_for_uri sucht die Gutenprint-PPD zum Modell aus dem Geraete-URI.
#
# Bewusst ueber die Modellbezeichnung und nicht ueber eine fest eingetragene
# PPD: So richtet sich auch ein Schwestermodell ein, ohne dass jemand dieses
# Skript anfasst.
ppd_for_uri() {
  local key
  key="$(model_key "$1")" || return 1
  [[ -n "${key}" ]] || return 1

  # Der abschliessende Schraegstrich grenzt cx-02 sauber gegen cx-02w ab.
  # Der fuehrende Bindestrich bzw. Doppelschraegstrich verhindert, dass eine
  # Teilzeichenkette in der Mitte trifft.
  lpinfo -m 2>/dev/null | awk -v full="//${key}/" -v tail="-${key}/" '
    $1 ~ /^gutenprint/ && (index($1, full) > 0 || index($1, tail) > 0) { print $1 }
  '
}

setup_printer_queue() {
  if ! command -v lpinfo >/dev/null 2>&1; then
    warn "lpinfo fehlt; die Warteschlange muss von Hand eingerichtet werden."
    return 1
  fi

  if lpstat -p "${PRINT_QUEUE}" >/dev/null 2>&1; then
    log "Warteschlange ${PRINT_QUEUE} besteht bereits und bleibt unveraendert"
    return 0
  fi

  local uris uri count
  mapfile -t uris < <(detect_printer_uri)

  if [[ ${#uris[@]} -eq 0 ]]; then
    warn "Kein Drucker am USB gefunden. Anschliessen, einschalten und danach:"
    warn "    sudo ${PREFIX}/scripts/install.sh"
    return 1
  fi

  if [[ ${#uris[@]} -gt 1 ]]; then
    warn "Mehrere USB-Drucker gefunden; hier raten waere falsch:"
    printf '       %s\n' "${uris[@]}" >&2
    warn "Bitte den gewuenschten von Hand einrichten."
    return 1
  fi

  uri="${uris[0]}"

  local ppds
  mapfile -t ppds < <(ppd_for_uri "${uri}")
  count=${#ppds[@]}

  if [[ ${count} -eq 0 ]]; then
    warn "Zu ${uri} gibt es keine Gutenprint-PPD. Ist printer-driver-gutenprint"
    warn "installiert und das Modell unterstuetzt?"
    return 1
  fi

  if [[ ${count} -gt 1 ]]; then
    warn "Mehrere PPDs passen auf ${uri}:"
    printf '       %s\n' "${ppds[@]}" >&2
    warn "Bitte die richtige von Hand waehlen."
    return 1
  fi

  log "Warteschlange ${PRINT_QUEUE} einrichten (${ppds[0]})"

  # -o printer-error-policy=abort-job: Ohne das wiederholt CUPS einen
  # gescheiterten Auftrag selbsttaetig und druckt dasselbe Bild ein zweites Mal.
  # Papier und Farbband einer Feier sind gezaehlt.
  run lpadmin -p "${PRINT_QUEUE}" -v "${uri}" -m "${ppds[0]}" \
      -o printer-error-policy=abort-job \
      -o printer-is-shared=false \
      -E

  run cupsenable "${PRINT_QUEUE}"
  run cupsaccept "${PRINT_QUEUE}"

  # Gegenprobe: Die Anwendung baut ihren Raster fuer genau dieses Format und
  # liest es aus der PPD der Warteschlange. Fehlt es, scheitert erst der erste
  # Druckversuch - im Zweifel mitten auf der Feier.
  if [[ ${DRY_RUN} -eq 0 ]]; then
    local ppd="/etc/cups/ppd/${PRINT_QUEUE}.ppd"
    if [[ -r "${ppd}" ]] && ! grep -q "^\*ImageableArea ${REQUIRED_PAGE_SIZE}/" "${ppd}"; then
      warn "Die PPD der Warteschlange kennt das Format ${REQUIRED_PAGE_SIZE} nicht."
      warn "Die Anwendung kann darauf nicht drucken. PPD: ${ppd}"
      return 1
    fi
  fi

  return 0
}

PRINTER_READY=0
if setup_printer_queue; then
  PRINTER_READY=1
fi

# -------------------------------------------------------------------- Kiosk ---
# Ein eigenes, eingeschraenktes Konto fuer den Bildschirm vor Ort.
#
# Nicht das Konto des Betreibers: Wer den Touchscreen bedient, sitzt sonst mit
# dessen Rechten, seinem SSH-Schluessel und seiner Shell vor der Kiste. Der
# Kiosk-Nutzer hat keine Shell, keine sudo-Rechte und ausser dem Browser
# nichts.
KIOSK_USER="${KIOSK_USER:-fotobox}"
KIOSK_URL="${KIOSK_URL:-http://localhost:3000}"

setup_kiosk() {
  # Die Shell ist /bin/bash und ausdruecklich nicht /usr/sbin/nologin.
  #
  # Das war ein Fehlschlag beim ersten Versuch: lightdm startet die grafische
  # Sitzung ueber die Login-Shell des Nutzers. Mit nologin bricht das sofort ab,
  # lightdm faellt auf den Anmeldebildschirm zurueck, und von der Fotobox ist
  # nichts zu sehen.
  #
  # Der Schutz haengt nicht an der Shell, sondern an dreierlei: Das Passwort ist
  # gesperrt, das Konto hat kein sudo, und openbox kennt keine Tastenbindung,
  # ueber die man ein Terminal oeffnen koennte.
  if id -u "${KIOSK_USER}" >/dev/null 2>&1; then
    log "Kiosk-Nutzer ${KIOSK_USER} existiert bereits"
    run usermod --shell /bin/bash "${KIOSK_USER}"
  else
    log "Kiosk-Nutzer ${KIOSK_USER} anlegen"
    run useradd --create-home --shell /bin/bash "${KIOSK_USER}"
  fi

  # Gesperrtes Passwort: Das Konto kommt nur ueber die automatische Anmeldung
  # hinein, nicht ueber die Konsole und nicht ueber SSH.
  run passwd --lock "${KIOSK_USER}"

  # video und input fuer Bildschirm und Touch, sonst nichts. Insbesondere
  # nicht sudo.
  for group in video input render; do
    if getent group "${group}" >/dev/null 2>&1; then
      run usermod -aG "${group}" "${KIOSK_USER}"
    fi
  done

  # Beim Probelauf gibt es das Konto noch nicht, getent scheitert dann. Ohne
  # den Rueckfall stirbt das Skript hier stumm - und der Probelauf ist der
  # einzige Weg, diesen Abschnitt zu pruefen, ohne die Maschine anzufassen.
  local home
  home="$(getent passwd "${KIOSK_USER}" 2>/dev/null | cut -d: -f6 || true)"
  home="${home:-/home/${KIOSK_USER}}"

  run install -m 0755 "${PREFIX}/deploy/kiosk/mirror-displays.sh" /usr/local/bin/eventprint-mirror-displays
  run install -m 0755 "${PREFIX}/deploy/kiosk/kiosk-session.sh" /usr/local/bin/eventprint-kiosk-session

  run install -d -o "${KIOSK_USER}" -g "${KIOSK_USER}" -m 0755 "${home}/.config/openbox"
  run install -o "${KIOSK_USER}" -g "${KIOSK_USER}" -m 0644 \
      "${PREFIX}/deploy/kiosk/openbox-rc.xml" "${home}/.config/openbox/rc.xml"

  if [[ ${DRY_RUN} -eq 1 ]]; then
    printf '   \033[2m[dry-run]\033[0m %s\n' "schreibe ${home}/.config/openbox/autostart"
  else
    cat >"${home}/.config/openbox/autostart" <<EOF
# Von install.sh angelegt. Startet die Fotobox-Sitzung.
EVENTPRINT_KIOSK_URL=${KIOSK_URL} /usr/local/bin/eventprint-kiosk-session &
EOF
    chown "${KIOSK_USER}:${KIOSK_USER}" "${home}/.config/openbox/autostart"
    chmod 0644 "${home}/.config/openbox/autostart"
  fi

  configure_autologin
  disable_console_autologin
}

# disable_console_autologin schliesst eine Tuer neben dem Kiosk.
#
# Raspberry Pi OS meldet den Erstbenutzer auf tty1 automatisch an. Auf einem
# Arbeitsplatzrechner ist das bequem; an einer unbeaufsichtigten Fotobox mit
# eingebauter Tastatur ist es ein offenes Scheunentor: Strg+Alt+F1 fuehrt an
# eine bereits angemeldete Shell - mit den sudo-Rechten des Betreibers und ohne
# jede Passwortabfrage. Der ganze Aufwand am Kiosk waere damit umsonst.
#
# Die Anmeldung selbst bleibt moeglich, sie verlangt nur wieder ein Passwort.
disable_console_autologin() {
  local dir=/etc/systemd/system/getty@tty1.service.d
  local conf="${dir}/autologin.conf"

  [[ -f "${conf}" ]] || return 0

  if [[ ${DRY_RUN} -eq 1 ]]; then
    printf '   \033[2m[dry-run]\033[0m %s\n' "schalte die automatische Konsolenanmeldung ab (${conf})"
    return 0
  fi

  # Verschieben statt loeschen, damit sich der frühere Zustand nachvollziehen
  # und wiederherstellen laesst.
  mv "${conf}" "${conf}.vor-eventprint"
  systemctl daemon-reload

  log "automatische Konsolenanmeldung auf tty1 abgeschaltet"
  warn "Strg+Alt+F1 verlangt jetzt wieder ein Passwort. Die frühere Einstellung"
  warn "liegt unter ${conf}.vor-eventprint"
}

# configure_autologin traegt die automatische Anmeldung ein.
#
# Und zwar in /etc/lightdm/lightdm.conf selbst, nicht nur in lightdm.conf.d.
# Der Grund ist eine Reihenfolge, die man leicht falsch herum erwartet: lightdm
# liest die Hauptdatei ZULETZT, sie gewinnt also gegen alles in conf.d. Raspberry
# Pi OS traegt dort ab Werk den Erstbenutzer ein. Eine Datei in conf.d allein
# bleibt damit wirkungslos - die Fotobox meldete sich weiter als Betreiber in
# einer Wayland-Sitzung an, ohne Kiosk und ohne Spiegelung.
#
# Die Reihenfolge laesst sich jederzeit nachsehen mit:
#   sudo lightdm --show-config
#
# X11 und nicht Wayland ist dabei keine Geschmacksfrage: labwc und wlroots
# kennen keinen Clone-Modus. Ein Fernseher als zweiter Bildschirm zeigt dort
# zwangslaeufig einen anderen Ausschnitt. xrandr --same-as spiegelt.
configure_autologin() {
  local conf=/etc/lightdm/lightdm.conf
  local backup="${conf}.vor-eventprint"

  if [[ ${DRY_RUN} -eq 1 ]]; then
    printf '   \033[2m[dry-run]\033[0m %s\n' "sichere ${conf} nach ${backup} und traege die Fotobox-Sitzung ein"
    printf '   \033[2m[dry-run]\033[0m %s\n' "schreibe /etc/lightdm/lightdm.conf.d/90-eventprint.conf"
    return 0
  fi

  install -d -m 0755 /etc/lightdm/lightdm.conf.d
  cat >/etc/lightdm/lightdm.conf.d/90-eventprint.conf <<EOF
# Von install.sh angelegt: automatische Anmeldung der Fotobox.
#
# Achtung: /etc/lightdm/lightdm.conf wird SPAETER gelesen und gewinnt gegen
# diese Datei. Die eigentliche Einstellung steht deshalb dort; hier steht sie
# nur fuer den Fall, dass die Hauptdatei die Schluessel gar nicht kennt.
[Seat:*]
autologin-user=${KIOSK_USER}
autologin-user-timeout=0
autologin-session=openbox
user-session=openbox
EOF

  [[ -f "${conf}" ]] || return 0

  # Einmalig sichern, damit der Zustand vor der Fotobox nachvollziehbar bleibt.
  if [[ ! -f "${backup}" ]]; then
    cp -a "${conf}" "${backup}"
    log "bisherige lightdm.conf gesichert nach ${backup}"
  fi

  local key value
  for pair in \
    "autologin-user=${KIOSK_USER}" \
    "autologin-user-timeout=0" \
    "autologin-session=openbox" \
    "user-session=openbox"
  do
    key="${pair%%=*}"
    value="${pair#*=}"

    if grep -qE "^[[:space:]]*${key}=" "${conf}"; then
      sed -i -E "s|^[[:space:]]*${key}=.*|${key}=${value}|" "${conf}"
    else
      # Der Schluessel fehlt: direkt hinter [Seat:*] einfuegen, sonst landete er
      # in einem fremden Abschnitt und bliebe wirkungslos.
      sed -i "0,/^\[Seat:\*\]/s//[Seat:*]\n${key}=${value}/" "${conf}"
    fi
  done

  log "automatische Anmeldung: ${KIOSK_USER} in eine openbox-Sitzung"
}

setup_kiosk

# ----------------------------------------------------------------- systemd ---
log "Dienste einrichten"

if [[ ${DRY_RUN} -eq 1 ]]; then
  printf '   \033[2m[dry-run]\033[0m %s\n' "schreibe /etc/default/eventprint (FACECROP=${FACECROP}, EVENTPRINT_BRANCH=${BRANCH})"
else
  queue_value=""
  if [[ ${PRINTER_READY} -eq 1 ]]; then
    queue_value="${PRINT_QUEUE}"
  fi

  cat >/etc/default/eventprint <<EOF
# Umgebung der eventprint-Dienste. Wird von install.sh angelegt und darf von
# Hand geändert werden.

# Zweig, den die Aktualisierung beim Hochfahren verfolgt.
EVENTPRINT_BRANCH=${BRANCH}

# 0 baut ohne Gesichtserkennung, für Systeme mit zu alter OpenCV-Fassung.
FACECROP=${FACECROP}

# Adresse, auf der die Oberfläche lauscht.
HOST=0.0.0.0

# CUPS-Warteschlange. Leer bedeutet Testbetrieb: Die Fotobox nimmt Aufträge an,
# druckt aber nichts. Ohne diese Zeile lief die Box selbst mit fertig
# eingerichtetem Drucker im Testbetrieb weiter - ein Fehler, der erst auffällt,
# wenn am Abend kein Blatt kommt.
EVENTPRINT_PRINTER=${queue_value}
EOF
fi

for unit in eventprint-update.service eventprint.service; do
  run install -m 0644 "${PREFIX}/deploy/systemd/${unit}" "/etc/systemd/system/${unit}"
done

run systemctl daemon-reload
run systemctl enable eventprint-update.service eventprint.service
run systemctl enable cups.service

log "fertig"

if [[ ${PRINTER_READY} -eq 1 ]]; then
  printer_line="    Drucker:       ${PRINT_QUEUE} ist eingerichtet und aktiv."
else
  printer_line="    Drucker:       NICHT eingerichtet - die Fotobox laeuft im Testbetrieb.
                   Drucker anschliessen, einschalten, dann dieses Skript erneut."
fi

cat <<EOF

  Stand:
${printer_line}
    Oberflaeche:   http://$(hostname -I 2>/dev/null | awk '{print $1}'):3000

  Naechste Schritte:

    1. Neu starten. Danach laeuft alles von selbst:
         sudo reboot

    2. Beim ersten Aufruf eine Betreuer-PIN vergeben:
         Auf dem Startbildschirm fuenfmal zuegig den QR-Code antippen.
         Die Fotobox ist fabrikneu und hat noch keine PIN, vergibt sie also,
         wer davorsteht. Das gehoert an den Aufbau - nicht auf den Abend,
         wenn schon Gaeste da sind.

  Beim naechsten Hochfahren holt eventprint-update.service den Stand von
  origin/${BRANCH} und baut bei Bedarf neu. Schlaegt das fehl, startet die
  Fotobox mit dem zuletzt lauffaehigen Stand.

EOF
