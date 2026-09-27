# eventprint

Derived from the source by `speclink generate`. Do not edit: every sentence here is written somewhere else, and the point of this file is that there is only one such place.

## How to read this

This document is derived from the source. Nothing in it was written by hand, and nothing in it can be edited into agreement with something that is not true of the code — which is the property that makes it worth reading, and the reason it is regenerated rather than maintained.

It is written for four readers at once. Each chapter below names the ones it is for.

| If you are | Start at | Because it answers |
|---|---|---|
| **running the project** | Where it stands, Gaps, The register | how much is agreed, built, tested and signed, and what is left |
| **auditing it** | The register, Standards, Source documents, Requirements | what is claimed, what evidence stands behind each claim, and what was never measured |
| **the one who asked for it** | Courses of business, The material, Requirements | whether the sentences you wrote survived into the thing that was built |
| **building on it** | The boundary, What answers from outside, How it is put together | what the system exposes, what it talks to, and the rules the code is held to |

### What a blank means

A chapter with nothing in it says which of two things happened. _Not declared_ means the project states nothing of that kind, and the emptiness is a fact about the project. _Not measured_ means this run could not look, and the emptiness is a fact about the run. They are never printed as the same blank, because a reader has no other way to tell them apart, and treating an unasked question as a clean answer is the failure this whole tool exists to prevent.

## Where it stands

|  | measured | complete |
|---|---:|---:|
| Source segments accounted for | 40 | 100% |
| Normative requirements covered | 39 | 100% |
| … claimed by a test | 39 | 100% |
| … demonstrated by a run | 39 | 100% |
| … read by a person | 39 | 0% |

## Gaps

### Requirements no test claims

- R-DEC-ZUSTANDSABLAGE — _accepted:_ Die Entscheidung gegen eine Ereignisfolge ist die Abwesenheit einer Sache; ein Test kann sie nicht zeigen.

### Requirements nobody has read

- R-ARCHIV-EXPORT
- R-ARCHIV-LOESCHEN
- R-ARCHIV-PLATZ
- R-DEC-ZUSTANDSABLAGE
- R-DRUCK-ABBRUCH
- R-DRUCK-AUFTRAG
- R-DRUCK-DIAGNOSE
- R-DRUCK-FREIGABE
- R-DRUCK-GESTALTUNG
- R-DRUCK-KEIN-NACHDRUCK
- R-DRUCK-KIOSK
- R-DRUCK-PAPIER
- R-DRUCK-STATUS
- R-DRUCK-VORSCHAU
- R-DRUCK-WIEDERHOLUNG
- R-FOTO-DRUCKVORLAGE
- R-FOTO-EINGANG
- R-FOTO-EINZELBILD
- R-FOTO-HISTORIE
- R-FOTO-IMPORT
- R-FOTO-LOESCHEN
- R-MODUS-ANZEIGE
- R-MODUS-BETREUUNG
- R-MODUS-EINSTELLUNGEN
- R-MODUS-HEIM
- R-MODUS-KIOSK
- R-MODUS-PRIVAT
- R-NETZ-BETREUUNG
- R-NETZ-SUCHE
- R-NETZ-VERBINDEN
- R-NETZ-ZUSTAND
- R-QUELLEN-NAS
- R-QUELLEN-USB
- R-UPLOAD-ABHOLUNG
- R-UPLOAD-BESTAETIGUNG
- R-UPLOAD-BILD
- R-UPLOAD-EINGANG
- R-UPLOAD-KOPPLUNG
- R-UPLOAD-SITZUNG

## What has actually been run

A test that claims a requirement is a claim. Evidence that the test ran is something else, and this chapter keeps them apart.

|  | count | of normative |
|---|---:|---:|
| Normative requirements | 39 |  |
| … a test claims | 39 | 100% |
| … a run demonstrated | 39 | 100% |

### How much of the code a run went through

_no coverage profile has been handed to speclink evidence, so nothing is known about which code ran_

## The material

| Document | Kind | Segments | Cited | Read | Drifted |
|---|---|---:|---:|---:|---:|
| `requirements/_sources/archiv.md` | markdown | 3 | 3 | 0 | 0 |
| `requirements/_sources/druck.md` | markdown | 11 | 11 | 0 | 0 |
| `requirements/_sources/entscheidungen.md` | markdown | 2 | 2 | 0 | 0 |
| `requirements/_sources/foto.md` | markdown | 6 | 6 | 0 | 0 |
| `requirements/_sources/modus.md` | markdown | 6 | 6 | 0 | 0 |
| `requirements/_sources/netz.md` | markdown | 4 | 4 | 0 | 0 |
| `requirements/_sources/quellen.md` | markdown | 2 | 2 | 0 | 0 |
| `requirements/_sources/upload.md` | markdown | 6 | 6 | 0 | 0 |

## Themes

_No theme is declared, so the requirements are not grouped._

## Standards

_No standard is declared, so no external clause is answered here._

## What gets built

This module builds 2 programs.

### gift-app

Command gift-app ist die Oberfläche des Fotodruckers auf dem Touchscreen.

Built from `cmd/gift-app`.

**Assembles** `device`

**Appears to accept** — inferred from the code rather than declared, so treat it as a starting point and not as a contract.

Flags: `-size`, `-window`

### photoupld

Command photoupld exposes the public upload relay for a private photobox. #\[go.permission.generateTable\]

Built from `cmd/photoupld`.

**Assembles** `photoupld`

_How this program is invoked could not be read from the source. That is a limit of the reading, not a statement that it takes no arguments._

## How it is put together

Every rule below is enforced by `speclink verify`. None of it is advice: a violation is a finding with the same identifier printed here, so a rule a reader finds in this chapter is one the build already refuses to ignore.

Convention: `ddd1`.

### Where code lives

| Layer | Path | What belongs there |
|---|---|---|
| **Bounded contexts** | `app/<context>` | One context per part of the business. What a context knows is its own; nothing reaches across. |
| **Entry points** | `cmd/<binary>` | Where the program is assembled. The only place allowed to choose which adapter is used. |
| **Infrastructure** | `pkg/, foundation/` | Technical helpers that know nothing about the business, and may not learn. Shared by every context, which is why they must stay ignorant of all of them. |

### What the code is held to

**The module has a main package.** (`K8-MAIN-EXISTS`)

A module with no entry point is a library, and every statement in this document about what the system does would be about something that never runs.

**Every main package lives under cmd/.** (`K8-MAIN-LOCATION`)

The place a program is assembled is the place its dependencies are chosen. Scattering that makes the wiring impossible to find and impossible to review.

**pkg/ and foundation/ hold no business knowledge and declare no use case.** (`K7-INFRA-DOMAIN-FREE`)

Infrastructure is shared by every context. The moment it knows about one, every other context inherits that knowledge and the contexts stop being separate.

**A context does not import its own user interface.** (`K6-CTX-NO-UI-IMPORT`)

The direction of that dependency is the whole point of the separation: the interface is built on the rules, and rules that reach back into a screen cannot be reused behind a second one.

**A use case is declared in a file of its own, named after it.** (`K5-UC-FILE`)

It is the unit this document is organised around and the unit a reviewer is asked to read. A file holding three of them cannot be reviewed as any of them.

**A use case checks a permission before it does anything.** (`K5-UC-AUTHZ`)

Authorisation placed anywhere else is authorisation that a second caller can skip.

**The framework's generic create-read-update-delete factories are not used.** (`K4-NO-GENERIC-CRUD`)

A screen generated from a type is a screen with no use case behind it, and nothing this document could trace a requirement to.

## How the code is composed

31 packages in 11 bounded contexts, and 71 dependencies between them. Only this module's own packages: a dependency on the standard library or on a third party is not a fact about the shape of this system.

8 packages declare this specification rather than the system — the requirements, the courses of business, the boundary. They are left out of the drawing below: in a project that uses this tool properly they are most of the nodes and most of the arrows, and the architecture disappears underneath its own documentation.

_No diagram is included in this document. Pass -figures to speclink generate, after rendering the sources written by speclink diagrams._

### Where one context reaches into another

26 dependencies cross from one context into another. Each is a place the two are no longer independent, and each is worth a reason.

| From | To |
|---|---|
| `app/device` | `app/nas` |
| `app/device` | `app/photo` |
| `app/device` | `app/printing` |
| `app/device/cfg` | `app/camera` |
| `app/device/cfg` | `app/nas` |
| `app/device/cfg` | `app/photo` |
| `app/device/cfg` | `app/printing` |
| `app/device/cfg` | `app/relay` |
| `app/device/cfg` | `app/usb` |
| `app/device/cfg` | `app/wifi` |
| `app/device/ui` | `app/camera` |
| `app/device/ui` | `app/nas` |
| `app/device/ui` | `app/photo` |
| `app/device/ui` | `app/printing` |
| `app/device/ui` | `app/relay` |
| `app/device/ui` | `app/usb` |
| `app/device/ui` | `app/wifi` |
| `app/photoupld/cfg` | `app/pairing` |
| `app/photoupld/cfg` | `app/upld` |
| `app/photoupld/ui` | `app/pairing` |
| `app/photoupld/ui` | `app/printing` |
| `app/photoupld/ui` | `app/upld` |
| `app/photoupld/ui/preview` | `app/printing` |
| `app/printing` | `app/photo` |
| `app/relay` | `app/printing` |
| `app/upld` | `app/printing` |

## What the code declares

113 constructs, each recognised by what it is rather than by an annotation saying so. Everything elsewhere in this document that names one of them points here.

### app/device

<a id="req-code-github-com-torbenschinke-eventprint-app-device-consumepaper"></a>
#### ConsumePaper

_use case_ — `app/device/uc_consume_paper.go:13`

**Answers to** [R-DRUCK-PAPIER](#req-R-DRUCK-PAPIER)

<a id="req-code-github-com-torbenschinke-eventprint-app-device-currentkiosk"></a>
#### CurrentKiosk

_query_ — `app/device/uc_current_kiosk.go:7`

**Answers to** [R-MODUS-HEIM](#req-R-MODUS-HEIM)

<a id="req-code-github-com-torbenschinke-eventprint-app-device-intake"></a>
#### Intake

_query_ — `app/device/uc_intake.go:38`

**Answers to** [R-DRUCK-KIOSK](#req-R-DRUCK-KIOSK), [R-FOTO-EINGANG](#req-R-FOTO-EINGANG)

<a id="req-code-github-com-torbenschinke-eventprint-app-device-loadsettings"></a>
#### LoadSettings

_query_ — `app/device/uc_load_settings.go:6`

**Answers to** [R-MODUS-EINSTELLUNGEN](#req-R-MODUS-EINSTELLUNGEN)

<a id="req-code-github-com-torbenschinke-eventprint-app-device-preflight"></a>
#### Preflight

_query_ — `app/device/uc_preflight.go:50`

**Answers to** [R-MODUS-KIOSK](#req-R-MODUS-KIOSK)

<a id="req-code-github-com-torbenschinke-eventprint-app-device-refillpaper"></a>
#### RefillPaper

_query_ — `app/device/uc_refill_paper.go:10`

**Answers to** [R-DRUCK-PAPIER](#req-R-DRUCK-PAPIER)

<a id="req-code-github-com-torbenschinke-eventprint-app-device-savesettings"></a>
#### SaveSettings

_query_ — `app/device/uc_save_settings.go:17`

**Answers to** [R-MODUS-EINSTELLUNGEN](#req-R-MODUS-EINSTELLUNGEN)

<a id="req-code-github-com-torbenschinke-eventprint-app-device-setpin"></a>
#### SetPin

_use case_ — `app/device/uc_set_pin.go:13`

**Answers to** [R-MODUS-BETREUUNG](#req-R-MODUS-BETREUUNG)

<a id="req-code-github-com-torbenschinke-eventprint-app-device-startkiosk"></a>
#### StartKiosk

_query_ — `app/device/uc_start_kiosk.go:25`

**Answers to** [R-MODUS-KIOSK](#req-R-MODUS-KIOSK)

<a id="req-code-github-com-torbenschinke-eventprint-app-device-stopkiosk"></a>
#### StopKiosk

_use case_ — `app/device/uc_stop_kiosk.go:9`

**Answers to** [R-MODUS-BETREUUNG](#req-R-MODUS-BETREUUNG)

<a id="req-code-github-com-torbenschinke-eventprint-app-device-unlock"></a>
#### Unlock

_use case_ — `app/device/uc_unlock.go:10`

**Answers to** [R-MODUS-BETREUUNG](#req-R-MODUS-BETREUUNG)

<a id="req-code-de-torbenschinke-eventprint-device-consume-paper"></a>
#### de.torbenschinke.eventprint.device.consume\_paper

_permission_ — `app/device/perm.go:87`

<a id="req-code-de-torbenschinke-eventprint-device-current-kiosk"></a>
#### de.torbenschinke.eventprint.device.current\_kiosk

_permission_ — `app/device/perm.go:59`

<a id="req-code-de-torbenschinke-eventprint-device-intake"></a>
#### de.torbenschinke.eventprint.device.intake

_permission_ — `app/device/perm.go:95`

<a id="req-code-de-torbenschinke-eventprint-device-load-settings"></a>
#### de.torbenschinke.eventprint.device.load\_settings

_permission_ — `app/device/perm.go:24`

<a id="req-code-de-torbenschinke-eventprint-device-preflight"></a>
#### de.torbenschinke.eventprint.device.preflight

_permission_ — `app/device/perm.go:73`

<a id="req-code-de-torbenschinke-eventprint-device-refill-paper"></a>
#### de.torbenschinke.eventprint.device.refill\_paper

_permission_ — `app/device/perm.go:80`

<a id="req-code-de-torbenschinke-eventprint-device-save-settings"></a>
#### de.torbenschinke.eventprint.device.save\_settings

_permission_ — `app/device/perm.go:31`

<a id="req-code-de-torbenschinke-eventprint-device-set-pin"></a>
#### de.torbenschinke.eventprint.device.set\_pin

_permission_ — `app/device/perm.go:38`

<a id="req-code-de-torbenschinke-eventprint-device-start-kiosk"></a>
#### de.torbenschinke.eventprint.device.start\_kiosk

_permission_ — `app/device/perm.go:45`

<a id="req-code-de-torbenschinke-eventprint-device-stop-kiosk"></a>
#### de.torbenschinke.eventprint.device.stop\_kiosk

_permission_ — `app/device/perm.go:52`

<a id="req-code-de-torbenschinke-eventprint-device-unlock"></a>
#### de.torbenschinke.eventprint.device.unlock

_permission_ — `app/device/perm.go:66`

### app/nas

<a id="req-code-github-com-torbenschinke-eventprint-app-nas-browse"></a>
#### Browse

_query_ — `app/nas/uc_browse.go:17`

**Answers to** [R-QUELLEN-NAS](#req-R-QUELLEN-NAS)

<a id="req-code-github-com-torbenschinke-eventprint-app-nas-read"></a>
#### Read

_query_ — `app/nas/uc_read.go:12`

**Answers to** [R-QUELLEN-NAS](#req-R-QUELLEN-NAS)

<a id="req-code-github-com-torbenschinke-eventprint-app-nas-shares"></a>
#### Shares

_query_ — `app/nas/uc_shares.go:17`

**Answers to** [R-QUELLEN-NAS](#req-R-QUELLEN-NAS)

<a id="req-code-github-com-torbenschinke-eventprint-app-nas-thumbnail"></a>
#### Thumbnail

_query_ — `app/nas/uc_thumbnail.go:19`

**Answers to** [R-QUELLEN-NAS](#req-R-QUELLEN-NAS)

<a id="req-code-de-torbenschinke-eventprint-nas-browse"></a>
#### de.torbenschinke.eventprint.nas.browse

_permission_ — `app/nas/perm.go:27`

<a id="req-code-de-torbenschinke-eventprint-nas-read"></a>
#### de.torbenschinke.eventprint.nas.read

_permission_ — `app/nas/perm.go:41`

<a id="req-code-de-torbenschinke-eventprint-nas-shares"></a>
#### de.torbenschinke.eventprint.nas.shares

_permission_ — `app/nas/perm.go:20`

<a id="req-code-de-torbenschinke-eventprint-nas-thumbnail"></a>
#### de.torbenschinke.eventprint.nas.thumbnail

_permission_ — `app/nas/perm.go:34`

### app/pairing

<a id="req-code-github-com-torbenschinke-eventprint-app-pairing-confirmpairing"></a>
#### ConfirmPairing

_query_ — `app/pairing/uc_confirm_pairing.go:19`

**Answers to** [R-UPLOAD-KOPPLUNG](#req-R-UPLOAD-KOPPLUNG)

<a id="req-code-github-com-torbenschinke-eventprint-app-pairing-findmyboxes"></a>
#### FindMyBoxes

_query_ — `app/pairing/uc_find_my_boxes.go:11`

**Answers to** [R-UPLOAD-KOPPLUNG](#req-R-UPLOAD-KOPPLUNG)

<a id="req-code-github-com-torbenschinke-eventprint-app-pairing-requestpairing"></a>
#### RequestPairing

_query_ — `app/pairing/uc_request_pairing.go:25`

**Answers to** [R-UPLOAD-KOPPLUNG](#req-R-UPLOAD-KOPPLUNG)

<a id="req-code-github-com-torbenschinke-eventprint-app-pairing-unpairbox"></a>
#### UnpairBox

_use case_ — `app/pairing/uc_unpair_box.go:13`

**Answers to** [R-UPLOAD-KOPPLUNG](#req-R-UPLOAD-KOPPLUNG)

<a id="req-code-de-torbenschinke-photoupld-pairing-confirm"></a>
#### de.torbenschinke.photoupld.pairing.confirm

_permission_ — `app/pairing/perm.go:24`

<a id="req-code-de-torbenschinke-photoupld-pairing-find-my-boxes"></a>
#### de.torbenschinke.photoupld.pairing.find\_my\_boxes

_permission_ — `app/pairing/perm.go:31`

<a id="req-code-de-torbenschinke-photoupld-pairing-request"></a>
#### de.torbenschinke.photoupld.pairing.request

_permission_ — `app/pairing/perm.go:17`

<a id="req-code-de-torbenschinke-photoupld-pairing-unpair-box"></a>
#### de.torbenschinke.photoupld.pairing.unpair\_box

_permission_ — `app/pairing/perm.go:38`

<a id="req-code-github-com-torbenschinke-eventprint-app-pairing-box"></a>
#### Box

_aggregate_ — `app/pairing/model.go:23`

**Answers to** [R-DEC-ZUSTANDSABLAGE](#req-R-DEC-ZUSTANDSABLAGE)

### app/photo

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-delete"></a>
#### Delete

_use case_ — `app/photo/uc_delete.go:14`

**Answers to** [R-FOTO-LOESCHEN](#req-R-FOTO-LOESCHEN)

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-findall"></a>
#### FindAll

_query_ — `app/photo/uc_find_all.go:14`

**Answers to** [R-FOTO-HISTORIE](#req-R-FOTO-HISTORIE)

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-findbyid"></a>
#### FindByID

_query_ — `app/photo/uc_find_by_id.go:10`

**Answers to** [R-FOTO-EINZELBILD](#req-R-FOTO-EINZELBILD), [R-MODUS-PRIVAT](#req-R-MODUS-PRIVAT)

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-findevent"></a>
#### FindEvent

_query_ — `app/photo/uc_find_event.go:14`

**Answers to** [R-MODUS-PRIVAT](#req-R-MODUS-PRIVAT)

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-import"></a>
#### Import

_query_ — `app/photo/uc_import.go:48`

**Answers to** [R-FOTO-IMPORT](#req-R-FOTO-IMPORT)

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-inspectstorage"></a>
#### InspectStorage

_query_ — `app/photo/uc_inspect_storage.go:11`

**Answers to** [R-ARCHIV-PLATZ](#req-R-ARCHIV-PLATZ)

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-locate"></a>
#### Locate

_query_ — `app/photo/uc_locate.go:23`

**Answers to** [R-ARCHIV-EXPORT](#req-R-ARCHIV-EXPORT), [R-FOTO-DRUCKVORLAGE](#req-R-FOTO-DRUCKVORLAGE), [R-MODUS-PRIVAT](#req-R-MODUS-PRIVAT)

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-markprinted"></a>
#### MarkPrinted

_use case_ — `app/photo/uc_mark_printed.go:10`

**Answers to** [R-FOTO-HISTORIE](#req-R-FOTO-HISTORIE)

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-markseen"></a>
#### MarkSeen

_use case_ — `app/photo/uc_mark_seen.go:14`

**Answers to** [R-FOTO-EINGANG](#req-R-FOTO-EINGANG)

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-openoriginal"></a>
#### OpenOriginal

_query_ — `app/photo/uc_open_original.go:16`

**Answers to** [R-FOTO-DRUCKVORLAGE](#req-R-FOTO-DRUCKVORLAGE)

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-purgeevent"></a>
#### PurgeEvent

_query_ — `app/photo/uc_purge_event.go:16`

**Answers to** [R-ARCHIV-LOESCHEN](#req-R-ARCHIV-LOESCHEN)

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-setfavorite"></a>
#### SetFavorite

_use case_ — `app/photo/uc_set_favorite.go:10`

**Answers to** [R-FOTO-HISTORIE](#req-R-FOTO-HISTORIE)

<a id="req-code-de-torbenschinke-eventprint-photo-delete"></a>
#### de.torbenschinke.eventprint.photo.delete

_permission_ — `app/photo/perm.go:55`

<a id="req-code-de-torbenschinke-eventprint-photo-find-all"></a>
#### de.torbenschinke.eventprint.photo.find\_all

_permission_ — `app/photo/perm.go:34`

<a id="req-code-de-torbenschinke-eventprint-photo-find-by-id"></a>
#### de.torbenschinke.eventprint.photo.find\_by\_id

_permission_ — `app/photo/perm.go:48`

<a id="req-code-de-torbenschinke-eventprint-photo-find-event"></a>
#### de.torbenschinke.eventprint.photo.find\_event

_permission_ — `app/photo/perm.go:41`

<a id="req-code-de-torbenschinke-eventprint-photo-import"></a>
#### de.torbenschinke.eventprint.photo.import

_permission_ — `app/photo/perm.go:27`

<a id="req-code-de-torbenschinke-eventprint-photo-inspect-storage"></a>
#### de.torbenschinke.eventprint.photo.inspect\_storage

_permission_ — `app/photo/perm.go:97`

<a id="req-code-de-torbenschinke-eventprint-photo-locate"></a>
#### de.torbenschinke.eventprint.photo.locate

_permission_ — `app/photo/perm.go:90`

<a id="req-code-de-torbenschinke-eventprint-photo-mark-printed"></a>
#### de.torbenschinke.eventprint.photo.mark\_printed

_permission_ — `app/photo/perm.go:76`

<a id="req-code-de-torbenschinke-eventprint-photo-mark-seen"></a>
#### de.torbenschinke.eventprint.photo.mark\_seen

_permission_ — `app/photo/perm.go:69`

<a id="req-code-de-torbenschinke-eventprint-photo-open-original"></a>
#### de.torbenschinke.eventprint.photo.open\_original

_permission_ — `app/photo/perm.go:83`

<a id="req-code-de-torbenschinke-eventprint-photo-purge-event"></a>
#### de.torbenschinke.eventprint.photo.purge\_event

_permission_ — `app/photo/perm.go:104`

<a id="req-code-de-torbenschinke-eventprint-photo-set-favorite"></a>
#### de.torbenschinke.eventprint.photo.set\_favorite

_permission_ — `app/photo/perm.go:62`

<a id="req-code-github-com-torbenschinke-eventprint-app-photo-photo"></a>
#### Photo

_aggregate_ — `app/photo/model.go:116`

**Answers to** [R-DEC-ZUSTANDSABLAGE](#req-R-DEC-ZUSTANDSABLAGE)

### app/printing

<a id="req-code-github-com-torbenschinke-eventprint-app-printing-cancel"></a>
#### Cancel

_use case_ — `app/printing/uc_cancel.go:17`

**Answers to** [R-DRUCK-ABBRUCH](#req-R-DRUCK-ABBRUCH)

<a id="req-code-github-com-torbenschinke-eventprint-app-printing-diagnose"></a>
#### Diagnose

_query_ — `app/printing/uc_diagnose.go:19`

**Answers to** [R-DRUCK-DIAGNOSE](#req-R-DRUCK-DIAGNOSE)

<a id="req-code-github-com-torbenschinke-eventprint-app-printing-findalljobs"></a>
#### FindAllJobs

_query_ — `app/printing/uc_find_all_jobs.go:13`

**Answers to** [R-DRUCK-STATUS](#req-R-DRUCK-STATUS)

<a id="req-code-github-com-torbenschinke-eventprint-app-printing-findjobbyid"></a>
#### FindJobByID

_query_ — `app/printing/uc_find_job_by_id.go:9`

**Answers to** [R-DRUCK-STATUS](#req-R-DRUCK-STATUS)

<a id="req-code-github-com-torbenschinke-eventprint-app-printing-preview"></a>
#### Preview

_query_ — `app/printing/uc_preview.go:28`

**Answers to** [R-DRUCK-VORSCHAU](#req-R-DRUCK-VORSCHAU)

<a id="req-code-github-com-torbenschinke-eventprint-app-printing-print"></a>
#### Print

_query_ — `app/printing/uc_print.go:43`

**Answers to** [R-DRUCK-AUFTRAG](#req-R-DRUCK-AUFTRAG), [R-DRUCK-GESTALTUNG](#req-R-DRUCK-GESTALTUNG), [R-DRUCK-KEIN-NACHDRUCK](#req-R-DRUCK-KEIN-NACHDRUCK)

<a id="req-code-github-com-torbenschinke-eventprint-app-printing-printsimple"></a>
#### PrintSimple

_query_ — `app/printing/uc_print_simple.go:26`

**Answers to** [R-DRUCK-KIOSK](#req-R-DRUCK-KIOSK), [R-MODUS-PRIVAT](#req-R-MODUS-PRIVAT)

<a id="req-code-github-com-torbenschinke-eventprint-app-printing-resume"></a>
#### Resume

_use case_ — `app/printing/uc_resume.go:17`

**Answers to** [R-DRUCK-FREIGABE](#req-R-DRUCK-FREIGABE)

<a id="req-code-github-com-torbenschinke-eventprint-app-printing-retry"></a>
#### Retry

_use case_ — `app/printing/uc_retry.go:15`

**Answers to** [R-DRUCK-WIEDERHOLUNG](#req-R-DRUCK-WIEDERHOLUNG)

<a id="req-code-de-torbenschinke-eventprint-printing-cancel"></a>
#### de.torbenschinke.eventprint.printing.cancel

_permission_ — `app/printing/perm.go:78`

<a id="req-code-de-torbenschinke-eventprint-printing-diagnose"></a>
#### de.torbenschinke.eventprint.printing.diagnose

_permission_ — `app/printing/perm.go:57`

<a id="req-code-de-torbenschinke-eventprint-printing-find-all-jobs"></a>
#### de.torbenschinke.eventprint.printing.find\_all\_jobs

_permission_ — `app/printing/perm.go:29`

<a id="req-code-de-torbenschinke-eventprint-printing-find-job-by-id"></a>
#### de.torbenschinke.eventprint.printing.find\_job\_by\_id

_permission_ — `app/printing/perm.go:36`

<a id="req-code-de-torbenschinke-eventprint-printing-preview"></a>
#### de.torbenschinke.eventprint.printing.preview

_permission_ — `app/printing/perm.go:50`

<a id="req-code-de-torbenschinke-eventprint-printing-print"></a>
#### de.torbenschinke.eventprint.printing.print

_permission_ — `app/printing/perm.go:22`

<a id="req-code-de-torbenschinke-eventprint-printing-print-simple"></a>
#### de.torbenschinke.eventprint.printing.print\_simple

_permission_ — `app/printing/perm.go:71`

<a id="req-code-de-torbenschinke-eventprint-printing-resume"></a>
#### de.torbenschinke.eventprint.printing.resume

_permission_ — `app/printing/perm.go:64`

<a id="req-code-de-torbenschinke-eventprint-printing-retry"></a>
#### de.torbenschinke.eventprint.printing.retry

_permission_ — `app/printing/perm.go:43`

<a id="req-code-github-com-torbenschinke-eventprint-app-printing-job"></a>
#### Job

_aggregate_ — `app/printing/model.go:63`

**Answers to** [R-DEC-ZUSTANDSABLAGE](#req-R-DEC-ZUSTANDSABLAGE)

### app/relay

<a id="req-code-github-com-torbenschinke-eventprint-app-relay-beginpairing"></a>
#### BeginPairing

_query_ — `app/relay/uc_begin_pairing.go:15`

**Answers to** [R-UPLOAD-KOPPLUNG](#req-R-UPLOAD-KOPPLUNG)

<a id="req-code-github-com-torbenschinke-eventprint-app-relay-completepairing"></a>
#### CompletePairing

_query_ — `app/relay/uc_complete_pairing.go:14`

**Answers to** [R-UPLOAD-KOPPLUNG](#req-R-UPLOAD-KOPPLUNG)

<a id="req-code-github-com-torbenschinke-eventprint-app-relay-uploadaddress"></a>
#### UploadAddress

_query_ — `app/relay/uc_upload_address.go:25`

**Answers to** [R-UPLOAD-EINGANG](#req-R-UPLOAD-EINGANG), [R-UPLOAD-SITZUNG](#req-R-UPLOAD-SITZUNG)

<a id="req-code-de-torbenschinke-eventprint-relay-complete-pair"></a>
#### de.torbenschinke.eventprint.relay.complete\_pair

_permission_ — `app/relay/perm.go:35`

<a id="req-code-de-torbenschinke-eventprint-relay-pair"></a>
#### de.torbenschinke.eventprint.relay.pair

_permission_ — `app/relay/perm.go:25`

<a id="req-code-de-torbenschinke-eventprint-relay-upload-address"></a>
#### de.torbenschinke.eventprint.relay.upload\_address

_permission_ — `app/relay/perm.go:13`

### app/upld

<a id="req-code-github-com-torbenschinke-eventprint-app-upld-ackjob"></a>
#### AckJob

_use case_ — `app/upld/uc_ack_job.go:9`

**Answers to** [R-UPLOAD-BESTAETIGUNG](#req-R-UPLOAD-BESTAETIGUNG)

<a id="req-code-github-com-torbenschinke-eventprint-app-upld-findpendingjobs"></a>
#### FindPendingJobs

_query_ — `app/upld/uc_find_pending_jobs.go:7`

**Answers to** [R-UPLOAD-ABHOLUNG](#req-R-UPLOAD-ABHOLUNG)

<a id="req-code-github-com-torbenschinke-eventprint-app-upld-openjobimage"></a>
#### OpenJobImage

_query_ — `app/upld/uc_open_job_image.go:15`

**Answers to** [R-UPLOAD-BILD](#req-R-UPLOAD-BILD)

<a id="req-code-github-com-torbenschinke-eventprint-app-upld-opensession"></a>
#### OpenSession

_query_ — `app/upld/uc_open_session.go:11`

**Answers to** [R-UPLOAD-SITZUNG](#req-R-UPLOAD-SITZUNG)

<a id="req-code-de-torbenschinke-photoupld-ack"></a>
#### de.torbenschinke.photoupld.ack

_permission_ — `app/upld/perm.go:38`

<a id="req-code-de-torbenschinke-photoupld-fetch"></a>
#### de.torbenschinke.photoupld.fetch

_permission_ — `app/upld/perm.go:31`

<a id="req-code-de-torbenschinke-photoupld-poll"></a>
#### de.torbenschinke.photoupld.poll

_permission_ — `app/upld/perm.go:24`

<a id="req-code-de-torbenschinke-photoupld-session"></a>
#### de.torbenschinke.photoupld.session

_permission_ — `app/upld/perm.go:17`

### app/usb

<a id="req-code-github-com-torbenschinke-eventprint-app-usb-drives"></a>
#### Drives

_query_ — `app/usb/uc_drives.go:13`

**Answers to** [R-ARCHIV-EXPORT](#req-R-ARCHIV-EXPORT), [R-QUELLEN-USB](#req-R-QUELLEN-USB)

<a id="req-code-github-com-torbenschinke-eventprint-app-usb-eject"></a>
#### Eject

_use case_ — `app/usb/uc_eject.go:15`

**Answers to** [R-ARCHIV-EXPORT](#req-R-ARCHIV-EXPORT)

<a id="req-code-github-com-torbenschinke-eventprint-app-usb-export"></a>
#### Export

_query_ — `app/usb/uc_export.go:22`

**Answers to** [R-ARCHIV-EXPORT](#req-R-ARCHIV-EXPORT)

<a id="req-code-github-com-torbenschinke-eventprint-app-usb-images"></a>
#### Images

_query_ — `app/usb/uc_images.go:15`

**Answers to** [R-QUELLEN-USB](#req-R-QUELLEN-USB)

<a id="req-code-github-com-torbenschinke-eventprint-app-usb-read"></a>
#### Read

_query_ — `app/usb/uc_read.go:16`

**Answers to** [R-QUELLEN-USB](#req-R-QUELLEN-USB)

<a id="req-code-de-torbenschinke-eventprint-usb-drives"></a>
#### de.torbenschinke.eventprint.usb.drives

_permission_ — `app/usb/perm.go:23`

<a id="req-code-de-torbenschinke-eventprint-usb-eject"></a>
#### de.torbenschinke.eventprint.usb.eject

_permission_ — `app/usb/perm.go:37`

<a id="req-code-de-torbenschinke-eventprint-usb-export"></a>
#### de.torbenschinke.eventprint.usb.export

_permission_ — `app/usb/perm.go:30`

<a id="req-code-de-torbenschinke-eventprint-usb-images"></a>
#### de.torbenschinke.eventprint.usb.images

_permission_ — `app/usb/perm.go:44`

<a id="req-code-de-torbenschinke-eventprint-usb-read"></a>
#### de.torbenschinke.eventprint.usb.read

_permission_ — `app/usb/perm.go:51`

### app/wifi

<a id="req-code-github-com-torbenschinke-eventprint-app-wifi-connect"></a>
#### Connect

_use case_ — `app/wifi/uc_connect.go:15`

**Answers to** [R-NETZ-BETREUUNG](#req-R-NETZ-BETREUUNG), [R-NETZ-VERBINDEN](#req-R-NETZ-VERBINDEN)

<a id="req-code-github-com-torbenschinke-eventprint-app-wifi-current"></a>
#### Current

_query_ — `app/wifi/uc_current.go:11`

**Answers to** [R-NETZ-BETREUUNG](#req-R-NETZ-BETREUUNG), [R-NETZ-ZUSTAND](#req-R-NETZ-ZUSTAND)

<a id="req-code-github-com-torbenschinke-eventprint-app-wifi-scan"></a>
#### Scan

_query_ — `app/wifi/uc_scan.go:13`

**Answers to** [R-NETZ-BETREUUNG](#req-R-NETZ-BETREUUNG), [R-NETZ-SUCHE](#req-R-NETZ-SUCHE)

<a id="req-code-de-torbenschinke-eventprint-wifi-connect"></a>
#### de.torbenschinke.eventprint.wifi.connect

_permission_ — `app/wifi/perm.go:33`

<a id="req-code-de-torbenschinke-eventprint-wifi-scan"></a>
#### de.torbenschinke.eventprint.wifi.scan

_permission_ — `app/wifi/perm.go:19`

<a id="req-code-de-torbenschinke-eventprint-wifi-status"></a>
#### de.torbenschinke.eventprint.wifi.status

_permission_ — `app/wifi/perm.go:26`

## The boundary

_No topology is declared, so what this system talks to is stated nowhere._

## What answers from outside

| Address | Takes | Returns | Serves | Asked for by |
|---|---|---|---|---|
| `DELETE /api/v1/job` | — | `AckResponse` | [AckJob](#req-code-github-com-torbenschinke-eventprint-app-upld-ackjob) | R-UPLOAD-BESTAETIGUNG |
| `GET /api/v1/job/image` | — | — | [OpenJobImage](#req-code-github-com-torbenschinke-eventprint-app-upld-openjobimage) | R-UPLOAD-BILD |
| `GET /api/v1/jobs` | — | `JobResponse` | [FindPendingJobs](#req-code-github-com-torbenschinke-eventprint-app-upld-findpendingjobs) | R-UPLOAD-ABHOLUNG |
| `POST /api/v1/pairing` | `PairingRequest` | `PairingResponse` | [RequestPairing](#req-code-github-com-torbenschinke-eventprint-app-pairing-requestpairing) | R-UPLOAD-KOPPLUNG |
| `POST /api/v1/pairing/confirm` | `PairingConfirmation` | `Result` | [ConfirmPairing](#req-code-github-com-torbenschinke-eventprint-app-pairing-confirmpairing) | R-UPLOAD-KOPPLUNG |
| `POST /api/v1/session` | — | `SessionResponse` | [OpenSession](#req-code-github-com-torbenschinke-eventprint-app-upld-opensession) | R-UPLOAD-SITZUNG |

### What crosses each address

The fields below are read from the code that mounts the route, not from a hand written schema. A name in the **wire** column is what appears in the payload; where it differs from the field it is because the code says so.

#### DELETE /api/v1/job

Reaches `AckJob`.

**Takes** _nothing_

**Returns** `AckResponse`

| Field | Wire | Shape | Omitted when empty |
|---|---|---|---:|
| `Acknowledged` | `acknowledged` | `bool` | no |

#### GET /api/v1/jobs

Reaches `FindPendingJobs`.

**Takes** _nothing_

**Returns** `JobResponse` — `[]{id:string,template:string,filename:string,createdAt:string}`

#### POST /api/v1/pairing

Reaches `RequestPairing`.

**Takes** `PairingRequest`

| Field | Wire | Shape | Omitted when empty |
|---|---|---|---:|
| `Mail` | `mail` | `string` | no |
| `Device` | `device` | `string` | no |

**Returns** `PairingResponse`

| Field | Wire | Shape | Omitted when empty |
|---|---|---|---:|
| `Pairing` | `pairing` | `string` | no |

#### POST /api/v1/pairing/confirm

Reaches `ConfirmPairing`.

**Takes** `PairingConfirmation`

| Field | Wire | Shape | Omitted when empty |
|---|---|---|---:|
| `Pairing` | `pairing` | `string` | no |
| `Code` | `code` | `string` | no |

**Returns** `Result`

| Field | Wire | Shape | Omitted when empty | Means |
|---|---|---|---:|---|
| `Status` | `status` | `string` | no |  |
| `Token` | `token` | `string` | yes | Token ist das Zugangstoken der Box, nur bei \[StatusPaired\]. Es wird genau einmal herausgegeben und nirgends im Klartext gespeichert. |

#### POST /api/v1/session

Reaches `OpenSession`.

**Takes** _nothing_

**Returns** `SessionResponse`

| Field | Wire | Shape | Omitted when empty |
|---|---|---|---:|
| `UploadID` | `uploadId` | `string` | no |
| `UploadURL` | `uploadUrl` | `string` | no |

## Courses of business

_No course of business is declared, so no requirement is placed in one._

## The register

Every requirement that was read, and how far each one has got. A mark states what was measured; where nothing looked, it says so rather than reporting a zero.

| Requirement | Kind | Field | Status | Built | Tested | Run | Read |
|---|---|---|---|---:|---:|---:|---:|
| [R-ARCHIV-EXPORT](#req-R-ARCHIV-EXPORT) | functional | business | normative | yes | yes | yes | no |
| [R-ARCHIV-LOESCHEN](#req-R-ARCHIV-LOESCHEN) | functional | business | normative | yes | yes | yes | no |
| [R-ARCHIV-PLATZ](#req-R-ARCHIV-PLATZ) | functional | business | normative | yes | yes | yes | no |
| [R-DEC-OBERFLAECHE](#req-R-DEC-OBERFLAECHE) | decision | technical | informative | n/a | n/a | n/a | n/a |
| [R-DEC-ZUSTANDSABLAGE](#req-R-DEC-ZUSTANDSABLAGE) | decision | technical | normative | yes | no | n/a | no |
| [R-DRUCK-ABBRUCH](#req-R-DRUCK-ABBRUCH) | functional | business | normative | yes | yes | yes | no |
| [R-DRUCK-AUFTRAG](#req-R-DRUCK-AUFTRAG) | functional | business | normative | yes | yes | yes | no |
| [R-DRUCK-DIAGNOSE](#req-R-DRUCK-DIAGNOSE) | functional | mixed | normative | yes | yes | yes | no |
| [R-DRUCK-FREIGABE](#req-R-DRUCK-FREIGABE) | functional | mixed | normative | yes | yes | yes | no |
| [R-DRUCK-GESTALTUNG](#req-R-DRUCK-GESTALTUNG) | functional | business | normative | yes | yes | yes | no |
| [R-DRUCK-KEIN-NACHDRUCK](#req-R-DRUCK-KEIN-NACHDRUCK) | functional | mixed | normative | yes | yes | yes | no |
| [R-DRUCK-KIOSK](#req-R-DRUCK-KIOSK) | functional | business | normative | yes | yes | yes | no |
| [R-DRUCK-PAPIER](#req-R-DRUCK-PAPIER) | functional | business | normative | yes | yes | yes | no |
| [R-DRUCK-STATUS](#req-R-DRUCK-STATUS) | functional | business | normative | yes | yes | yes | no |
| [R-DRUCK-VORSCHAU](#req-R-DRUCK-VORSCHAU) | functional | business | normative | yes | yes | yes | no |
| [R-DRUCK-WIEDERHOLUNG](#req-R-DRUCK-WIEDERHOLUNG) | functional | business | normative | yes | yes | yes | no |
| [R-FOTO-DRUCKVORLAGE](#req-R-FOTO-DRUCKVORLAGE) | functional | mixed | normative | yes | yes | yes | no |
| [R-FOTO-EINGANG](#req-R-FOTO-EINGANG) | functional | business | normative | yes | yes | yes | no |
| [R-FOTO-EINZELBILD](#req-R-FOTO-EINZELBILD) | functional | business | normative | yes | yes | yes | no |
| [R-FOTO-HISTORIE](#req-R-FOTO-HISTORIE) | functional | business | normative | yes | yes | yes | no |
| [R-FOTO-IMPORT](#req-R-FOTO-IMPORT) | functional | business | normative | yes | yes | yes | no |
| [R-FOTO-LOESCHEN](#req-R-FOTO-LOESCHEN) | functional | business | normative | yes | yes | yes | no |
| [R-MODUS-ANZEIGE](#req-R-MODUS-ANZEIGE) | functional | business | normative | yes | yes | yes | no |
| [R-MODUS-BETREUUNG](#req-R-MODUS-BETREUUNG) | functional | business | normative | yes | yes | yes | no |
| [R-MODUS-EINSTELLUNGEN](#req-R-MODUS-EINSTELLUNGEN) | functional | business | normative | yes | yes | yes | no |
| [R-MODUS-HEIM](#req-R-MODUS-HEIM) | functional | business | normative | yes | yes | yes | no |
| [R-MODUS-KIOSK](#req-R-MODUS-KIOSK) | functional | business | normative | yes | yes | yes | no |
| [R-MODUS-PRIVAT](#req-R-MODUS-PRIVAT) | functional | business | normative | yes | yes | yes | no |
| [R-NETZ-BETREUUNG](#req-R-NETZ-BETREUUNG) | functional | mixed | normative | yes | yes | yes | no |
| [R-NETZ-SUCHE](#req-R-NETZ-SUCHE) | functional | mixed | normative | yes | yes | yes | no |
| [R-NETZ-VERBINDEN](#req-R-NETZ-VERBINDEN) | functional | mixed | normative | yes | yes | yes | no |
| [R-NETZ-ZUSTAND](#req-R-NETZ-ZUSTAND) | functional | mixed | normative | yes | yes | yes | no |
| [R-QUELLEN-NAS](#req-R-QUELLEN-NAS) | functional | business | normative | yes | yes | yes | no |
| [R-QUELLEN-USB](#req-R-QUELLEN-USB) | functional | business | normative | yes | yes | yes | no |
| [R-UPLOAD-ABHOLUNG](#req-R-UPLOAD-ABHOLUNG) | functional | business | normative | yes | yes | yes | no |
| [R-UPLOAD-BESTAETIGUNG](#req-R-UPLOAD-BESTAETIGUNG) | functional | business | normative | yes | yes | yes | no |
| [R-UPLOAD-BILD](#req-R-UPLOAD-BILD) | functional | business | normative | yes | yes | yes | no |
| [R-UPLOAD-EINGANG](#req-R-UPLOAD-EINGANG) | functional | business | normative | yes | yes | yes | no |
| [R-UPLOAD-KOPPLUNG](#req-R-UPLOAD-KOPPLUNG) | functional | mixed | normative | yes | yes | yes | no |
| [R-UPLOAD-SITZUNG](#req-R-UPLOAD-SITZUNG) | functional | business | normative | yes | yes | yes | no |

### Reading the marks

- yes  yes, and it was measured
- part  in part
- no  no, and it was measured
- ?  not measured; nothing is claimed either way
- n/a  does not apply to this entry

- **Built** — something in the source declares that it satisfies this.
- **Tested** — a test claims it.
- **Run** — that test was seen to pass against this exact wording.
- **Read** — a named person recorded that they read this exact wording.

## Requirements

<a id="req-R-ARCHIV-EXPORT"></a>
### R-ARCHIV-EXPORT — Fotos auf einen USB-Stick kopieren

Die Fotos einer Feier, eine Auswahl oder alle Fotos MÜSSEN sich im Original auf einen USB-Stick kopieren lassen; ein Abbruch MUSS sich ohne doppelte Dateien wiederholen lassen, und der Stick MUSS sich sicher auswerfen lassen.

_functional, business, normative._

- **Asked for in** requirements/\_sources/archiv.md#fotos-weitergeben
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/photo.Locate`
  - `github.com/torbenschinke/eventprint/app/photo.Photo.Name`
  - `github.com/torbenschinke/eventprint/app/usb.Drives`
  - `github.com/torbenschinke/eventprint/app/usb.Eject`
  - `github.com/torbenschinke/eventprint/app/usb.Export`
- **Demonstrated by** TestEject, TestEjectUnmountsAndPowersOff, TestExportCountsOnlyMissingFilesAgainstFreeSpace, TestExportFailsEarlyWhenTheStickIsFull, TestExportIsIdempotent, TestExportMountsOnDemandAndCopies, TestExportNeverOverwrites, TestExportRefusesTheSystemDisk, TestExportStopsWhenCancelled

<a id="req-R-ARCHIV-LOESCHEN"></a>
### R-ARCHIV-LOESCHEN — Feier abschließen

Die Fotos einer Feier MÜSSEN sich gesammelt löschen lassen; private Fotos DÜRFEN davon nicht betroffen sein.

_functional, business, normative._

- **Asked for in** requirements/\_sources/archiv.md#feier-abschließen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/photo.PurgeEvent`
- **Demonstrated by** TestPurgeEventRemovesOnlyThatEvent

<a id="req-R-ARCHIV-PLATZ"></a>
### R-ARCHIV-PLATZ — Speicherplatz einsehen

Es MUSS sichtbar sein, wie viel Platz die Fotos belegen und wie viel frei ist.

_functional, business, normative._

- **Asked for in** requirements/\_sources/archiv.md#speicherplatz-einsehen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/photo.InspectStorage`
- **Demonstrated by** TestInspectStorageReportsPhotosAndDisk

<a id="req-R-DEC-OBERFLAECHE"></a>
### R-DEC-OBERFLAECHE — Die Oberfläche wird mit gift gezeichnet, nicht im Browser

Die Oberfläche am Gerät wird mit gift direkt auf die GPU gezeichnet; es gibt keinen Browser und keinen lokalen Webserver mehr.

_decision, technical, informative._

**Why.** Ohne Chromium startet das Gerät schneller, braucht weniger Speicher und
kann nicht versehentlich eine Webseite verlassen. Ein Absturz des Browsers,
eine Wiederherstellungsfrage nach dem harten Ausschalten und die Ableitung der
öffentlichen Adresse aus der ersten Verbindung entfallen als Fehlerquellen.

**What it costs.** Die Oberfläche lässt sich nicht mehr aus der Ferne im Browser öffnen.
Oberflächentests laufen mit dem Test-Harness von gift statt mit Playwright.
Der Upload-Dienst im Internet bleibt eine Nago-Anwendung.

- **Asked for in** requirements/\_sources/entscheidungen.md#oberfläche-ohne-browser

<a id="req-R-DEC-ZUSTANDSABLAGE"></a>
### R-DEC-ZUSTANDSABLAGE — Aggregate werden als Zustand abgelegt, nicht als Ereignisfolge

Foto und Druckauftrag MÜSSEN als aktueller Zustand gespeichert werden; ihr Verlauf wird nicht als Folge von Ereignissen aufbewahrt.

_decision, technical, normative._

**Why.** Eine Fotobox läuft einen Abend lang. Gefragt ist, ob das Bild auf Papier
ist, nicht, in welcher Reihenfolge ein Auftrag seine Zustände durchlaufen hat.
Der Zustand passt in eine JSON-Ablage, die sich ohne Werkzeug lesen und im
Zweifel von Hand reparieren lässt – auf einer Feier um Mitternacht ist das der
entscheidende Vorteil.

**What it costs.** Der Verlauf ist unwiederbringlich verloren: Warum ein Auftrag zweimal
gescheitert ist, lässt sich hinterher nicht mehr rekonstruieren, und genau das
hat die Suche nach den ungewollten Nachdrucken erschwert. Eine spätere
Auswertung über mehrere Veranstaltungen hinweg ist aus diesen Daten nicht zu
gewinnen. Die Umstellung wäre nachträglich teuer, weil bestehende Daten keine
Ereignisse enthalten, aus denen sich ein Verlauf bilden ließe.

- **Asked for in** requirements/\_sources/entscheidungen.md#form-der-ablage
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/pairing.Box`
  - `github.com/torbenschinke/eventprint/app/photo.Photo`
  - `github.com/torbenschinke/eventprint/app/printing.Job`

<a id="req-R-DRUCK-ABBRUCH"></a>
### R-DRUCK-ABBRUCH — Auftrag abbrechen

Ein noch nicht gedruckter Auftrag MUSS sich abbrechen lassen und darf danach nicht doch noch gedruckt werden.

_functional, business, normative._

- **Asked for in** requirements/\_sources/druck.md#auftrag-abbrechen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/printing.Cancel`
- **Demonstrated by** TestCancelQueuedJobIsNeverPrinted, TestCancelWhilePrintingWithdrawsPrinterJob

<a id="req-R-DRUCK-AUFTRAG"></a>
### R-DRUCK-AUFTRAG — Druckauftrag annehmen und im Hintergrund abarbeiten

Ein Foto MUSS mit dem gewählten Layout sofort in die Warteschlange gestellt und im Hintergrund gedruckt werden.

_functional, business, normative._

- **Asked for in** requirements/\_sources/druck.md#druckauftrag-erteilen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/printing.Job.Photos`
  - `github.com/torbenschinke/eventprint/app/printing.Print`
  - `github.com/torbenschinke/eventprint/app/printing.Print`
- **Demonstrated by** TestWorkerReportsSuccess

<a id="req-R-DRUCK-DIAGNOSE"></a>
### R-DRUCK-DIAGNOSE — Zustand des Druckers ohne Terminal erkennen

Der Zustand des Druckers MUSS in der Oberfläche erkennbar sein, einschließlich fehlender Warteschlange, angehaltenem Gerät und Meldungen des Geräts.

_functional, mixed, normative._

- **Asked for in** requirements/\_sources/druck.md#zustand-des-druckers
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/printing.Diagnose`
  - `github.com/torbenschinke/eventprint/app/printing.Diagnose`
- **Demonstrated by** TestDiagnoseReportsPrinterState

<a id="req-R-DRUCK-FREIGABE"></a>
### R-DRUCK-FREIGABE — Angehaltenen Drucker ohne Terminal freigeben

Hält der Druckdienst den Drucker an, MUSS die Fotobox ihn selbsttätig wieder freigeben, und die Betreuung MUSS ihn in der Oberfläche sofort freigeben können. Solange er angehalten ist, DARF ein wartender Auftrag nicht wegen Zeitüberschreitung verworfen werden.

_functional, mixed, normative._

- **Asked for in** requirements/\_sources/druck.md#drucker-freigeben
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/printing.Resume`
- **Demonstrated by** TestAwaitJobPausesDeadlineWhileStopped, TestResumeGuardReleasesStoppedQueue, TestResumeUseCase

<a id="req-R-DRUCK-GESTALTUNG"></a>
### R-DRUCK-GESTALTUNG — Gestaltung des Blattes

Das Blatt MUSS sich in Format, Rahmen und Rahmenfarbe, Farbanmutung, Beschriftung, Datumsstempel und Oberfläche gestalten lassen; alle Formate teilen sich das eine Papier des Druckers.

_functional, business, normative._

- **Asked for in** requirements/\_sources/druck.md#gestaltung-des-blattes
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/photo.Photo.Height`
  - `github.com/torbenschinke/eventprint/app/photo.Photo.Width`
  - `github.com/torbenschinke/eventprint/app/printing.Job.Layout`
  - `github.com/torbenschinke/eventprint/app/printing.Print`
  - `github.com/torbenschinke/eventprint/app/printing.Print`
- **Demonstrated by** TestComposeMatteWidths, TestPolaroidFallsBackWhenFacesDoNotFit, TestPrintSheetsFollowTheFormat, TestPrintSplitsBatchIntoSheets

<a id="req-R-DRUCK-KEIN-NACHDRUCK"></a>
### R-DRUCK-KEIN-NACHDRUCK — Kein Ausdruck ohne Auslösung

Ein aufgegebener Druckauftrag MUSS beim Druckdienst zurückgenommen werden, damit kein Ausdruck ohne erneute Auslösung entsteht.

_functional, mixed, normative._

- **Asked for in** requirements/\_sources/druck.md#kein-ungewollter-ausdruck
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/printing.Job.PrinterJob`
  - `github.com/torbenschinke/eventprint/app/printing.Print`
  - `github.com/torbenschinke/eventprint/app/printing.Print`
- **Demonstrated by** TestAwaitJobCancelsAbandonedJob, TestCancelQueuedJobIsNeverPrinted, TestCancelWhilePrintingWithdrawsPrinterJob, TestRecoverStaleJobsOnRestart

<a id="req-R-DRUCK-KIOSK"></a>
### R-DRUCK-KIOSK — Drucken im Kiosk

Gäste MÜSSEN ein Foto mit einem Tipp in einem freigegebenen Kiosk-Layout in begrenzter Anzahl drucken können; im Kiosk MÜSSEN Bilder von Handy und Kamera je nach Einstellung sofort gedruckt werden.

_functional, business, normative._

- **Asked for in** requirements/\_sources/druck.md#drucken-im-kiosk
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/device.Intake`
  - `github.com/torbenschinke/eventprint/app/printing.PrintSimple`
- **Demonstrated by** TestIntakeKiosk, TestIntakePrintFailure, TestKioskGuestSeesOnlyPhotosOfTheEvent, TestPolaroidFallsBackWhenFacesDoNotFit, TestPrintSimpleClampsCopies, TestPrintSimpleWithoutLimitPrintsOnce

<a id="req-R-DRUCK-PAPIER"></a>
### R-DRUCK-PAPIER — Papiervorrat

Der verbleibende Papiervorrat MUSS sichtbar sein; ein neu eingelegtes Set MUSS sich melden lassen.

_functional, business, normative._

- **Asked for in** requirements/\_sources/druck.md#papiervorrat
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/device.ConsumePaper`
  - `github.com/torbenschinke/eventprint/app/device.RefillPaper`
- **Demonstrated by** TestPaper

<a id="req-R-DRUCK-STATUS"></a>
### R-DRUCK-STATUS — Zustand der Druckaufträge einsehen

Alle Druckaufträge MÜSSEN mit Zustand und Fehlerursache abrufbar sein, vollständig wie auch einzeln anhand ihrer Kennung.

_functional, business, normative._

- **Asked for in** requirements/\_sources/druck.md#zustand-der-aufträge
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/printing.FindAllJobs`
  - `github.com/torbenschinke/eventprint/app/printing.FindAllJobs`
  - `github.com/torbenschinke/eventprint/app/printing.FindJobByID`
  - `github.com/torbenschinke/eventprint/app/printing.FindJobByID`
  - `github.com/torbenschinke/eventprint/app/printing.Job.Batch`
  - `github.com/torbenschinke/eventprint/app/printing.Job.CreatedAt`
  - `github.com/torbenschinke/eventprint/app/printing.Job.FinishedAt`
  - `github.com/torbenschinke/eventprint/app/printing.Job.ID`
  - `github.com/torbenschinke/eventprint/app/printing.Job.Message`
  - `github.com/torbenschinke/eventprint/app/printing.Job.Printer`
  - `github.com/torbenschinke/eventprint/app/printing.Job.Reason`
  - `github.com/torbenschinke/eventprint/app/printing.Job.Sheet`
  - `github.com/torbenschinke/eventprint/app/printing.Job.Sheets`
  - `github.com/torbenschinke/eventprint/app/printing.Job.State`
- **Demonstrated by** TestJobsAreListedNewestFirst

<a id="req-R-DRUCK-VORSCHAU"></a>
### R-DRUCK-VORSCHAU — Vorschau vor dem Druck

Vor dem Druck MUSS das Ergebnis des gewählten Layouts als Bild sichtbar sein.

_functional, business, normative._

- **Asked for in** requirements/\_sources/druck.md#vorschau-des-ergebnisses
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/printing.Preview`
  - `github.com/torbenschinke/eventprint/app/printing.Preview`
- **Demonstrated by** TestDecodeOriginalWithoutScaledDecoder, TestPreviewLoadsReducedAndOnce, TestPreviewRendersWithoutPrinting

<a id="req-R-DRUCK-WIEDERHOLUNG"></a>
### R-DRUCK-WIEDERHOLUNG — Gescheiterten Auftrag wiederholen

Ein gescheiterter Druckauftrag MUSS sich wiederholen lassen, ohne dass ein zweiter Ausdruck desselben Bildes entsteht.

_functional, business, normative._

- **Asked for in** requirements/\_sources/druck.md#auftrag-wiederholen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/printing.Retry`
  - `github.com/torbenschinke/eventprint/app/printing.Retry`
- **Demonstrated by** TestRetryCancelsPreviousPrinterJob

<a id="req-R-FOTO-DRUCKVORLAGE"></a>
### R-FOTO-DRUCKVORLAGE — Druck aus den Originaldaten

Das Bild, das der Drucker bekommt, MUSS aus derselben unveränderten Quelle stammen wie das, was die Mediathek zeigt.

_functional, mixed, normative._

- **Asked for in** requirements/\_sources/foto.md#vorlage-für-den-druck
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/photo.Locate`
  - `github.com/torbenschinke/eventprint/app/photo.OpenOriginal`
  - `github.com/torbenschinke/eventprint/app/photo.Photo.File`
- **Demonstrated by** TestOriginalIsThePrintSource

<a id="req-R-FOTO-EINGANG"></a>
### R-FOTO-EINGANG — Eingang im Heimbetrieb

Im Heimbetrieb MÜSSEN Bilder von Handy und Kamera im Eingang landen, ohne von allein gedruckt zu werden, und als neu gelten, bis sie für den Druck ausgewählt werden.

_functional, business, normative._

- **Asked for in** requirements/\_sources/foto.md#eingang
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/device.Intake`
  - `github.com/torbenschinke/eventprint/app/photo.MarkSeen`
  - `github.com/torbenschinke/eventprint/app/photo.Photo.Unseen`
- **Demonstrated by** TestHomeModeShowsInboxAfterStart, TestInboxCollectsPrivatePhotosFromPhoneAndCamera, TestInboxPhotoStaysNewUntilSelected, TestIntakeHome

<a id="req-R-FOTO-EINZELBILD"></a>
### R-FOTO-EINZELBILD — Einzelnes Bild über seine Kennung finden

Ein einzelnes Bild MUSS anhand seiner Kennung auffindbar sein.

_functional, business, normative._

- **Asked for in** requirements/\_sources/foto.md#einzelnes-bild
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/photo.FindByID`
  - `github.com/torbenschinke/eventprint/app/photo.Photo.ID`
- **Demonstrated by** TestFindByIDReturnsTheSinglePhoto

<a id="req-R-FOTO-HISTORIE"></a>
### R-FOTO-HISTORIE — Mediathek mit Favoriten und gedruckten Fotos

Alle Fotos MÜSSEN in einer Mediathek sichtbar sein, die neuesten zuerst; Fotos MÜSSEN sich als Favorit markieren lassen, gedruckte Fotos MÜSSEN erkennbar sein.

_functional, business, normative._

- **Asked for in** requirements/\_sources/foto.md#mediathek
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/photo.FindAll`
  - `github.com/torbenschinke/eventprint/app/photo.MarkPrinted`
  - `github.com/torbenschinke/eventprint/app/photo.Photo.CreatedAt`
  - `github.com/torbenschinke/eventprint/app/photo.Photo.Favorite`
  - `github.com/torbenschinke/eventprint/app/photo.Photo.Prints`
  - `github.com/torbenschinke/eventprint/app/photo.SetFavorite`
- **Demonstrated by** TestConcurrentUpdatesAreNotLost, TestHistoryNewestFirstWithFavoritesAndPrinted

<a id="req-R-FOTO-IMPORT"></a>
### R-FOTO-IMPORT — Eingehende Bilder aufnehmen und im Original sichern

Ein eingehendes Bild MUSS unabhängig von seiner Herkunft aufgenommen und dabei unverändert gesichert werden.

_functional, business, normative._

- **Asked for in** requirements/\_sources/foto.md#bilder-aufnehmen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/photo.Import`
  - `github.com/torbenschinke/eventprint/app/photo.Photo.Source`
- **Demonstrated by** TestImportHonoursExifOrientationWithoutTouchingTheFile, TestImportKeepsHEICAsHEIC, TestImportRejectsWhatIsNoPhoto, TestImportStoresOriginalByteForByteFromEverySource

<a id="req-R-FOTO-LOESCHEN"></a>
### R-FOTO-LOESCHEN — Bilder endgültig entfernen

Ein Bild MUSS sich samt gesicherter Datei endgültig entfernen lassen.

_functional, business, normative._

- **Asked for in** requirements/\_sources/foto.md#bilder-entfernen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/photo.Delete`
- **Demonstrated by** TestDeleteRemovesMetadataAndOriginal

<a id="req-R-MODUS-ANZEIGE"></a>
### R-MODUS-ANZEIGE — Bedienbar auf jedem Touchscreen

Die Oberfläche MUSS auf Touchscreens von 800 × 480 bis Full-HD vollständig bedienbar sein und sich nach Auflösung und Größe des Panels bemessen; das Erscheinungsbild MUSS sich hell, dunkel oder nach der Tageszeit wählen lassen.

_functional, business, normative._

- **Asked for in** requirements/\_sources/modus.md#bildschirme
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/device.Appearance`
- **Demonstrated by** TestSmallPanelIsUsable

<a id="req-R-MODUS-BETREUUNG"></a>
### R-MODUS-BETREUUNG — Betreuung im Kiosk nur mit PIN

Im Kiosk MÜSSEN Einstellungen und Mediathek gesperrt sein; mit einer PIN MUSS sich die Betreuung befristet freischalten lassen, und Fehleingaben MÜSSEN das Raten ausbremsen.

_functional, business, normative._

- **Asked for in** requirements/\_sources/modus.md#betreuung-im-kiosk
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/device.SetPin`
  - `github.com/torbenschinke/eventprint/app/device.StopKiosk`
  - `github.com/torbenschinke/eventprint/app/device.Unlock`
- **Demonstrated by** TestKioskOperatorNeedsThePin, TestPinNotInClearText, TestSetPinValidation, TestStopKiosk, TestUnlock, TestUnlockStaysThrottledAfterManyGuesses, TestUnlockThrottlesGuessing

<a id="req-R-MODUS-EINSTELLUNGEN"></a>
### R-MODUS-EINSTELLUNGEN — Einstellungen am Gerät

Alle Einstellungen MÜSSEN sich am Gerät selbst vornehmen lassen, ohne Terminal und ohne zweiten Rechner.

_functional, business, normative._

- **Asked for in** requirements/\_sources/modus.md#einstellungen-am-gerät
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/device.LoadSettings`
  - `github.com/torbenschinke/eventprint/app/device.SaveSettings`
- **Demonstrated by** TestLoadSettingsDefaults, TestNormalized, TestSaveSettings, TestSettingsAreChangedOnTheDevice

<a id="req-R-MODUS-HEIM"></a>
### R-MODUS-HEIM — Heimbetrieb nach dem Einschalten

Nach jedem Einschalten MUSS das Gerät im Heimbetrieb starten; ein Neustart allein der Anwendung darf einen laufenden Kiosk nicht beenden.

_functional, business, normative._

- **Asked for in** requirements/\_sources/modus.md#heimbetrieb-nach-dem-start
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/device.CurrentKiosk`
- **Demonstrated by** TestHomeAfterBoot, TestHomeModeShowsInboxAfterStart

<a id="req-R-MODUS-KIOSK"></a>
### R-MODUS-KIOSK — Kiosk bis zum nächsten Einschalten, mit Vorabprüfung

Der Besitzer MUSS das Gerät in den Kiosk versetzen können, der bis zum nächsten Einschalten gilt; vorher MUSS das Gerät Drucker, Papier, Upload-Dienst und Kamera prüfen und das Ergebnis zeigen.

_functional, business, normative._

- **Asked for in** requirements/\_sources/modus.md#kiosk-starten
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/device.Preflight`
  - `github.com/torbenschinke/eventprint/app/device.StartKiosk`
- **Demonstrated by** TestKioskHidesTheLibraryFromGuests, TestPreflight, TestStartKiosk

<a id="req-R-MODUS-PRIVAT"></a>
### R-MODUS-PRIVAT — Gäste sehen nur die Fotos der Feier

Im Kiosk DÜRFEN Gäste ausschließlich die Fotos der laufenden Feier sehen und drucken; private Fotos und die anderer Feiern MÜSSEN verborgen bleiben, auch bei bekannter Kennung.

_functional, business, normative._

- **Asked for in** requirements/\_sources/modus.md#feier-und-mediathek-getrennt
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/photo.FindByID`
  - `github.com/torbenschinke/eventprint/app/photo.FindEvent`
  - `github.com/torbenschinke/eventprint/app/photo.Locate`
  - `github.com/torbenschinke/eventprint/app/photo.Photo.Event`
  - `github.com/torbenschinke/eventprint/app/printing.PrintSimple`
- **Demonstrated by** TestGuestSeesOnlyTheRunningEvent, TestKioskGuestSeesOnlyPhotosOfTheEvent, TestKioskHidesTheLibraryFromGuests

<a id="req-R-NETZ-BETREUUNG"></a>
### R-NETZ-BETREUUNG — Funknetz nur durch die Betreuung wechseln

Das Suchen, Anzeigen und Wechseln des Funknetzes MUSS der Betreuung vorbehalten sein.

_functional, mixed, normative._

- **Asked for in** requirements/\_sources/netz.md#nur-für-die-betreuung
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/wifi.Connect`
  - `github.com/torbenschinke/eventprint/app/wifi.Current`
  - `github.com/torbenschinke/eventprint/app/wifi.Scan`
- **Demonstrated by** TestGuestMayNotChangeTheNetwork

<a id="req-R-NETZ-SUCHE"></a>
### R-NETZ-SUCHE — Verfügbare Funknetze auflisten

Die verfügbaren Funknetze MÜSSEN sich auflisten lassen. Ein Funknetz MUSS genau einmal erscheinen, auch wenn es über mehrere Zugangspunkte oder Frequenzbänder empfangen wird.

_functional, mixed, normative._

- **Asked for in** requirements/\_sources/netz.md#netze-finden
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/wifi.Scan`
- **Demonstrated by** TestOneNetworkAppearsOnce

<a id="req-R-NETZ-VERBINDEN"></a>
### R-NETZ-VERBINDEN — Mit einem Funknetz verbinden

Die Fotobox MUSS sich über die Oberfläche mit einem gewählten Funknetz verbinden lassen. Das Kennwort eines gesicherten Netzes MUSS verdeckt abgefragt werden, und ein abgelehntes Kennwort MUSS sich von einer sonst gescheiterten Verbindung unterscheiden lassen.

_functional, mixed, normative._

- **Asked for in** requirements/\_sources/netz.md#verbindung-herstellen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/wifi.Connect`
- **Demonstrated by** TestWrongPasswordIsToldApartFromOtherFailures

<a id="req-R-NETZ-ZUSTAND"></a>
### R-NETZ-ZUSTAND — Bestehende Funkverbindung erkennen

Die Oberfläche MUSS zeigen, mit welchem Funknetz die Fotobox verbunden ist und wie gut der Empfang ist.

_functional, mixed, normative._

- **Asked for in** requirements/\_sources/netz.md#verbindung-erkennen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/wifi.Current`
- **Demonstrated by** TestStatusJoinsDeviceAndSignal

<a id="req-R-QUELLEN-NAS"></a>
### R-QUELLEN-NAS — Fotos vom NAS übernehmen

Eine Netzwerkfreigabe im Heimnetz MUSS sich am Gerät einrichten lassen; ihre Ordner und Bilder MÜSSEN sich durchsuchen und übernehmen lassen, und gelesen werden darf nur die eingerichtete Freigabe.

_functional, business, normative._

- **Asked for in** requirements/\_sources/quellen.md#nas-als-quelle
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/nas.Browse`
  - `github.com/torbenschinke/eventprint/app/nas.Read`
  - `github.com/torbenschinke/eventprint/app/nas.Shares`
  - `github.com/torbenschinke/eventprint/app/nas.Thumbnail`
- **Demonstrated by** TestBrowseShare, TestNASIsSetUpAndBrowsedOnTheDevice, TestSetUpShare, TestThumbnailsAndRead

<a id="req-R-QUELLEN-USB"></a>
### R-QUELLEN-USB — Bilder vom USB-Stick übernehmen

Bilder auf einem USB-Stick MÜSSEN sich durchsuchen und übernehmen lassen; gelesen werden darf nur der Stick.

_functional, business, normative._

- **Asked for in** requirements/\_sources/quellen.md#usb-stick-als-quelle
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/usb.Drives`
  - `github.com/torbenschinke/eventprint/app/usb.Images`
  - `github.com/torbenschinke/eventprint/app/usb.Read`
- **Demonstrated by** TestDrivesListsOnlyTheStick, TestImagesFindsPrintableImagesNewestFirst, TestReadStaysOnTheStick

<a id="req-R-UPLOAD-ABHOLUNG"></a>
### R-UPLOAD-ABHOLUNG — Wartende Aufträge abholen

Eine Fotobox MUSS die für sie hinterlegten Aufträge abrufen können und dabei ausschließlich ihre eigenen sehen.

_functional, business, normative._

- **Asked for in** requirements/\_sources/upload.md#wartende-aufträge-abholen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/upld.FindPendingJobs`
  - `github.com/torbenschinke/eventprint/app/upld.FindPendingJobs`
- **Demonstrated by** TestFindPendingJobsShowsOnlyOwnJobs

<a id="req-R-UPLOAD-BESTAETIGUNG"></a>
### R-UPLOAD-BESTAETIGUNG — Auftrag erst nach Bestätigung löschen

Ein Auftrag MUSS beim Dienst erhalten bleiben, bis die Fotobox seine Übernahme bestätigt hat.

_functional, business, normative._

- **Asked for in** requirements/\_sources/upload.md#übernahme-bestätigen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/upld.AckJob`
  - `github.com/torbenschinke/eventprint/app/upld.AckJob`
- **Demonstrated by** TestAckJobKeepsTheJobUntilConfirmed

<a id="req-R-UPLOAD-BILD"></a>
### R-UPLOAD-BILD — Originalbild eines wartenden Auftrags laden

Zu einem wartenden Auftrag MUSS das Originalbild abrufbar sein.

_functional, business, normative._

- **Asked for in** requirements/\_sources/upload.md#bild-eines-auftrags-laden
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/upld.OpenJobImage`
  - `github.com/torbenschinke/eventprint/app/upld.OpenJobImage`
- **Demonstrated by** TestOpenJobImageDeliversTheOriginal

<a id="req-R-UPLOAD-EINGANG"></a>
### R-UPLOAD-EINGANG — Upload mehrerer Bilder in den Eingang

Im Heimbetrieb MUSS die Upload-Seite mehrere Bilder auf einmal ohne Abfrage der Gestaltung annehmen; welche Art von Upload gemeint ist, MUSS die Adresse im QR-Code bestimmen.

_functional, business, normative._

- **Asked for in** requirements/\_sources/upload.md#upload-in-den-eingang
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/relay.UploadAddress`
- **Demonstrated by** TestEnqueueKeepsInboxTemplate, TestInboxAddressAddsModeAndKeepsSession, TestInboxTakesSeveralImagesWithoutLayout, TestJobsAreDeliveredOnceAndAcknowledged, TestRemainingCountsDownToFull, TestUploadAddressSelectsInbox

<a id="req-R-UPLOAD-KOPPLUNG"></a>
### R-UPLOAD-KOPPLUNG — Fotobox mit dem Konto koppeln

Registrierte, bestätigte Nutzer MÜSSEN Fotoboxen per Mailadresse und sechsstelligem, 30 Minuten gültigem Code selbst koppeln können; die Box DARF nicht erfahren, ob es das Konto gibt, das Zugangstoken MUSS automatisch ausgetauscht werden, und falsche Codes MÜSSEN nach wenigen Versuchen sperren.

_functional, mixed, normative._

- **Asked for in** requirements/\_sources/upload.md#fotobox-mit-dem-konto-koppeln
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/pairing.Box.Device`
  - `github.com/torbenschinke/eventprint/app/pairing.Box.ID`
  - `github.com/torbenschinke/eventprint/app/pairing.Box.Mail`
  - `github.com/torbenschinke/eventprint/app/pairing.Box.Owner`
  - `github.com/torbenschinke/eventprint/app/pairing.Box.PairedAt`
  - `github.com/torbenschinke/eventprint/app/pairing.ConfirmPairing`
  - `github.com/torbenschinke/eventprint/app/pairing.FindMyBoxes`
  - `github.com/torbenschinke/eventprint/app/pairing.RequestPairing`
  - `github.com/torbenschinke/eventprint/app/pairing.UnpairBox`
  - `github.com/torbenschinke/eventprint/app/relay.BeginPairing`
  - `github.com/torbenschinke/eventprint/app/relay.CompletePairing`
- **Demonstrated by** TestBoxCannotTellUnknownAccounts, TestBoxPairsWithMailAndCode, TestExpiryAndLockout, TestIssuerFailureIsAnError, TestOwnerSeesAndUnpairsOwnBoxes, TestPairWithMailAndCode, TestRepeatedRequestsAreThrottled, TestUnknownMailLooksTheSame

<a id="req-R-UPLOAD-SITZUNG"></a>
### R-UPLOAD-SITZUNG — Kurzlebige Upload-Adresse je Fotobox

Eine angemeldete Fotobox MUSS eine kurzlebige Upload-Adresse erhalten; je Fotobox darf höchstens eine Adresse gültig sein.

_functional, business, normative._

- **Asked for in** requirements/\_sources/upload.md#upload-sitzung
- **Implemented by**
  - `github.com/torbenschinke/eventprint/app/relay.UploadAddress`
  - `github.com/torbenschinke/eventprint/app/upld.OpenSession`
  - `github.com/torbenschinke/eventprint/app/upld.OpenSession`
- **Demonstrated by** TestChangedSettingsOpenANewSession, TestExpiredSessionIsReplaced, TestInboxAddressAddsModeAndKeepsSession, TestOpenSessionGivesEachBoxExactlyOneAddress, TestSessionProvidesTheUploadAddress, TestUploadAddressExplainsWhyThereIsNoCode/Token\_fehlt, TestUploadAddressExplainsWhyThereIsNoCode/nicht\_eingerichtet, TestUploadAddressExplainsWhyThereIsNoCode/noch\_keine\_Sitzung, TestUploadAddressExplainsWhyThereIsNoCode/nur\_Leerzeichen, TestUploadAddressReportsRejectedToken

## Source documents

What people wrote, and what became of each part of it.

### requirements/\_sources/archiv.md

| section | became |
|---|---|
| Weitergabe | _nothing, and says so_ |
| Fotos weitergeben | R-ARCHIV-EXPORT |
| Speicherplatz einsehen | R-ARCHIV-PLATZ |
| Feier abschließen | R-ARCHIV-LOESCHEN |

### requirements/\_sources/druck.md

| section | became |
|---|---|
| Drucken | _nothing, and says so_ |
| Druckauftrag erteilen | R-DRUCK-AUFTRAG |
| Gestaltung des Blattes | R-DRUCK-GESTALTUNG |
| Drucken im Kiosk | R-DRUCK-KIOSK |
| Auftrag abbrechen | R-DRUCK-ABBRUCH |
| Papiervorrat | R-DRUCK-PAPIER |
| Kein ungewollter Ausdruck | R-DRUCK-KEIN-NACHDRUCK |
| Zustand der Aufträge | R-DRUCK-STATUS |
| Auftrag wiederholen | R-DRUCK-WIEDERHOLUNG |
| Vorschau des Ergebnisses | R-DRUCK-VORSCHAU |
| Zustand des Druckers | R-DRUCK-DIAGNOSE |
| Drucker freigeben | R-DRUCK-FREIGABE |

### requirements/\_sources/entscheidungen.md

| section | became |
|---|---|
| Entscheidungen | _nothing, and says so_ |
| Form der Ablage | R-DEC-ZUSTANDSABLAGE |
| Oberfläche ohne Browser | R-DEC-OBERFLAECHE |

### requirements/\_sources/foto.md

| section | became |
|---|---|
| Fotos | _nothing, and says so_ |
| Bilder aufnehmen | R-FOTO-IMPORT |
| Eingang | R-FOTO-EINGANG |
| Mediathek | R-FOTO-HISTORIE |
| Einzelnes Bild | R-FOTO-EINZELBILD |
| Bilder entfernen | R-FOTO-LOESCHEN |
| Vorlage für den Druck | R-FOTO-DRUCKVORLAGE |

### requirements/\_sources/modus.md

| section | became |
|---|---|
| Betriebsarten | _nothing, and says so_ |
| Heimbetrieb nach dem Start | R-MODUS-HEIM |
| Kiosk starten | R-MODUS-KIOSK |
| Feier und Mediathek getrennt | R-MODUS-PRIVAT |
| Betreuung im Kiosk | R-MODUS-BETREUUNG |
| Einstellungen am Gerät | R-MODUS-EINSTELLUNGEN |
| Bildschirme | R-MODUS-ANZEIGE |

### requirements/\_sources/netz.md

| section | became |
|---|---|
| Funkverbindung | _nothing, and says so_ |
| Verbindung erkennen | R-NETZ-ZUSTAND |
| Netze finden | R-NETZ-SUCHE |
| Verbindung herstellen | R-NETZ-VERBINDEN |
| Nur für die Betreuung | R-NETZ-BETREUUNG |

### requirements/\_sources/quellen.md

| section | became |
|---|---|
| Quellen | _nothing, and says so_ |
| USB-Stick als Quelle | R-QUELLEN-USB |
| NAS als Quelle | R-QUELLEN-NAS |

### requirements/\_sources/upload.md

| section | became |
|---|---|
| Uploads aus dem Internet | _nothing, and says so_ |
| Upload-Sitzung | R-UPLOAD-SITZUNG |
| Wartende Aufträge abholen | R-UPLOAD-ABHOLUNG |
| Bild eines Auftrags laden | R-UPLOAD-BILD |
| Übernahme bestätigen | R-UPLOAD-BESTAETIGUNG |
| Upload in den Eingang | R-UPLOAD-EINGANG |
| Fotobox mit dem Konto koppeln | R-UPLOAD-KOPPLUNG |

