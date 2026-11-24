# PCPNDT Form F register and monthly report

A free, offline, single-binary tool for an ultrasound clinic or imaging centre
in India. It keeps the Form F record for every obstetric procedure, checks each
one against the whole prescribed field set in one pass, keeps the serially
numbered register that rule 9(1) requires, reconciles the month against the
centre's own day count, and assembles the monthly report that rule 9(8)
requires the centre to send to its Appropriate Authority.

It runs on the clinic's own machine, stores everything in one local file, and
makes no outbound network connections of any kind.

## What this tool will not hold

It does not record, infer, derive, display, print or export the sex of a
foetus. There is no field for it, no coded value that stands for it, no note
that carries it and no report that totals it, and there never will be. The
free-text clinical fields are checked before an entry is stored, and an entry
carrying such a term is refused with the term named rather than saved with a
warning. A test (`internal/formf/policy_test.go`) walks every field, item
number, label, column heading, choice, struct field name, JSON key and
generated page, and fails the build if any of them so much as names the
subject.

The Act this register serves exists to stop sex selection and the destruction
of unborn girls that follows from it. A tool built around its paperwork has to
be built so that it cannot be turned to that purpose.

One consequence is published rather than hidden: **item 4 of Form F is not
stored in full.** Item 4 asks for the total number of living children and then
splits that total two ways, giving the age of each child in each part. This
register holds the total and the ages and stops there. It keeps no field that
splits a household's children the way item 4 splits them. That split is the
thing the Act exists to stop anyone acting on, and a clinic computer holding it
for every household in the district would be a list of precisely the wrong
kind. It is completed on the signed Form F the centre keeps, which is the
document the Appropriate Authority inspects. The same reasoning applies to one
sub-field of the thumb-impression block in Section D. Both omissions are listed
in the interface and printed at the foot of every page this tool produces.

## What it does

Completeness checking is the product, not a feature of it. The proviso to
section 4(3) of the Act says that any deficiency or inaccuracy in the record
kept of an ultrasound on a pregnant woman amounts to a contravention of section
5 or section 6 **unless the contrary is proved** by the person who conducted it.
A blank field shifts the burden of proof onto the sonologist, and the exposure
behind it runs from prosecution under section 23 through suspension of the
centre's registration under section 20 to the seizure and sealing of records
and equipment under section 30 read with rule 12.

- **Every missing item at once.** `formf.Validate` walks the entire field set
  and returns every mandatory item that is blank, in the order the form prints
  them, with the item number beside each. Not the first blank — all of them.
- **What can only be wrong in combination.** A declaration dated after the
  procedure it was meant to precede. A gestational age that does not follow
  from the stated last menstrual period. A referral that is neither named nor a
  self-referral. An entry signed by somebody other than the person who
  performed the scan. A thumb impression with nobody attesting it. An invasive
  technique written into Section B.
- **Serial integrity.** The register assigns its own numbers, never reissues
  one left free by a gap, and has no way at all to delete an entry — a gap
  reads as a page torn out, so the tool offers no means of making one. Gaps are
  reported as runs ("serials 34 to 37 are missing — 4 entries"), alongside
  repeats, numbers from an earlier paper register, and entries dated backwards
  against their numbers.
- **Monthly completeness.** A Form F that was never written is invisible to the
  register that does not hold it. The Ministry's inspection guidance settles
  this by tallying Form F against the OPD register, so the centre enters its own
  day count and the month is reconciled day by day, with every shortfall named.
- **Retention per entry.** Two years from the date the procedure was completed,
  with calendar-correct arithmetic. An entry with proceedings noted against it
  never shows as releasable.
- **Pages to print.** The rule 9(1) register in its statutory column order, the
  face of any single Form F item by item with blanks marked, and the month's
  report — each carrying the authentication block rule 9(7) calls for.

## Who it's for

Ultrasound clinics, imaging centres, radiology and sonography practices, and
OB-GYN practices with their own machine: registered under the Act, keeping Form
F for every obstetric procedure, reporting monthly to a District Appropriate
Authority.

It is **not** for genetic clinics performing invasive procedures. Section C of
Form F — items 17 to 27, covering amniocentesis, chorionic villi aspiration,
foetal biopsy, cordocentesis and the Form G consent for them — is deliberately
not reproduced, because a centre registered for ultrasound alone is not
registered to perform any of them.

## The regulation

**Pre-conception and Pre-natal Diagnostic Techniques (Prohibition of Sex
Selection) Act, 1994**, with the **PNDT Rules, 1996**, as amended:

| Provision | What it requires |
|---|---|
| Rule 9(4) | The record for each person subjected to a procedure is as specified in **Form F**. The form in force was substituted by **G.S.R. 77(E) of 31 January 2014, w.e.f. 4 February 2014**: 27 numbered items in four sections, cited as `[see proviso to section 4(3), rule 9(4) and rule 10(1A)]`. This tool keeps Sections A, B and D. |
| Proviso to s.4(3) | A complete record; any deficiency or inaccuracy amounts to a contravention of s.5 or s.6 unless the contrary is proved. |
| Rule 10(1A) | The two declarations the form carries — by the person undergoing the procedure, and by the person conducting it, on each report. |
| Rule 9(1) | A register showing, **in serial order**, names and addresses, the spouse's or father's name, and the date of first reporting. Form F itself has no serial-number item; this is the register the tool prints for inspection. |
| Rule 9(6), s.29(1) | Records preserved **two years from the date of completion of the procedure or test**, and until the final disposal of any legal proceedings or the expiry of the two years, whichever is later. |
| Rule 9(7) | Where records are kept on a computer, a printed copy is taken and preserved **after authentication by a person responsible** for the record. |
| Rule 9(8) | A complete report of all procedures conducted in each month, to the concerned Appropriate Authority, **by the 5th day of the following month** (inserted 2003). |
| Section 17 | The Appropriate Authority. For an ultrasound clinic this is almost always the District Appropriate Authority; its name is plain text you fill in, because it is set by state notification. |

## Install and run

```sh
git clone https://github.com/sreenivas-sadhu-prabhakara/pcpndt-form-f-register-monthly.git
cd pcpndt-form-f-register-monthly

go build -o form-f-register .
./form-f-register
# then open http://127.0.0.1:8080

# or
go run .

# a different port, and a register file kept somewhere specific
./form-f-register -addr 127.0.0.1:9000 -data /srv/clinic/form-f-register.json
```

Go 1.25 to build, and nothing else: standard library only, no `require` block in
`go.mod`, no database, no container, no account. It binds to `127.0.0.1`, so it
answers only to the machine it runs on.

| Flag | Default |
|---|---|
| `-addr` | `127.0.0.1:8080` |
| `-data` | `./pcpndt-form-f-register-monthly-data.json` |

## Where the data is kept

One JSON file at the `-data` path, written atomically (temporary file, flushed,
then renamed) and left at mode `0600`. No account, no sync, no upload. The
program makes **no outbound network connections of any kind** — no telemetry, no
analytics, no external fonts or scripts, no update check. Pointing `-data` at a
JSON file this program did not write is refused rather than silently treated as
an empty register, so an unrelated file cannot be overwritten.

**Patient data must not leave the clinic.** This register holds names, ages,
addresses, telephone numbers and clinical findings for pregnant women. Keep the
file on a machine only the centre's own people can reach, back it up the way you
back up the rest of the clinic's records, and do not put it on a shared drive, a
personal laptop, a messaging app or any cloud folder. This tool gives the data
no way out; the rest is the centre's to protect.

## The figures it applies

Every statutory figure is a named constant in one place
(`internal/formf/retention.go`) and can be changed in the tool's settings if
what applies in your district differs.

| Figure | Default | Source | Overridable |
|---|---|---|---|
| Retention window | 2 years from the procedure date | Rule 9(6); s.29(1) | yes |
| Monthly report due | day 5 of the following month | Rule 9(8) | yes |
| Inspection sample | 30 forms or 10%, whichever is more | MoHFW Standard Operating Guidelines for District Appropriate Authorities | — |
| Advanced maternal age | 35 years | Item 19B of the form | — |

All of them are **indicative**. Confirm what applies in your district with your
Appropriate Authority before relying on any of them.

Date arithmetic is calendar-correct and tested on the awkward cases: a two-year
window on a procedure completed 29 February 2024 runs to 28 February 2026, not
1 March; the hold-until date is inclusive, so the entry is not called releasable
until 1 March 2026; a report due on day 5 of the following month lands correctly
across December into January.

## What we could not confirm

- **No format is prescribed for the monthly report.** Rule 9(8) requires "a
  complete report" and prescribes no form for it. Districts and states direct
  variously that it be filed as copies of the month's Form F, as a consolidated
  statement, or both, and several states run their own portal with its own
  cut-off. This tool assembles a complete month pack and tells you to copy it
  onto whatever format your Appropriate Authority requires. It imitates no
  state's form.
- **Online filing is a state direction, not a rule.** There is no online-filing
  provision in the central Rules. Rule 9(7) does require a printed,
  authenticated copy of a record kept on a computer. Where your state mandates
  online submission, that is in addition to the signed physical form.
- **Form F's exact typography varies between reproductions.** The field set was
  taken from the 2014 gazette text and cross-checked against three independent
  government reproductions that agree with each other, but reproductions differ
  among themselves on spelling ("fetal" against "foetal" in the same printed
  list) and punctuation. Nothing of substance has been reworded; one indication
  is phrased in current medical terms where the form uses an older phrasing. If
  the form in circulation in your state words an item differently, treat the
  form in your inspection file as the authority and use the free-text box
  beside the indication list.

## Layout

```
main.go                        flags, the JSON API, the embedded interface, the data file
internal/formf/fields.go       the prescribed field set — the one description everything is built from
internal/formf/record.go       the record, and which sections of Form F this register keeps
internal/formf/indications.go  item 10's twenty-three indications, in the form's numbering
internal/formf/validate.go     completeness and contradiction checking
internal/formf/screening.go    what this register refuses to store, and why
internal/formf/serial.go       serial integrity: gaps, repeats, back-dated entries
internal/formf/monthly.go      the day-count reconciliation and the month's report
internal/formf/retention.go    the statutory figures, in one place, with their sources
internal/formf/register.go     the printed pages
internal/formf/calendar.go     calendar arithmetic, clamped so a leap day stays in February
web/                           the embedded interface
docs/                          the explainer site
```

The rules live in `internal/formf` as pure functions, testable without HTTP,
with table-driven tests beside each one.

```sh
gofmt -l .
go vet ./...
go test ./... -count=1
go build -o /tmp/form-f-register .
```

## Scope and accuracy

This is a record-keeping aid that runs on your own computer. It is not legal
advice, it is not endorsed by or connected to any authority, and it **files
nothing** — it submits nothing, sends nothing and connects to nothing. Your
Appropriate Authority's requirements govern over anything this tool states. The
signed Form F, the written consent, the referral slip and the case papers held
at the centre remain the records that are inspected; the pages this tool prints
index them and do not replace them. Nothing here prevents an offence, and
nothing here is a defence to one. Confirm every figure with your Appropriate
Authority.

MIT licensed.
