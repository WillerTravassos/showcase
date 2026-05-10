# Overview: Medical Test Parser

> Reference this file for sessions 08–09.

---

## Binary Entrypoint

`cmd/parser/main.go` — connects to NATS, subscribes to `parser.inbound`, and processes messages concurrently (worker pool, configurable size, default 4).

The parser is a **separate binary** with no HTTP server. It communicates outbound only via the internal backend API (HTTP) and NATS.

---

## Pipeline Stages

```
NATS message (parser.inbound)
        │
        ▼
  [1] Download PDF from object storage
        │
        ▼
  [2] Rasterise pages → PNG @ 300 DPI (poppler / pdftoppm)
        │
        ▼
  [3] OCR each page → raw text (Tesseract, lang: eng+fra)
        │
        ▼
  [4] AI extraction → structured LabReport JSON
        │
        ▼
  [5] Confidence validation
       ├── All fields >= threshold → POST to backend API
       └── Any field < threshold  → mark needs_review, publish to review queue
        │
        ▼
  [6] Emit results.received to NATS (on success)
```

---

## Inbound Message Schema

```json
{
  "trace_id": "uuid",
  "occurred_at": "RFC3339",
  "schema_version": "1",
  "payload": {
    "appointment_id": "uuid",
    "uploaded_by": "uuid",
    "file_reference": "object-storage-key"
  }
}
```

---

## LabReport — The Central Contract

This struct is the interface between the AI extraction stage and the FHIR mapping stage. **Do not change field names without updating the AI prompt and the FHIR mapper.**

```go
// internal/parser/labreport.go

type LabReport struct {
    PatientIdentifier string           `json:"patient_identifier"` // name+DOB from doc, for matching only
    CollectionDate    time.Time        `json:"collection_date"`
    ReportDate        time.Time        `json:"report_date"`
    LabFacility       string           `json:"lab_facility"`
    OrderingProvider  string           `json:"ordering_provider"`
    PanelName         string           `json:"panel_name"`
    Analytes          []AnalyteResult  `json:"analytes"`
    RawText           string           `json:"raw_text"`   // preserved for audit
}

type AnalyteResult struct {
    Name           string   `json:"name"`
    Value          string   `json:"value"`
    Unit           string   `json:"unit"`
    ReferenceRange string   `json:"reference_range"`
    Abnormal       bool     `json:"abnormal"`
    Confidence     float64  `json:"confidence"` // 0.0–1.0
}
```

---

## Confidence Threshold

- Configurable via `PARSER_CONFIDENCE_THRESHOLD` env var (default `0.85`)
- Applied per-analyte and per-header-field
- If **any** critical field (PatientIdentifier, CollectionDate, PanelName, or any Analyte.Value) is below threshold → entire report is `needs_review`
- `needs_review` reports are published to NATS subject `parser.review` for manual processing via the Admin Portal

---

## FHIR Mapping (LabReport → DiagnosticReport)

```
LabReport.PanelName       → DiagnosticReport.code
LabReport.CollectionDate  → DiagnosticReport.effectiveDateTime
LabReport.ReportDate      → DiagnosticReport.issued
LabReport.LabFacility     → DiagnosticReport.performer[0].display
LabReport.Analytes[]      → DiagnosticReport.result[] (references to Observations)

Each AnalyteResult maps to an Observation:
  .Name                   → Observation.code.text
  .Value + .Unit          → Observation.valueQuantity
  .ReferenceRange         → Observation.referenceRange[0].text
  .Abnormal               → Observation.interpretation (HI/LO/A)
```

---

## Future API Migration Path

The pipeline is designed so that Stage 3 (OCR) and Stage 4 (AI extraction) are behind an interface:

```go
type ReportExtractor interface {
    Extract(ctx context.Context, pages [][]byte) (LabReport, error)
}
```

Current implementation: `OCRAIExtractor`. Future: `LabAPIExtractor` per lab partner. The FHIR mapping and backend submission stages are unchanged.
