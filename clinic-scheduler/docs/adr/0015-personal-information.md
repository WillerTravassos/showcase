# 0015. Personal information: minimization and crypto-shredding

Status: accepted
Date: 2026-11-08

## Context

The system holds sexual health information. Law 25 and PIPEDA give patients rights of
access, rectification, and destruction, and TGV requires that destruction be
irreversible across all copies, backups included (P10.08). Event-sourced services never
delete events. TGV also requires encryption of sensitive data at rest (S07.01) and
minimal collection (P11.03, P11.06).

## Decision

- **One owner**: personal information lives only in `patient` (profile, questionnaire
  answers, consent) and `care` (test results). Other services and every event, log,
  span, and metric carry patient IDs only.
- **Per-patient keys**: health answers and test results are encrypted with a data key per
  patient, wrapped by [OpenBao](https://openbao.org) transit (envelope encryption: the
  key that encrypts the data is itself encrypted by a key OpenBao never releases).
- **Erasure**: destroying the patient's key (crypto-shredding) makes every copy,
  including backups, unreadable; rows are deleted as well.
- **Minimal profile**: legal name, date of birth, contact details, reminder preferences.
  Jurisdiction identity fields (Quebec: NAM, the health insurance number) are optional
  and modelled per jurisdiction; the full CN-GIU field set applies only to provincial
  integration, which is out of scope.
- **Questionnaires**: personal answers pre-fill from the profile; a change updates the
  profile only when the patient ticks "update my profile". Health answers belong to the
  appointment and are never pre-filled.
- **Consent**: recorded at registration and at each booking (purpose, date, privacy
  notice version), withdrawable.
- **Retention**: each data category has a retention period per jurisdiction, applied by a
  scheduled destruction job.

## Alternatives

- **Personal information inside events**: erasure would need rewriting history.
- **Database-level encryption only**: protects disks, not backups after erasure.

## Consequences

- Reads of encrypted fields cost an OpenBao call; data keys are cached briefly in memory,
  never on disk.
- Retention periods are placeholders until legal input confirms them.
