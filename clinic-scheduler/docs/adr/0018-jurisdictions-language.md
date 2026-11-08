# 0018. Jurisdictions, holidays, time zones, and language

Status: accepted
Date: 2026-11-08

## Context

The first clinics are in Quebec, with Ontario, the rest of Canada, and the United States
to follow. Holidays, privacy law, patient identity fields, and language differ by
jurisdiction. TGV G01 requires a French interface.

## Decision

- A clinic has a jurisdiction (ISO 3166-2 code, `CA-QC`) and an IANA time zone
  (`America/Toronto`).
- Holidays come from [rickar/cal](https://github.com/rickar/cal) v2 for the clinic's
  jurisdiction, plus admin-entered closures for a clinic or a room.
- Times are stored in UTC and shown in the clinic's time zone. Block arithmetic happens
  in clinic local time, so daylight saving changes never split a block.
- The jurisdiction selects the default language, the privacy notice, the retention
  periods, and the patient identity profile.
- Every user-facing string comes from a message catalogue
  ([go-i18n](https://github.com/nicksnyder/go-i18n)) in English and French. English is the
  default; the locale comes from the patient profile, then `Accept-Language`, and can be
  switched on every page.

## Alternatives

- **Hard-coded Quebec holidays**: blocks expansion.
- **Holidays entered by an admin only**: error-prone and repetitive each year.

## Consequences

- Coverage of each new jurisdiction in rickar/cal is checked before it is enabled.
- Strings missing a French translation fail a catalogue check in `make go-verify`.
