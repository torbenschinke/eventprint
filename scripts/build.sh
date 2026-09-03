#!/usr/bin/env bash
#
# Baut alle Anwendungen und leitet das speclink-Dokument ab.
#
# Dies ist die vollständige Schleife für Entwicklung und CI. Sie ist bewusst
# gründlich und entsprechend langsam. Die Produktionsmaschine baut beim
# Hochfahren mit scripts/update.sh nur die Binärdateien; die Prüfung gehört
# dorthin, wo jemand auf sie reagieren kann.
#
# Reihenfolge ist vorgegeben und nicht verhandelbar:
#   go build  ->  go test | speclink evidence  ->  speclink verify  ->  generate
#
# Der Nachweis läuft vor der Prüfung, weil "verify" wissen will, welche Tests
# tatsächlich durchgelaufen sind. Umgekehrt prüfte es einen Nachweis von
# gestern.
#
# Umgebungsvariablen:
#   FACECROP=0   ohne Gesichtserkennung bauen (ohne OpenCV/gocv)
#   SKIP_TESTS=1 nur bauen, nichts prüfen; das Dokument entsteht dann nicht
#   TARGETS      Leerzeichenliste "os/arch", z. B. "linux/arm64 linux/amd64"

set -euo pipefail

readonly ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
readonly DIST_DIR="${ROOT_DIR}/dist"
readonly TOOLS_DIR="${ROOT_DIR}/.tools/bin"
readonly SPEC_DOC="${ROOT_DIR}/SPECIFICATION.md"

cd "${ROOT_DIR}"

# Die Anwendungen dieses Moduls. photobox braucht cgo und OpenCV, photoupld
# nicht – deshalb steht die Anforderung je Anwendung dabei.
readonly APPS=(
  "photobox:cgo"
  "photoupld:pure"
)

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m!!\033[0m %s\n' "$*" >&2; exit 1; }

build_tags=()
if [[ "${FACECROP:-1}" == "0" ]]; then
  build_tags+=("nofacecrop")
  warn "Gesichtserkennung wird nicht eingebaut (FACECROP=0)."
fi

tagflag=()
if [[ ${#build_tags[@]} -gt 0 ]]; then
  tagflag=(-tags "$(IFS=,; echo "${build_tags[*]}")")
fi

command -v go >/dev/null 2>&1 || die "go wurde nicht gefunden."

# ---------------------------------------------------------------- speclink ---
# Die Fassung steht in go.mod. Eine zweite Angabe hier wäre eine zweite
# Wahrheit, und die wäre irgendwann die falsche.
speclink_version() {
  go list -m -f '{{.Version}}' github.com/worldiety/speclink 2>/dev/null
}

ensure_speclink() {
  local want
  want="$(speclink_version)" || die "github.com/worldiety/speclink steht nicht in go.mod."
  [[ -n "${want}" ]] || die "Fassung von speclink nicht ermittelbar."

  local marker="${TOOLS_DIR}/.speclink-version"
  if [[ -x "${TOOLS_DIR}/speclink" && "$(cat "${marker}" 2>/dev/null || true)" == "${want}" ]]; then
    return
  fi

  log "speclink ${want} wird bereitgestellt"
  mkdir -p "${TOOLS_DIR}"
  GOBIN="${TOOLS_DIR}" go install "github.com/worldiety/speclink/cmd/speclink@${want}"
  printf '%s' "${want}" >"${marker}"
}

# ------------------------------------------------------------------ bauen ---
build_app() {
  local name="$1" kind="$2" goos="$3" goarch="$4"
  local out="${DIST_DIR}/${name}-${goos}-${goarch}"

  # Fremdarchitektur und cgo vertragen sich nur mit einem Cross-Compiler, den
  # hier niemand voraussetzen kann. Lieber ehrlich auslassen als eine Binärdatei
  # abliefern, die auf dem Zielgerät nicht startet.
  local cgo=0
  if [[ "${kind}" == "cgo" && "${FACECROP:-1}" != "0" ]]; then
    if [[ "${goos}/${goarch}" != "$(go env GOHOSTOS)/$(go env GOHOSTARCH)" ]]; then
      warn "${name} für ${goos}/${goarch} übersprungen: braucht cgo und OpenCV der Zielarchitektur."
      return 0
    fi
    cgo=1
  fi

  log "baue ${name} (${goos}/${goarch}, cgo=${cgo})"
  CGO_ENABLED="${cgo}" GOOS="${goos}" GOARCH="${goarch}" \
    go build -trimpath "${tagflag[@]+"${tagflag[@]}"}" -o "${out}" "./cmd/${name}"

  # Ein Name ohne Architektur macht die systemd-Unit unabhängig davon, auf
  # welchem Gerät gebaut wurde.
  if [[ "${goos}/${goarch}" == "$(go env GOHOSTOS)/$(go env GOHOSTARCH)" ]]; then
    ln -sf "$(basename "${out}")" "${DIST_DIR}/${name}"
  fi
}

mkdir -p "${DIST_DIR}"

log "go build ./..."
go build "${tagflag[@]+"${tagflag[@]}"}" ./...

targets="${TARGETS:-$(go env GOHOSTOS)/$(go env GOHOSTARCH)}"
for target in ${targets}; do
  goos="${target%%/*}"
  goarch="${target##*/}"

  for entry in "${APPS[@]}"; do
    build_app "${entry%%:*}" "${entry##*:}" "${goos}" "${goarch}"
  done
done

if [[ "${SKIP_TESTS:-0}" == "1" ]]; then
  warn "Tests und Spezifikation übersprungen (SKIP_TESTS=1)."
  log "fertig: ${DIST_DIR}"
  exit 0
fi

ensure_speclink
readonly SPECLINK="${TOOLS_DIR}/speclink"

log "go vet ./..."
go vet "${tagflag[@]+"${tagflag[@]}"}" ./...

# ------------------------------------------------------------- Nachweis ---
# go test liefert bei fehlgeschlagenen Tests einen Fehlercode. Der darf nicht
# in der Pipe verlorengehen, deshalb wird der Strom erst vollständig
# geschrieben und danach ausgewertet.
#
# -count=1 schaltet den Testcache ab. Der Nachweis ist die Aussage, dass ein
# Test in diesem Lauf durchgelaufen ist; ein Ergebnis aus dem Cache trägt die
# Zeilen nicht, die speclink dafür liest, und die Prüfung meldete dann
# wahrheitsgemäß, dass nichts die Anforderung gezeigt hat.
log "go test -json -count=1 ./..."
test_log="$(mktemp)"
trap 'rm -f "${test_log}"' EXIT

test_status=0
go test -json -count=1 "${tagflag[@]+"${tagflag[@]}"}" ./... >"${test_log}" || test_status=$?

if [[ ${test_status} -ne 0 ]]; then
  # Die lesbare Fassung nachreichen: Der JSON-Strom taugt für speclink, nicht
  # für einen Menschen um Mitternacht.
  go test -count=1 "${tagflag[@]+"${tagflag[@]}"}" ./... || true
  die "Tests fehlgeschlagen."
fi

log "speclink evidence"
"${SPECLINK}" evidence -in "${test_log}"

log "speclink verify"
"${SPECLINK}" verify -root "${ROOT_DIR}" ./...

log "speclink generate -> $(basename "${SPEC_DOC}")"
"${SPECLINK}" generate -root "${ROOT_DIR}" -title "eventprint" -out "${SPEC_DOC}" ./...

log "fertig"
printf '    Binärdateien: %s\n' "${DIST_DIR}"
printf '    Spezifikation: %s\n' "${SPEC_DOC}"
