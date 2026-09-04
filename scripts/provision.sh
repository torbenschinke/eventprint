#!/usr/bin/env bash
#
# Gleicht die Systemkonfiguration der Box an das Repository an.
#
# Der Grund, warum es diese Datei gibt: update.sh laeuft unprivilegiert und
# kann ausschliesslich Go-Code ausspielen. Jede Aenderung an udev, systemd oder
# Paketen erforderte deshalb, dass jemand mit root vor der Box sitzt - und beim
# ersten Einsatz hiess das eine Autofahrt, weil die Box im Gastnetz hinter NAT
# stand und nicht erreichbar war.
#
# Dieses Skript laeuft als root beim Hochfahren, vor dem Updater. Damit ist
# Systemkonfiguration per "git push" zustellbar: pushen, jemanden vor Ort den
# Strom ziehen lassen, fertig.
#
# Zwei Regeln wie beim Updater:
#
#   1. Es endet immer mit Erfolg. Es laeuft vor dem Dienst; braeche es ab,
#      startete die Fotobox nicht.
#   2. Es ist wiederholbar und aendert nichts, was schon stimmt.
#
# ACHTUNG: Wer auf diesen Zweig pushen kann, fuehrt hier Befehle als root aus.
# Das ist bewusst so entschieden, damit eine Reparatur aus der Ferne moeglich
# ist, und es setzt voraus, dass der Zugang zum Repository entsprechend eng
# bleibt.

set -uo pipefail

log() { printf '%s eventprint-provision: %s\n' "$(date -Is)" "$*"; }

if [[ "${EUID}" -ne 0 ]]; then
  log "laeuft nicht als root, es gibt nichts zu tun"
  exit 0
fi

changed=0

# ------------------------------------------------------------------ Journal ---
# Nach der ersten Veranstaltung war das Journal des Abends verloren. Raspberry
# Pi OS liefert 40-rpi-volatile-storage.conf aus und haelt die Logs im RAM, um
# die SD-Karte zu schonen. Damit stirbt beim Neustart genau die Auskunft, die
# man braucht, um einen Ausfall zu verstehen.
#
# Die 99 im Namen ist wesentlich: Drop-ins werden lexikalisch gelesen, eine 10
# vorne haette keine Wirkung gegen die 40 der Distribution.
journal_conf=/etc/systemd/journald.conf.d/99-eventprint-persistent.conf
if [[ ! -f "${journal_conf}" ]]; then
  log "Journal auf persistent stellen"
  mkdir -p /etc/systemd/journald.conf.d
  cat >"${journal_conf}" <<'CONF'
# Von eventprint gesetzt. Siehe scripts/provision.sh.
#
# Raspberry Pi OS haelt das Journal im RAM (40-rpi-volatile-storage.conf).
# Auf einem Geraet ohne Fernzugang sind Logs, die einen Neustart nicht
# ueberleben, wertlos: Nach der ersten Veranstaltung war nicht mehr
# feststellbar, warum Aufnahmen fehlten.
[Journal]
Storage=persistent
SystemMaxUse=500M
CONF
  systemctl restart systemd-journald && journalctl --flush
  changed=1
fi

# ------------------------------------------------------------------- Kamera ---
# Der Volume-Monitor von GVFS greift jede PTP-Kamera ab, sobald sie am Bus
# auftaucht, und haelt sie fest, damit man sie als Laufwerk durchblaettern
# kann. gphoto2 kommt dann nicht mehr an das Geraet heran oder verliert es
# mitten im Betrieb. Beides trifft die Fotobox an der teuersten Stelle:
# Waehrend das Tethering neu aufgebaut wird, lauscht niemand am USB, und eine
# in dieser Zeit ausgeloeste Aufnahme bleibt auf der Speicherkarte liegen.
#
# Auf einem Geraet, das den Abend ueber Fotos druckt, wird die Kamera nie als
# Laufwerk gebraucht.
for unit in gvfs-gphoto2-volume-monitor gphoto2-volume-monitor; do
  if [[ -e "/usr/libexec/${unit}" || -e "/usr/lib/gvfs/${unit}" ]]; then
    # Das Ergebnis wird in eine Variable geholt statt durch eine Pipe geprueft:
    # is-enabled liefert fuer maskierte Units einen Fehlercode, und mit
    # pipefail scheitert dann die ganze Pipeline, obwohl grep getroffen hat.
    # Die Bedingung war dadurch immer wahr und der Block lief bei jedem Start
    # erneut - genau das, was ein wiederholbares Skript nicht tun darf.
    state="$(systemctl --global is-enabled "${unit}.service" 2>/dev/null || true)"
    if [[ "${state}" != *masked* ]]; then
      log "${unit} abschalten, damit gphoto2 an die Kamera kommt"
      systemctl --global mask "${unit}.service" 2>/dev/null && changed=1
    fi
  fi
done

# Der D-Bus-Dienst startet den Monitor auch ohne systemd-Unit nach. Ihn
# stillzulegen ist der eigentlich wirksame Griff.
dbus_service=/usr/share/dbus-1/services/org.gtk.vfs.GPhoto2VolumeMonitor.service
dbus_override=/usr/local/share/dbus-1/services/org.gtk.vfs.GPhoto2VolumeMonitor.service
if [[ -f "${dbus_service}" && ! -f "${dbus_override}" ]]; then
  log "D-Bus-Aktivierung des GPhoto2-Volume-Monitors stilllegen"
  mkdir -p "$(dirname "${dbus_override}")"
  # /usr/local/share hat Vorrang vor /usr/share. Die Datei der Distribution
  # bleibt unberuehrt, ein Paketupdate ueberschreibt hier also nichts.
  cat >"${dbus_override}" <<'CONF'
[D-BUS Service]
Name=org.gtk.vfs.GPhoto2VolumeMonitor
Exec=/bin/false
CONF
  changed=1
fi

# udev: Ohne Regel gehoert der USB-Knoten der Kamera root, und der Dienst
# laeuft als eventprint. uaccess vergibt die ACL an die aktive lokale Sitzung -
# das ist der Kiosk-Nutzer, nicht der Dienst. Deshalb zusaetzlich plugdev.
udev_rule=/etc/udev/rules.d/70-eventprint-camera.rules
udev_want=$(cat <<'RULE'
# Von eventprint verwaltet. Siehe scripts/provision.sh.
#
# Gibt PTP-Kameras dem Dienstnutzer frei und haelt die Automatik des Desktops
# davon fern. Ohne die Freigabe scheitert gphoto2 mit "Could not claim the USB
# device", ohne UDISKS_IGNORE greift der Datei-Manager zu.
SUBSYSTEM=="usb", ENV{ID_GPHOTO2}=="1", MODE="0664", GROUP="plugdev", TAG+="uaccess", ENV{UDISKS_IGNORE}="1", ENV{MTP_NO_PROBE}="1"
SUBSYSTEM=="usb", ENV{ID_USB_INTERFACES}=="*:060101:*", MODE="0664", GROUP="plugdev", TAG+="uaccess", ENV{UDISKS_IGNORE}="1", ENV{MTP_NO_PROBE}="1"
RULE
)
if [[ "$(cat "${udev_rule}" 2>/dev/null)" != "${udev_want}" ]]; then
  log "udev-Regel fuer die Kamera schreiben"
  printf '%s\n' "${udev_want}" >"${udev_rule}"
  udevadm control --reload-rules
  changed=1
fi

# USB-Autosuspend: Der Kernel legt Geraete nach zwei Sekunden Leerlauf schlafen.
# Eine Kamera im Tethering ist die meiste Zeit im Leerlauf - sie wartet auf den
# Ausloeser. Schlaeft sie ein, bricht PTP, und genau das sieht aus wie "nach
# ein paar Fotos geht nichts mehr".
autosuspend_rule=/etc/udev/rules.d/71-eventprint-usb-nosuspend.rules
autosuspend_want=$(cat <<'RULE'
# Von eventprint verwaltet. Siehe scripts/provision.sh.
#
# Haelt PTP-Kameras wach. Der Kernel setzt usbcore.autosuspend auf zwei
# Sekunden; eine Kamera im Tethering wartet aber die meiste Zeit auf den
# Ausloeser und gilt damit als untaetig. Schlaeft sie ein, bricht die
# PTP-Sitzung.
SUBSYSTEM=="usb", ENV{ID_GPHOTO2}=="1", TEST=="power/control", ATTR{power/control}="on"
SUBSYSTEM=="usb", ENV{ID_USB_INTERFACES}=="*:060101:*", TEST=="power/control", ATTR{power/control}="on"
RULE
)
if [[ "$(cat "${autosuspend_rule}" 2>/dev/null)" != "${autosuspend_want}" ]]; then
  log "USB-Autosuspend fuer Kameras abschalten"
  printf '%s\n' "${autosuspend_want}" >"${autosuspend_rule}"
  udevadm control --reload-rules
  changed=1
fi

if [[ ${changed} -eq 0 ]]; then
  log "nichts zu tun, die Box stimmt bereits"
else
  log "fertig"
fi

exit 0
