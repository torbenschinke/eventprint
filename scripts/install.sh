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
  git ca-certificates curl        # Quelltext holen, TLS, Download des Go-Archivs
  build-essential pkg-config      # cgo braucht einen C-Übersetzer
  cups cups-client                # Druckdienst und lp/lpstat/lpadmin
  printer-driver-gutenprint       # Treiber und Backend für den CZ-01
  gphoto2                         # Tethering der Kamera
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
# Die benötigte Fassung steht in go.mod und nirgends sonst.
required_go="$(sed -n 's/^go \([0-9][0-9.]*\).*/\1/p' "${SCRIPT_DIR}/../go.mod" 2>/dev/null | head -1)"
required_go="${required_go:-1.27.0}"
GO_VERSION="${GO_VERSION:-${required_go}}"

version_ge() {
  # sort -V ist hier genau richtig: Es vergleicht 1.10 korrekt gegen 1.9.
  [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -1)" == "$2" ]]
}

installed_go=""
if command -v go >/dev/null 2>&1; then
  installed_go="$(go env GOVERSION 2>/dev/null | sed 's/^go//')"
elif [[ -x /usr/local/go/bin/go ]]; then
  installed_go="$(/usr/local/go/bin/go env GOVERSION 2>/dev/null | sed 's/^go//')"
fi

if [[ -n "${installed_go}" ]] && version_ge "${installed_go}" "${required_go}"; then
  log "Go ${installed_go} ist vorhanden und ausreichend"
else
  case "$(dpkg --print-architecture 2>/dev/null || echo unknown)" in
    arm64)  go_arch="arm64"  ;;
    armhf)  go_arch="armv6l" ;;
    amd64)  go_arch="amd64"  ;;
    *)      die "Unbekannte Architektur; Go bitte von Hand installieren (mindestens ${required_go})." ;;
  esac

  tarball="go${GO_VERSION}.linux-${go_arch}.tar.gz"
  log "Go ${GO_VERSION} (${go_arch}) wird installiert${installed_go:+, vorhanden war ${installed_go}}"

  run curl -fsSL --retry 3 -o "/tmp/${tarball}" "https://go.dev/dl/${tarball}"
  run rm -rf /usr/local/go
  run tar -C /usr/local -xzf "/tmp/${tarball}"
  run rm -f "/tmp/${tarball}"
  run install -m 0644 /dev/stdin /etc/profile.d/go.sh <<<'export PATH=$PATH:/usr/local/go/bin'
fi

export PATH="/usr/local/go/bin:${PATH}"

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
  "PATH=/usr/local/go/bin:/usr/bin:/bin"
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

# ----------------------------------------------------------------- systemd ---
log "Dienste einrichten"

if [[ ${DRY_RUN} -eq 1 ]]; then
  printf '   \033[2m[dry-run]\033[0m %s\n' "schreibe /etc/default/eventprint (FACECROP=${FACECROP}, EVENTPRINT_BRANCH=${BRANCH})"
else
  cat >/etc/default/eventprint <<EOF
# Umgebung der eventprint-Dienste. Wird von install.sh angelegt und darf von
# Hand geändert werden.

# Zweig, den die Aktualisierung beim Hochfahren verfolgt.
EVENTPRINT_BRANCH=${BRANCH}

# 0 baut ohne Gesichtserkennung, für Systeme mit zu alter OpenCV-Fassung.
FACECROP=${FACECROP}

# Adresse, auf der die Oberfläche lauscht.
HOST=0.0.0.0
EOF
fi

for unit in eventprint-update.service eventprint.service; do
  run install -m 0644 "${PREFIX}/deploy/systemd/${unit}" "/etc/systemd/system/${unit}"
done

run systemctl daemon-reload
run systemctl enable eventprint-update.service eventprint.service
run systemctl enable cups.service

log "fertig"
cat <<EOF

  Nächste Schritte:

    1. Drucker in CUPS einrichten – siehe DRUCKER.md:
         sudo /usr/lib/cups/backend/gutenprint53+usb      # URI ablesen
         sudo lpadmin -p CZ01 -v "<URI>" -m <ppd> -E

    2. Dienst starten:
         sudo systemctl start eventprint.service
         journalctl -u eventprint.service -f

    3. Oberfläche öffnen und als Betreuer anmelden:
         http://$(hostname -I 2>/dev/null | awk '{print $1}'):3000

  Beim nächsten Hochfahren holt eventprint-update.service den Stand von
  origin/${BRANCH} und baut bei Bedarf neu. Schlägt das fehl, startet die
  Fotobox mit dem zuletzt lauffähigen Stand.

EOF
