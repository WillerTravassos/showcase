# Glossary

The shared vocabulary for clinic-scheduler. Tickets, test names, identifiers, and docs
use these terms with these meanings. Add a term when a new concept appears; change one
only with the user's agreement.

## Places and calendars

| Term | Meaning |
|------|---------|
| Jurisdiction | The region whose rules apply to a clinic, identified by an ISO 3166-2 code (`CA-QC`, `CA-ON`, `US-NY`). It selects the holiday calendar, the default language, the privacy notice, and the patient identity profile. |
| Clinic | A physical site where clinic appointments happen. Belongs to one jurisdiction and one IANA time zone. Has one to five rooms. Only an admin creates one. |
| Room | A space inside a clinic where one clinic appointment happens at a time. Only an admin creates one. |
| Holiday | A day on which no clinic in a jurisdiction is open, taken from the jurisdiction's holiday calendar. |
| Closure | A date range during which a clinic or a single room is not schedulable (renovation, outage). Entered by an admin. |
| Clinic schedule | The opening hours of a clinic, set months ahead. The main schedule every other schedule fits inside. |

## Time units

| Term | Meaning |
|------|---------|
| Block | The smallest unit of scheduled time. Its length is the clinic's block duration. |
| Block duration | The length of a block for a clinic (10 minutes by default). Effective-dated, divides 60 evenly, and cannot change for dates that already have bookings. |
| Slot | A bookable block at a clinic, with a capacity of one or more (the number of rooms and staff free in that block). Patients pick slots, never rooms or staff. Maps to FHIR `Slot`. |
| Hold | A ten-minute reservation of one room and one staff member in a slot, made when a patient selects the slot. Expires on its own unless the patient confirms. |

## People and roles

| Term | Meaning |
|------|---------|
| Patient | A person who books clinic appointments through the patient portal. Never "customer" or "user". |
| Staff | A clinic employee who runs clinic appointments. Works shifts, can work at several clinics in one day. |
| Doctor | A physician who takes follow-up calls from an office, within their availability. Maps to FHIR `Practitioner`. |
| Admin | A workforce member who manages clinics, rooms, schedules, staff, and doctors. |
| Auditor | A workforce member who reads audit records and reports, and nothing else. |
| Workforce | Staff, doctors, admins, and auditors together: everyone who signs in to the admin portal. |

## Schedules

| Term | Meaning |
|------|---------|
| Shift | A daily stretch of work for one staff member at one clinic, at most 8 hours. |
| Break | The last block of every hour of a shift. Never bookable. |
| Leave | Time a staff member or doctor is away: vacation, sick leave, or unpaid leave. |
| Availability | The time windows in which a doctor takes follow-up calls, with its own call length. |

## Booking

| Term | Meaning |
|------|---------|
| Clinic appointment | A confirmed booking that ties a patient, a clinic, a room, a staff member, and a slot together. |
| Follow-up call | A phone appointment between a patient and a doctor, booked by the patient after the test results of a clinic appointment arrive. |
| Booking flow | The ordered set of steps a patient goes through to book a clinic appointment, modelled as a directed acyclic graph. |
| Step | One screen of the booking flow (slot selection, personal questionnaire, health questionnaire, test selection, confirmation). Declares the steps it depends on. |
| Flow version | A published, immutable arrangement of the booking flow for one clinic. A booking session stays on the version it started with. |
| Booking session | One patient's progress through the booking flow, from slot selection to confirmation or expiry. |
| Questionnaire | A set of questions the patient answers during booking: personal (pre-filled from the profile) or health (never pre-filled). |
| Test | A laboratory test a patient can choose during booking. |
| Eligibility rule | A rule that turns health questionnaire answers into the tests a patient may select. Versioned per clinic. |
| Test result | The outcome of a test, entered by staff or a doctor. |

## Notifications, privacy, and audit

| Term | Meaning |
|------|---------|
| Reminder | A message sent the day before a clinic appointment, by email or SMS, at a time the patient chose. At most three per appointment. Carries no health information. |
| Consent | A recorded agreement by a patient to a stated purpose, with its date and the privacy notice version. Can be withdrawn. |
| Retention period | How long a category of personal information is kept in a jurisdiction before it is destroyed. |
| Audit record | An append-only entry stating who read or changed which personal information, when, and through what. |
