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
| Source segments accounted for | 15 | 100% |
| Normative requirements covered | 15 | 100% |
| … claimed by a test | 15 | 100% |
| … demonstrated by a run | 15 | 100% |
| … read by a person | 15 | 0% |

## Gaps

### Requirements nobody has read

- R-DRUCK-AUFTRAG
- R-DRUCK-DIAGNOSE
- R-DRUCK-KEIN-NACHDRUCK
- R-DRUCK-STATUS
- R-DRUCK-VORSCHAU
- R-DRUCK-WIEDERHOLUNG
- R-FOTO-DRUCKVORLAGE
- R-FOTO-EINZELBILD
- R-FOTO-HISTORIE
- R-FOTO-IMPORT
- R-FOTO-LOESCHEN
- R-UPLOAD-ABHOLUNG
- R-UPLOAD-BESTAETIGUNG
- R-UPLOAD-BILD
- R-UPLOAD-SITZUNG

## What has actually been run

A test that claims a requirement is a claim. Evidence that the test ran is something else, and this chapter keeps them apart.

|  | count | of normative |
|---|---:|---:|
| Normative requirements | 15 |  |
| … a test claims | 15 | 100% |
| … a run demonstrated | 15 | 100% |

### How much of the code a run went through

_no coverage profile has been handed to speclink evidence, so nothing is known about which code ran_

## The material

| Document | Kind | Segments | Cited | Read | Drifted |
|---|---|---:|---:|---:|---:|
| `requirements/_sources/druck.md` | markdown | 6 | 6 | 0 | 0 |
| `requirements/_sources/foto.md` | markdown | 5 | 5 | 0 | 0 |
| `requirements/_sources/upload.md` | markdown | 4 | 4 | 0 | 0 |

## Themes

_No theme is declared, so the requirements are not grouped._

## Standards

_No standard is declared, so no external clause is answered here._

## What gets built

This module builds 2 programs.

### photobox

Command photobox ist die Fotobox-Anwendung für Hochzeiten, Jubiläen und ähnliche Veranstaltungen.

Built from `cmd/photobox`.

_How this program is invoked could not be read from the source. That is a limit of the reading, not a statement that it takes no arguments._

### photoupld

Command photoupld exposes the public upload relay for a private photobox. #\[go.permission.generateTable\]

Built from `cmd/photoupld`.

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

17 packages in 0 bounded contexts, and 28 dependencies between them. Only this module's own packages: a dependency on the standard library or on a third party is not a fact about the shape of this system.

3 packages declare this specification rather than the system — the requirements, the courses of business, the boundary. They are left out of the drawing below: in a project that uses this tool properly they are most of the nodes and most of the arrows, and the architecture disappears underneath its own documentation.

_No diagram is included in this document. Pass -figures to speclink generate, after rendering the sources written by speclink diagrams._

### Where one context reaches into another

No package of one bounded context imports a package of another. The contexts are separate in the only sense a compiler can hold them to.

## What the code declares

30 constructs, each recognised by what it is rather than by an annotation saying so. Everything elsewhere in this document that names one of them points here.

### photo

<a id="req-code-github-com-torbenschinke-eventprint-photo-delete"></a>
#### Delete

_use case_ — `photo/usecases.go:36`

**Answers to** [R-FOTO-LOESCHEN](#req-R-FOTO-LOESCHEN)

<a id="req-code-github-com-torbenschinke-eventprint-photo-findall"></a>
#### FindAll

_use case_ — `photo/usecases.go:30`

**Answers to** [R-FOTO-HISTORIE](#req-R-FOTO-HISTORIE)

<a id="req-code-github-com-torbenschinke-eventprint-photo-findbyid"></a>
#### FindByID

_query_ — `photo/usecases.go:27`

**Answers to** [R-FOTO-EINZELBILD](#req-R-FOTO-EINZELBILD)

<a id="req-code-github-com-torbenschinke-eventprint-photo-findlatest"></a>
#### FindLatest

_query_ — `photo/usecases.go:33`

**Answers to** [R-FOTO-HISTORIE](#req-R-FOTO-HISTORIE)

<a id="req-code-github-com-torbenschinke-eventprint-photo-import"></a>
#### Import

_query_ — `photo/usecases.go:24`

**Answers to** [R-FOTO-IMPORT](#req-R-FOTO-IMPORT)

<a id="req-code-github-com-torbenschinke-eventprint-photo-openoriginal"></a>
#### OpenOriginal

_query_ — `photo/usecases.go:40`

**Answers to** [R-FOTO-DRUCKVORLAGE](#req-R-FOTO-DRUCKVORLAGE)

<a id="req-code-de-torbenschinke-eventprint-photo-delete"></a>
#### de.torbenschinke.eventprint.photo.delete

_permission_ — `photo/perm.go:24`

<a id="req-code-de-torbenschinke-eventprint-photo-find-all"></a>
#### de.torbenschinke.eventprint.photo.find\_all

_permission_ — `photo/perm.go:18`

<a id="req-code-de-torbenschinke-eventprint-photo-find-by-id"></a>
#### de.torbenschinke.eventprint.photo.find\_by\_id

_permission_ — `photo/perm.go:12`

<a id="req-code-de-torbenschinke-eventprint-photo-import"></a>
#### de.torbenschinke.eventprint.photo.import

_permission_ — `photo/perm.go:6`

<a id="req-code-github-com-torbenschinke-eventprint-photo-photo"></a>
#### Photo

_aggregate_ — `photo/model.go:57`

### printing

<a id="req-code-github-com-torbenschinke-eventprint-printing-diagnose"></a>
#### Diagnose

_use case_ — `printing/usecases.go:40`

**Answers to** [R-DRUCK-DIAGNOSE](#req-R-DRUCK-DIAGNOSE)

<a id="req-code-github-com-torbenschinke-eventprint-printing-findalljobs"></a>
#### FindAllJobs

_use case_ — `printing/usecases.go:23`

**Answers to** [R-DRUCK-STATUS](#req-R-DRUCK-STATUS)

<a id="req-code-github-com-torbenschinke-eventprint-printing-findjobbyid"></a>
#### FindJobByID

_query_ — `printing/usecases.go:26`

**Answers to** [R-DRUCK-STATUS](#req-R-DRUCK-STATUS)

<a id="req-code-github-com-torbenschinke-eventprint-printing-preview"></a>
#### Preview

_query_ — `printing/usecases.go:33`

**Answers to** [R-DRUCK-VORSCHAU](#req-R-DRUCK-VORSCHAU)

<a id="req-code-github-com-torbenschinke-eventprint-printing-print"></a>
#### Print

_query_ — `printing/usecases.go:20`

**Answers to** [R-DRUCK-AUFTRAG](#req-R-DRUCK-AUFTRAG), [R-DRUCK-KEIN-NACHDRUCK](#req-R-DRUCK-KEIN-NACHDRUCK)

<a id="req-code-github-com-torbenschinke-eventprint-printing-retry"></a>
#### Retry

_use case_ — `printing/usecases.go:29`

**Answers to** [R-DRUCK-WIEDERHOLUNG](#req-R-DRUCK-WIEDERHOLUNG)

<a id="req-code-de-torbenschinke-eventprint-printing-find-all-jobs"></a>
#### de.torbenschinke.eventprint.printing.find\_all\_jobs

_permission_ — `printing/perm.go:12`

<a id="req-code-de-torbenschinke-eventprint-printing-find-job-by-id"></a>
#### de.torbenschinke.eventprint.printing.find\_job\_by\_id

_permission_ — `printing/perm.go:18`

<a id="req-code-de-torbenschinke-eventprint-printing-print"></a>
#### de.torbenschinke.eventprint.printing.print

_permission_ — `printing/perm.go:6`

<a id="req-code-de-torbenschinke-eventprint-printing-retry"></a>
#### de.torbenschinke.eventprint.printing.retry

_permission_ — `printing/perm.go:24`

<a id="req-code-github-com-torbenschinke-eventprint-printing-job"></a>
#### Job

_aggregate_ — `printing/model.go:63`

### upld

<a id="req-code-github-com-torbenschinke-eventprint-upld-ackjob"></a>
#### AckJob

_use case_ — `upld/usecases.go:34`

**Answers to** [R-UPLOAD-BESTAETIGUNG](#req-R-UPLOAD-BESTAETIGUNG)

<a id="req-code-github-com-torbenschinke-eventprint-upld-findpendingjobs"></a>
#### FindPendingJobs

_query_ — `upld/usecases.go:23`

**Answers to** [R-UPLOAD-ABHOLUNG](#req-R-UPLOAD-ABHOLUNG)

<a id="req-code-github-com-torbenschinke-eventprint-upld-openjobimage"></a>
#### OpenJobImage

_query_ — `upld/usecases.go:28`

**Answers to** [R-UPLOAD-BILD](#req-R-UPLOAD-BILD)

<a id="req-code-github-com-torbenschinke-eventprint-upld-opensession"></a>
#### OpenSession

_query_ — `upld/usecases.go:19`

**Answers to** [R-UPLOAD-SITZUNG](#req-R-UPLOAD-SITZUNG)

<a id="req-code-de-torbenschinke-photoupld-ack"></a>
#### de.torbenschinke.photoupld.ack

_permission_ — `upld/perm.go:12`

<a id="req-code-de-torbenschinke-photoupld-fetch"></a>
#### de.torbenschinke.photoupld.fetch

_permission_ — `upld/perm.go:11`

<a id="req-code-de-torbenschinke-photoupld-poll"></a>
#### de.torbenschinke.photoupld.poll

_permission_ — `upld/perm.go:10`

<a id="req-code-de-torbenschinke-photoupld-session"></a>
#### de.torbenschinke.photoupld.session

_permission_ — `upld/perm.go:9`

## The boundary

_No topology is declared, so what this system talks to is stated nowhere._

## What answers from outside

| Address | Takes | Returns | Serves | Asked for by |
|---|---|---|---|---|
| `DELETE /api/v1/job` | — | `AckResponse` | [AckJob](#req-code-github-com-torbenschinke-eventprint-upld-ackjob) | R-UPLOAD-BESTAETIGUNG |
| `GET /api/v1/job/image` | — | — | [OpenJobImage](#req-code-github-com-torbenschinke-eventprint-upld-openjobimage) | R-UPLOAD-BILD |
| `GET /api/v1/jobs` | — | `JobResponse` | [FindPendingJobs](#req-code-github-com-torbenschinke-eventprint-upld-findpendingjobs) | R-UPLOAD-ABHOLUNG |
| `POST /api/v1/session` | — | `SessionResponse` | [OpenSession](#req-code-github-com-torbenschinke-eventprint-upld-opensession) | R-UPLOAD-SITZUNG |

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
| [R-DRUCK-AUFTRAG](#req-R-DRUCK-AUFTRAG) | functional | business | normative | yes | yes | yes | no |
| [R-DRUCK-DIAGNOSE](#req-R-DRUCK-DIAGNOSE) | functional | mixed | normative | yes | yes | yes | no |
| [R-DRUCK-KEIN-NACHDRUCK](#req-R-DRUCK-KEIN-NACHDRUCK) | functional | mixed | normative | yes | yes | yes | no |
| [R-DRUCK-STATUS](#req-R-DRUCK-STATUS) | functional | business | normative | yes | yes | yes | no |
| [R-DRUCK-VORSCHAU](#req-R-DRUCK-VORSCHAU) | functional | business | normative | yes | yes | yes | no |
| [R-DRUCK-WIEDERHOLUNG](#req-R-DRUCK-WIEDERHOLUNG) | functional | business | normative | yes | yes | yes | no |
| [R-FOTO-DRUCKVORLAGE](#req-R-FOTO-DRUCKVORLAGE) | functional | mixed | normative | yes | yes | yes | no |
| [R-FOTO-EINZELBILD](#req-R-FOTO-EINZELBILD) | functional | business | normative | yes | yes | yes | no |
| [R-FOTO-HISTORIE](#req-R-FOTO-HISTORIE) | functional | business | normative | yes | yes | yes | no |
| [R-FOTO-IMPORT](#req-R-FOTO-IMPORT) | functional | business | normative | yes | yes | yes | no |
| [R-FOTO-LOESCHEN](#req-R-FOTO-LOESCHEN) | functional | business | normative | yes | yes | yes | no |
| [R-UPLOAD-ABHOLUNG](#req-R-UPLOAD-ABHOLUNG) | functional | business | normative | yes | yes | yes | no |
| [R-UPLOAD-BESTAETIGUNG](#req-R-UPLOAD-BESTAETIGUNG) | functional | business | normative | yes | yes | yes | no |
| [R-UPLOAD-BILD](#req-R-UPLOAD-BILD) | functional | business | normative | yes | yes | yes | no |
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

<a id="req-R-DRUCK-AUFTRAG"></a>
### R-DRUCK-AUFTRAG — Druckauftrag annehmen und im Hintergrund abarbeiten

Ein Foto MUSS mit dem gewählten Layout sofort in die Warteschlange gestellt und im Hintergrund gedruckt werden.

_functional, business, normative._

- **Asked for in** requirements/\_sources/druck.md#druckauftrag-erteilen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/printing.Print`
- **Demonstrated by** TestWorkerReportsSuccess

<a id="req-R-DRUCK-DIAGNOSE"></a>
### R-DRUCK-DIAGNOSE — Zustand des Druckers ohne Terminal erkennen

Der Zustand des Druckers MUSS in der Oberfläche erkennbar sein, einschließlich fehlender Warteschlange, angehaltenem Gerät und Meldungen des Geräts.

_functional, mixed, normative._

- **Asked for in** requirements/\_sources/druck.md#zustand-des-druckers
- **Implemented by**
  - `github.com/torbenschinke/eventprint/printing.Diagnose`
- **Demonstrated by** TestDiagnoseReportsPrinterState

<a id="req-R-DRUCK-KEIN-NACHDRUCK"></a>
### R-DRUCK-KEIN-NACHDRUCK — Kein Ausdruck ohne Auslösung

Ein aufgegebener Druckauftrag MUSS beim Druckdienst zurückgenommen werden, damit kein Ausdruck ohne erneute Auslösung entsteht.

_functional, mixed, normative._

- **Asked for in** requirements/\_sources/druck.md#kein-ungewollter-ausdruck
- **Implemented by**
  - `github.com/torbenschinke/eventprint/printing.Print`
- **Demonstrated by** TestAwaitJobCancelsAbandonedJob

<a id="req-R-DRUCK-STATUS"></a>
### R-DRUCK-STATUS — Zustand der Druckaufträge einsehen

Alle Druckaufträge MÜSSEN mit Zustand und Fehlerursache abrufbar sein, vollständig wie auch einzeln anhand ihrer Kennung.

_functional, business, normative._

- **Asked for in** requirements/\_sources/druck.md#zustand-der-aufträge
- **Implemented by**
  - `github.com/torbenschinke/eventprint/printing.FindAllJobs`
  - `github.com/torbenschinke/eventprint/printing.FindJobByID`
- **Demonstrated by** TestJobsAreListedNewestFirst

<a id="req-R-DRUCK-VORSCHAU"></a>
### R-DRUCK-VORSCHAU — Vorschau vor dem Druck

Vor dem Druck MUSS das Ergebnis des gewählten Layouts als Bild sichtbar sein.

_functional, business, normative._

- **Asked for in** requirements/\_sources/druck.md#vorschau-des-ergebnisses
- **Implemented by**
  - `github.com/torbenschinke/eventprint/printing.Preview`
- **Demonstrated by** TestPreviewRendersWithoutPrinting

<a id="req-R-DRUCK-WIEDERHOLUNG"></a>
### R-DRUCK-WIEDERHOLUNG — Gescheiterten Auftrag wiederholen

Ein gescheiterter Druckauftrag MUSS sich wiederholen lassen, ohne dass ein zweiter Ausdruck desselben Bildes entsteht.

_functional, business, normative._

- **Asked for in** requirements/\_sources/druck.md#auftrag-wiederholen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/printing.Retry`
- **Demonstrated by** TestRetryCancelsPreviousPrinterJob

<a id="req-R-FOTO-DRUCKVORLAGE"></a>
### R-FOTO-DRUCKVORLAGE — Originaldaten als Vorlage für den Druck

Der Druck MUSS aus den unveränderten Originaldaten desselben Bildes erfolgen, das die Historie zeigt.

_functional, mixed, normative._

- **Asked for in** requirements/\_sources/foto.md#vorlage-für-den-druck
- **Implemented by**
  - `github.com/torbenschinke/eventprint/photo.OpenOriginal`
- **Demonstrated by** TestOpenOriginalDeliversThePrintSource

<a id="req-R-FOTO-EINZELBILD"></a>
### R-FOTO-EINZELBILD — Einzelnes Bild anhand seiner Kennung finden

Ein einzelnes Bild MUSS anhand seiner Kennung auffindbar sein, damit ein Nachdruck ohne Durchsuchen der Historie möglich ist.

_functional, business, normative._

- **Asked for in** requirements/\_sources/foto.md#einzelnes-bild
- **Implemented by**
  - `github.com/torbenschinke/eventprint/photo.FindByID`
- **Demonstrated by** TestFindByIDReturnsTheImportedPhoto

<a id="req-R-FOTO-HISTORIE"></a>
### R-FOTO-HISTORIE — Historie der Bilder, die neuesten zuerst

Die entstandenen Bilder MÜSSEN abrufbar sein, beginnend mit dem neuesten; wahlweise vollständig oder auf die jüngsten begrenzt.

_functional, business, normative._

- **Asked for in** requirements/\_sources/foto.md#historie
- **Implemented by**
  - `github.com/torbenschinke/eventprint/photo.FindAll`
  - `github.com/torbenschinke/eventprint/photo.FindLatest`
- **Demonstrated by** TestHistoryListsNewestFirst

<a id="req-R-FOTO-IMPORT"></a>
### R-FOTO-IMPORT — Eingehende Bilder aufnehmen und im Original sichern

Ein eingehendes Bild MUSS unabhängig von seiner Herkunft aufgenommen und dabei unverändert gesichert werden.

_functional, business, normative._

- **Asked for in** requirements/\_sources/foto.md#bilder-aufnehmen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/photo.Import`
- **Demonstrated by** TestImportArchivesUntouchedOriginal

<a id="req-R-FOTO-LOESCHEN"></a>
### R-FOTO-LOESCHEN — Bild aus der Historie entfernen

Ein Bild MUSS sich aus der Historie entfernen lassen, damit eine Fehlaufnahme nicht den ganzen Abend sichtbar bleibt.

_functional, business, normative._

- **Asked for in** requirements/\_sources/foto.md#bilder-entfernen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/photo.Delete`
- **Demonstrated by** TestDeleteRemovesPhotoFromHistory

<a id="req-R-UPLOAD-ABHOLUNG"></a>
### R-UPLOAD-ABHOLUNG — Wartende Aufträge abholen

Eine Fotobox MUSS die für sie hinterlegten Aufträge abrufen können und dabei ausschließlich ihre eigenen sehen.

_functional, business, normative._

- **Asked for in** requirements/\_sources/upload.md#wartende-aufträge-abholen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/upld.FindPendingJobs`
- **Demonstrated by** TestFindPendingJobsShowsOnlyOwnJobs

<a id="req-R-UPLOAD-BESTAETIGUNG"></a>
### R-UPLOAD-BESTAETIGUNG — Auftrag erst nach Bestätigung löschen

Ein Auftrag MUSS beim Dienst erhalten bleiben, bis die Fotobox seine Übernahme bestätigt hat.

_functional, business, normative._

- **Asked for in** requirements/\_sources/upload.md#übernahme-bestätigen
- **Implemented by**
  - `github.com/torbenschinke/eventprint/upld.AckJob`
- **Demonstrated by** TestAckJobKeepsTheJobUntilConfirmed

<a id="req-R-UPLOAD-BILD"></a>
### R-UPLOAD-BILD — Originalbild eines wartenden Auftrags laden

Zu einem wartenden Auftrag MUSS das Originalbild abrufbar sein.

_functional, business, normative._

- **Asked for in** requirements/\_sources/upload.md#bild-eines-auftrags-laden
- **Implemented by**
  - `github.com/torbenschinke/eventprint/upld.OpenJobImage`
- **Demonstrated by** TestOpenJobImageDeliversTheOriginal

<a id="req-R-UPLOAD-SITZUNG"></a>
### R-UPLOAD-SITZUNG — Kurzlebige Upload-Adresse je Fotobox

Eine angemeldete Fotobox MUSS eine kurzlebige Upload-Adresse erhalten; je Fotobox darf höchstens eine Adresse gültig sein.

_functional, business, normative._

- **Asked for in** requirements/\_sources/upload.md#upload-sitzung
- **Implemented by**
  - `github.com/torbenschinke/eventprint/upld.OpenSession`
- **Demonstrated by** TestOpenSessionGivesEachBoxExactlyOneAddress

## Source documents

What people wrote, and what became of each part of it.

### requirements/\_sources/druck.md

| section | became |
|---|---|
| Drucken | _nothing, and says so_ |
| Druckauftrag erteilen | R-DRUCK-AUFTRAG |
| Kein ungewollter Ausdruck | R-DRUCK-KEIN-NACHDRUCK |
| Zustand der Aufträge | R-DRUCK-STATUS |
| Auftrag wiederholen | R-DRUCK-WIEDERHOLUNG |
| Vorschau des Ergebnisses | R-DRUCK-VORSCHAU |
| Zustand des Druckers | R-DRUCK-DIAGNOSE |

### requirements/\_sources/foto.md

| section | became |
|---|---|
| Fotos | _nothing, and says so_ |
| Bilder aufnehmen | R-FOTO-IMPORT |
| Historie | R-FOTO-HISTORIE |
| Einzelnes Bild | R-FOTO-EINZELBILD |
| Bilder entfernen | R-FOTO-LOESCHEN |
| Vorlage für den Druck | R-FOTO-DRUCKVORLAGE |

### requirements/\_sources/upload.md

| section | became |
|---|---|
| Uploads aus dem Internet | _nothing, and says so_ |
| Upload-Sitzung | R-UPLOAD-SITZUNG |
| Wartende Aufträge abholen | R-UPLOAD-ABHOLUNG |
| Bild eines Auftrags laden | R-UPLOAD-BILD |
| Übernahme bestätigen | R-UPLOAD-BESTAETIGUNG |

