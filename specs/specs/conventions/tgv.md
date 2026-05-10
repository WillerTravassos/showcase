# TGV Certification Requirements — Quebec

## What is TGV

The **Trousse globale de vérification (TGV)** is Quebec's provincial quality assurance
framework for sexually transmitted and blood-borne infection (ITSS) clinical services.
It is administered by the **MSSS (Ministère de la Santé et des Services sociaux du Québec)**
and evaluated through the **IQSS (Institut québécois de la qualité des soins en santé)**.

TGV certification is required for clinics in Quebec that perform ITSS screening and
wish to operate within the provincial health network and receive RAMQ reimbursement.

**This system must be designed so that every TGV requirement can be demonstrated
to an MSSS auditor from within the application itself — through the audit log,
the consultation DAG event history, and the reporting module.**

---

## Regulatory Framework

The TGV operates within this legislative stack:

| Law / Regulation | Scope |
|---|---|
| Loi sur les services de santé et les services sociaux (LSSSS) | General healthcare quality |
| Loi sur la protection de la santé publique (LPSP) | Mandatory disease declaration (MADO) |
| Loi sur la protection des renseignements personnels dans le secteur privé | Private-sector PII (PIPEDA equivalent for QC) |
| Règlement sur l'organisation et l'administration des établissements | Clinic operational standards |
| Code des professions (CMQ, OPQ) | Scope of practice for doctors and pharmacists |

---

## MADO — Maladies à Déclaration Obligatoire

Certain ITSS diagnoses are **mandatory reportable conditions** under the LPSP.
When a DiagnosticReport contains a positive result for a MADO-listed condition,
the system must:

1. **Flag the result** as requiring mandatory declaration
2. **Notify the responsible physician** immediately (within the consultation flow)
3. **Generate a declaration record** for transmission to the regional DSP
   (Direction de santé publique)
4. **Track declaration status** (pending → submitted → acknowledged) within the appointment

### MADO-Listed ITSS Conditions

| Condition | ICD-10 | Declaration Deadline | Recipient |
|---|---|---|---|
| HIV infection (new diagnosis) | B20, Z21 | 48 hours | DSP régionale |
| Syphilis (infectious stages) | A51, A52 | 48 hours | DSP régionale + LSPQ |
| Gonorrhoea | A54 | 7 days | DSP régionale |
| Chlamydia (under 25 or symptomatic) | A56 | 7 days | DSP régionale |
| Hepatitis B (acute) | B16 | 48 hours | DSP régionale |
| Hepatitis C (acute) | B17.1 | 48 hours | DSP régionale |
| LGV (Lymphogranuloma venereum) | A55 | 48 hours | DSP régionale + LSPQ |

### MADO Implementation Requirements

- The test parser must tag each `Observation` with its ICD-10 code (from the AI extraction step)
- A `MADOEvaluator` service checks DiagnosticReports on ingestion — if any Observation matches a MADO condition and is positive, it creates a `MADODeclaration` record
- `MADODeclaration` has states: `pending_physician_review → ready_for_submission → submitted → acknowledged`
- MADO declarations are blocked from state `ready_for_submission` until the physician has signed off on the DiagnosticReport
- The DSP transmission format is XML per the MSSS HL7v2 specification (future: this may move to FHIR MessageHeader)
- A failed DSP transmission retries every 30 minutes for 48 hours, then alerts the admin

```go
// internal/api/pkg/mado/mado.go
type Declaration struct {
    ID              uuid.UUID
    AppointmentID   uuid.UUID
    PatientUUID     uuid.UUID
    ClinicID        uuid.UUID
    PhysicianUUID   uuid.UUID
    Condition       MADOCondition   // enum of reportable conditions
    ICD10Code       string
    ObservationID   uuid.UUID       // the specific DiagnosticReport Observation
    Status          DeclarationStatus
    DeadlineAt      time.Time       // 48h or 7d from DiagnosticReport.issued
    SubmittedAt     *time.Time
    AcknowledgedAt  *time.Time
    DSPRegion       string          // which DSP to notify
    CreatedAt       time.Time
}
```

---

## LSPQ — Laboratoire de santé publique du Québec

For specific conditions (syphilis, LGV), isolates must be transmitted to the LSPQ
for surveillance typing. The system must:

- Track whether a specimen has been flagged for LSPQ submission
- Record the LSPQ specimen reference number when received
- Notify clinic staff of outstanding LSPQ submissions

---

## TGV Domain Requirements

TGV certification evaluates clinics across eight domains. This system must support
documentation and demonstration of compliance in each.

### Domain 1 — Accessibility and Continuity of Services

**Requirements:**
- Clinic operating hours are configurable and published (done: clinic config)
- Walk-in and appointment-based access supported (done: slot booking)
- After-hours referral pathway is documented (clinic config: `afterhours_referral_url`)
- Wait time from booking to appointment is tracked and reportable

**Implementation:**
- Add `afterhours_referral_url` field to clinic config
- Appointment wait time = `slot.start_time - appointment.created_at`; must be queryable for reporting

### Domain 2 — Patient-Centred Care

**Requirements:**
- Consent is obtained and documented before any test (done: patient_consents table)
- Patients receive pre-test counselling (tracked as a consultation note type)
- Test results are communicated to patients in a timely manner
- Patient can request their own health record

**Implementation:**
- Add consultation note type `pre_test_counselling` — required note before SAMPLES_COLLECTED state transition
- Results release latency = `results_released_at - results_received_at`; must be queryable
- Patient record export endpoint (`GET /api/v1/patients/{uuid}/export`) returns all FHIR resources as a Bundle — must be implemented as a TGV requirement

### Domain 3 — Clinical Quality and Safety

**Requirements:**
- Test panels follow provincial ITSS screening guidelines
- Clinicians follow approved treatment protocols
- Prescriptions follow CMQ-approved formularies for ITSS treatment
- Abnormal results trigger follow-up within defined timeframes

**Implementation:**
- Test panel config includes a `guideline_reference` field (e.g. "MSSS ITSS Screening Guide 2023")
- MADO declarations enforce follow-up deadlines (see above)
- Abnormal result → phone appointment booking must occur within 5 business days (configurable)
- A `ProtocolAdherence` check runs on DAG state transitions to verify clinical steps are not skipped

### Domain 4 — Infection Prevention and Control

**Requirements:**
- Sample collection procedures are documented per test type
- Clinic worker certifications are tracked

**Implementation:**
- Add `collection_procedure_url` to test panel config (links to MSSS documentation)
- Add `certifications` field to staff profile: `[{type, issued_at, expires_at}]`
- Expiring certifications trigger a notification to the clinic admin 30 days before expiry

### Domain 5 — Documentation and Information Management

**Requirements:**
- Patient health record is complete, accurate, and legible (FHIR + audit trail)
- All clinical encounters are documented (consultation DAG event history)
- Records are retained per provincial requirement (10 years from last encounter)
- Access to records is controlled and audited (done: RBAC + audit log)

**Implementation:**
- Every DAG state transition that represents a clinical action must have a `clinical_note` in the payload — enforced at the service layer for states: SAMPLES_COLLECTED, PHONE_APPT_COMPLETED, RESULTS_RELEASED
- Record retention job (10 years for QC) must be implemented

### Domain 6 — Human Resources and Training

**Requirements:**
- All clinical staff have documented scope of practice
- Training records are maintained

**Implementation:**
- Staff profiles include `scope_of_practice` (free text, per professional order)
- Training records: `[{course_name, completed_at, provider, certificate_url}]`
- Admin portal shows staff training summary and upcoming expiries

### Domain 7 — Leadership and Governance

**Requirements:**
- Clinic has a documented quality improvement plan
- Patient complaints are tracked and resolved

**Implementation:**
- Clinic config includes `quality_plan_url` and `complaints_contact`
- Add `patient_feedback` table: `{appointment_id, category, body, status, resolved_at}`
- Feedback is visible to clinic admin and the assigned doctor

### Domain 8 — Continuous Quality Improvement

**Requirements:**
- Key performance indicators are tracked and reported
- Adverse events are documented
- Results are used to improve services

**Implementation:**
- A `ReportingService` must produce these KPIs on demand (queryable from admin portal):
  - Average wait time (booking to appointment) per clinic
  - Average result turnaround time (sample collection to results released)
  - MADO declaration submission rate (% submitted within deadline)
  - Appointment no-show rate per clinic
  - Test panel completion rate (% of booked tests that were actually collected)
- Adverse event log: `{type, description, occurred_at, reported_by, resolution}`

---

## TGV Reporting Module

A dedicated `reporting` package (`internal/api/pkg/reporting/`) must produce
TGV-compliant reports. These are NOT ad-hoc queries — they are named, versioned
report types with defined inputs and outputs.

```go
// internal/api/pkg/reporting/reporting.go
type ReportingService interface {
    // KPI reports
    WaitTimeReport(ctx context.Context, clinicID uuid.UUID, period DateRange) (*WaitTimeReport, error)
    TurnaroundReport(ctx context.Context, clinicID uuid.UUID, period DateRange) (*TurnaroundReport, error)
    MADOComplianceReport(ctx context.Context, clinicID uuid.UUID, period DateRange) (*MADOReport, error)
    NoShowReport(ctx context.Context, clinicID uuid.UUID, period DateRange) (*NoShowReport, error)

    // Regulatory exports
    ExportPatientRecord(ctx context.Context, patientUUID uuid.UUID) (*fhir.Bundle, error)
    ExportMADODeclarations(ctx context.Context, clinicID uuid.UUID, period DateRange) ([]MADODeclaration, error)
}
```

---

## Pre-Test Counselling Enforcement

TGV Domain 2 requires documented pre-test counselling. This is enforced in the DAG:

- The transition `ARRIVED → SAMPLES_COLLECTED` is **blocked** unless a note of type
  `pre_test_counselling` exists on the appointment, created by a `doctor` or `clinic_worker`
  after the appointment reached `ARRIVED` state.
- If the note is absent, the advance API returns a domain error `ErrCounsellingRequired`.
- This check is in the DAG service, not in the handler.

---

## Audit Requirements (TGV-Specific)

Beyond the general audit log, TGV requires:

- Audit records retained for **10 years** (aligned with record retention)
- Audit log must be tamper-evident — SHA-256 chaining of audit events per patient per day
  (each event includes a `previous_hash` field; the chain can be verified by the MSSS auditor)
- The `GET /api/v1/audit` admin endpoint must support export as a CSV or JSON file
  for regulatory submission
- Any access by a user outside the patient's assigned clinic must generate an
  `AuditEvent` with `flag: cross_clinic_access = true`

---

## Language Requirements

Quebec law requires that patients be served in French. The patient web app must:

- Default to French (`fr-CA`) for all UI text
- Support English as a secondary language via `Accept-Language` header
- All notification templates must have both `fr-CA` and `en-CA` variants
- FHIR Patient `communication` field must be populated with the patient's preferred language
- MADO declarations to DSP must be in French

Template files: `web/patient/templates/` supports locale subdirectories:
```
web/patient/templates/
├── fr/   ← default
└── en/
```
