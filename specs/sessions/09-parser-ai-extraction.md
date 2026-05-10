# Session 09 — Test Parser: AI Extraction

## Claude Code Delegation Prompt

```
You are implementing Session 09 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/tooling.md — github.com/WillerTravassos/showcase/pkg/log
  4. specs/overview/03-parser.md — LabReport contract, extractor interface, FHIR mapping
  5. specs/conventions/tgv.md — ICD-10 code tagging required for MADO evaluation

Session 08 is complete — PipelineResult, OCR pipeline, InboundMessage all exist.

Your job is to implement Stage 4 (AI extraction), Stage 5 (confidence validation + ICD-10
tagging for MADO), FHIR mapping, backend submission, and the needs_review queue.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./cmd/parser && go vet ./...
```

---

## Dependencies
- Session 08 (PipelineResult, OCR stages, ObjectStorage, InboundMessage)
- Session 01 (config, NATS)

---

## Deliverables

### 1. ReportExtractor Interface (`internal/parser/pkg/pipeline/extractor.go`)

```go
// ReportExtractor is the swappable interface for extraction backends.
// Current: OCRAIExtractor (OCR text → AI JSON).
// Future: LabAPIExtractor (lab API → LabReport directly).
// The FHIR mapping and backend submission stages are unchanged regardless of extractor.
type ReportExtractor interface {
    Extract(ctx context.Context, pages []PageText) (LabReport, error)
}
```

### 2. LabReport — The Central Contract (`internal/parser/pkg/pipeline/labreport.go`)

```go
// LabReport is the interface contract between extraction and FHIR mapping.
// Field names are stable — changing them requires updating both the AI prompt and FHIRMapper.
type LabReport struct {
    PatientIdentifier string          `json:"patient_identifier"` // name+DOB from doc; for matching only, not stored
    CollectionDate    time.Time       `json:"collection_date"`
    ReportDate        time.Time       `json:"report_date"`
    LabFacility       string          `json:"lab_facility"`
    OrderingProvider  string          `json:"ordering_provider"`
    PanelName         string          `json:"panel_name"`
    Analytes          []AnalyteResult `json:"analytes"`
    RawText           string          `json:"raw_text"` // preserved for audit; not stored in DB
}

type AnalyteResult struct {
    Name           string  `json:"name"`
    Value          string  `json:"value"`
    Unit           string  `json:"unit"`
    ReferenceRange string  `json:"reference_range"`
    Abnormal       bool    `json:"abnormal"`
    Confidence     float64 `json:"confidence"` // 0.0–1.0 from AI step
    // TGV/MADO: ICD-10 code inferred by AI for positive abnormal results.
    // Required for MADO evaluation in Session 04.
    // Empty string if not applicable or not determinable.
    ICD10Code      string  `json:"icd10_code,omitempty"`
}
```

### 3. AI Extractor (`internal/parser/pkg/pipeline/ai_extractor.go`)

```go
type OCRAIExtractor struct {
    APIURL  string
    APIKey  string
    Model   string
    Timeout time.Duration
    log     log.Logger // injected; never zerolog directly
}
```

**Prompt design** (hardcoded in the extractor):
- Concatenate OCR text from all pages
- Instruct the model to return **only** valid JSON matching the `LabReport` schema — no markdown, no preamble
- Include the full LabReport JSON schema in the prompt as an example
- For each AnalyteResult: if the result is `abnormal: true`, attempt to assign an ICD-10 code
- If a field cannot be determined: use `""` for strings, `0.0` for confidence
- French and English terms are both acceptable in input (bilingual Canadian labs)

**Retry logic**: on HTTP non-200 or JSON parse failure: retry up to 2 times, backoff 1s → 2s.
Log each retry at warn level via `github.com/WillerTravassos/showcase/pkg/log`.

### 4. Confidence Validator (`internal/parser/pkg/pipeline/validator.go`)

Threshold configurable via `PARSER_CONFIDENCE_THRESHOLD` env (default 0.85).

```go
type ValidationResult struct {
    Valid        bool
    NeedsReview  bool
    FailedFields []string
}

// Validate checks all critical fields against the threshold.
// Critical fields: PatientIdentifier (non-empty), CollectionDate (non-zero),
// PanelName (non-empty), each AnalyteResult.Value confidence >= threshold.
func Validate(report LabReport, threshold float64) ValidationResult
```

### 5. FHIR Mapper (`internal/parser/pkg/pipeline/fhir_mapper.go`)

```go
type DiagnosticReportBundle struct {
    Report       fhir.DiagnosticReport
    Observations []fhir.Observation
}

// MapToFHIR converts a LabReport to a FHIR DiagnosticReport + Observations.
// appointmentID is used as the Encounter reference.
// ICD-10 codes from AnalyteResult.ICD10Code are set on Observation.code.coding[].
// This enables MADO evaluation in the DAG service (session 04).
func MapToFHIR(report LabReport, appointmentID uuid.UUID) (DiagnosticReportBundle, error)
```

New FHIR types needed (`pkg/fhir/diagnostic.go`). Named types for bounded-value fields:

```go
type DiagnosticReportStatus string

const (
    DiagnosticReportStatusFinal       DiagnosticReportStatus = "final"
    DiagnosticReportStatusPreliminary DiagnosticReportStatus = "preliminary"
)

type ObservationStatus string

const (
    ObservationStatusFinal       ObservationStatus = "final"
    ObservationStatusPreliminary ObservationStatus = "preliminary"
)

type DiagnosticReport struct {
    ResourceType      string                 `json:"resourceType"` // "DiagnosticReport"
    ID                string                 `json:"id"`
    Status            DiagnosticReportStatus `json:"status"`
    Code              CodeableConcept        `json:"code"`
    Subject           Reference              `json:"subject"`
    Encounter         Reference              `json:"encounter"`
    EffectiveDateTime time.Time              `json:"effectiveDateTime"`
    Issued            time.Time              `json:"issued"`
    Performer         []Reference            `json:"performer"`
    Result            []Reference            `json:"result"`
}

type Observation struct {
    ResourceType   string            `json:"resourceType"` // "Observation"
    ID             string            `json:"id"`
    Status         ObservationStatus `json:"status"`
    Code           CodeableConcept   `json:"code"` // includes ICD-10 coding if present
    Subject        Reference         `json:"subject"`
    ValueQuantity  *Quantity         `json:"valueQuantity,omitempty"`
    ValueString    string            `json:"valueString,omitempty"`
    Interpretation []CodeableConcept `json:"interpretation,omitempty"`
    ReferenceRange []ReferenceRange  `json:"referenceRange,omitempty"`
}
```

### 6. Backend Submission (`internal/parser/pkg/pipeline/submitter.go`)

```go
type BackendClient interface {
    // SubmitResults POSTs the DiagnosticReportBundle to POST /api/v1/appointments/:id/results
    SubmitResults(ctx context.Context, apptID uuid.UUID, bundle DiagnosticReportBundle) error
}

type HTTPBackendClient struct {
    BaseURL    string
    AuthToken  string // service-account JWT; rotated via config
    HTTPClient *http.Client
    log        log.Logger
}
```

On success: publish `appointments.results_received` to NATS.

### 7. Needs-Review Queue (`internal/parser/pkg/pipeline/review.go`)

On validation failure, publish to NATS subject `parser.review`:

```json
{
  "trace_id": "...",
  "appointment_id": "...",
  "file_reference": "...",
  "failed_fields": ["CollectionDate", "Analytes[2].Value"],
  "partial_report": { ... }
}
```

Low-confidence fields are surfaced in Admin Portal session 14 for manual correction.

### 8. Complete Pipeline (`internal/parser/pkg/pipeline/pipeline.go` — extend session 08)

```go
type PipelineDeps struct {
    Storage    ObjectStorage
    Rasteriser Rasteriser
    OCR        OCREngine
    Extractor  ReportExtractor
    Backend    BackendClient
    Queue      queue.Publisher  // pkg/queue interface
    Threshold  float64
    Log        log.Logger
}

// RunFullPipeline orchestrates all 5 stages.
// On needs_review: publishes to parser.review, does NOT submit to backend.
// On success: submits to backend, publishes results_received, deletes PDF from storage.
func RunFullPipeline(ctx context.Context, msg InboundMessage, deps PipelineDeps) error
```

---

## Tests Required

- `TestOCRAIExtractor_parsesValidResponse` — mock HTTP server returns valid JSON
- `TestOCRAIExtractor_retriesOnHTTP500` — mock returns 500 twice then 200; 3 total calls
- `TestOCRAIExtractor_setsICD10CodeOnAbnormalResult`
- `TestValidate_allAboveThreshold_valid`
- `TestValidate_lowConfidenceAnalyte_needsReview_withFieldName`
- `TestValidate_emptyPatientIdentifier_needsReview`
- `TestMapToFHIR_correctObservationCount` — 3 analytes → 3 Observations
- `TestMapToFHIR_icd10CodeInObservationCoding` — ICD-10 code propagated to FHIR Observation
- `TestMapToFHIR_abnormalFlagSetsInterpretation`
- `TestRunFullPipeline_successPath_submitsToBackend`
- `TestRunFullPipeline_needsReview_publishesToReviewQueue_notToBackend`
- `TestRunFullPipeline_deletesPDFOnSuccess`

---

## Done Criteria

- Full pipeline runs end-to-end with mock dependencies
- ICD-10 codes present on Observations for abnormal results (required by MADO evaluator)
- needs_review path publishes to `parser.review` without submitting to backend
- No direct zerolog anywhere — only `github.com/WillerTravassos/showcase/pkg/log`
- `go build ./cmd/parser` and `go vet ./...` pass
