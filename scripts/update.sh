#!/usr/bin/env bash
#
# Holt beim Hochfahren den aktuellen Stand aus dem Git-Repository und baut die
# Anwendungen neu, falls sich etwas geändert hat.
#
# Zwei Regeln bestimmen alles Weitere:
#
#   1. Dieses Skript endet immer mit Erfolg. Es läuft vor dem Dienst; bräche es
#      ab, startete die Fotobox nicht. Kein Netz auf einer Feier ist der
#      Normalfall, kein Fehlerfall.
#
#   2. Die Maschine bleibt nie ohne lauffähige Binärdatei zurück. Gebaut wird
#      neben den laufenden Stand und erst nach Erfolg umgehängt.
#
# Absichtlich wird hier nur gebaut und nicht geprüft: Tests und speclink
# gehören in scripts/build.sh, wo jemand auf einen Befund reagieren kann.
# Auf dem Gerät stünde ein roter Lauf um 18 Uhr niemandem zur Verfügung, und
# die Feier begänne trotzdem.

set -uo pipefail

readonly ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
readonly DIST_DIR="${ROOT_DIR}/dist"
readonly MARKER="${DIST_DIR}/.built-revision"
readonly BRANCH="${EVENTPRINT_BRANCH:-master}"

cd "${ROOT_DIR}" || exit 0

log() { printf '%s eventprint-update: %s\n' "$(date -Is)" "$*"; }

# Die Anwendungen, die auf dem Gerät gebraucht werden. Der Upload-Dienst läuft
# woanders; hier entsteht nur die Fotobox.
readonly APPS=("photobox")

binaries_present() {
  local app
  for app in "${APPS[@]}"; do
    [[ -x "${DIST_DIR}/${app}" ]] || return 1
  done
  return 0
}

if ! command -v go >/dev/null 2>&1; then
  log "go nicht gefunden, überspringe die Aktualisierung"
  exit 0
fi

if ! command -v git >/dev/null 2>&1 || [[ ! -d .git ]]; then
  log "kein Git-Arbeitsverzeichnis, überspringe die Aktualisierung"
  exit 0
fi

current="$(git rev-parse HEAD 2>/dev/null)" || {
  log "HEAD nicht lesbar, überspringe die Aktualisierung"
  exit 0
}

target="${current}"

# Ein kurzer Zeitrahmen: Hängt das Netz, soll die Fotobox nicht minutenlang im
# Startvorgang stehen. Der alte Stand ist ein vollständig brauchbarer Stand.
if timeout 60 git fetch --quiet origin "${BRANCH}" 2>/dev/null; then
  if remote="$(git rev-parse "origin/${BRANCH}" 2>/dev/null)"; then
    target="${remote}"
  fi
else
  log "kein Zugriff auf origin/${BRANCH}, bleibe bei ${current:0:12}"
fi

built="$(cat "${MARKER}" 2>/dev/null || true)"

if [[ "${target}" == "${built}" ]] && binaries_present; then
  log "bereits aktuell (${target:0:12})"
  exit 0
fi

if [[ "${target}" != "${current}" ]]; then
  log "aktualisiere ${current:0:12} -> ${target:0:12}"

  # Das Gerät ist ein Abbild des Repositories, keine Arbeitskopie. Lokale
  # Änderungen daran wären ein Versehen, und ein Versehen soll den nächsten
  # Start nicht blockieren.
  if ! git reset --hard --quiet "${target}"; then
    log "konnte nicht auf ${target:0:12} setzen, baue den vorhandenen Stand"
    target="${current}"
  fi
fi

# Ohne Gesichtserkennung bauen, wenn OpenCV auf diesem Gerät nicht taugt. Die
# Entscheidung fällt bei der Installation und steht in der Umgebung der Unit.
tagflag=()
if [[ "${FACECROP:-1}" == "0" ]]; then
  tagflag=(-tags nofacecrop)
fi

staging="$(mktemp -d "${DIST_DIR}/.staging-XXXXXX" 2>/dev/null)" || {
  log "kein Platz für den Zwischenbau, behalte den laufenden Stand"
  exit 0
}
trap 'rm -rf "${staging}"' EXIT

failed=0
for app in "${APPS[@]}"; do
  log "baue ${app}"

  if ! CGO_ENABLED=1 go build -trimpath "${tagflag[@]+"${tagflag[@]}"}" \
      -o "${staging}/${app}" "./cmd/${app}" 2>&1; then
    log "Bau von ${app} fehlgeschlagen"
    failed=1
    break
  fi
done

if [[ ${failed} -ne 0 ]]; then
  if binaries_present; then
    log "behalte den zuletzt lauffähigen Stand (${built:0:12})"
  else
    log "ACHTUNG: es gibt keine lauffähige Binärdatei; der Dienst wird nicht starten"
  fi
  exit 0
fi

# Umhängen erst jetzt, und je Datei atomar.
for app in "${APPS[@]}"; do
  if ! mv -f "${staging}/${app}" "${DIST_DIR}/${app}"; then
    log "konnte ${app} nicht übernehmen"
    exit 0
  fi
done

printf '%s' "${target}" >"${MARKER}"
log "fertig auf ${target:0:12}"
exit 0
