# Session 08 — Test Parser: OCR Stage

## Claude Code Delegation Prompt

```
You are implementing Session 08 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — package layout, no business logic in cmd/
  4. specs/conventions/tooling.md — github.com/WillerTravassos/showcase/pkg/log, never zerolog directly
  5. specs/overview/03-parser.md — pipeline stages, LabReport contract

Session 01 is complete (pkg/queue, pkg/log exist).

Your job is to build the parser binary skeleton and the first three pipeline stages:
NATS inbound subscription, PDF download from S3, rasterisation via poppler, and OCR
via Tesseract. Output is raw text per page, ready for Session 09.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./cmd/parser && go vet ./...
```

---

## Dependencies
- Session 01 (`pkg/queue`, `github.com/WillerTravassos/showcase/pkg/log`)

---

## Package Location

```
internal/parser/
├── app/
│   ├── service.go          # package app — composition root, starts the parser binary
│   ├── settings.go         # env-var config struct for the parser service
│   └── messageingress/
│       └── inbound.go      # NATS durable consumer; dispatches to pipeline
└── pkg/
    └── pipeline/           # TODO: domain packages TBD before implementation starts
        ├── pipeline.go     # Orchestration types and RunOCRStages
        ├── storage.go      # ObjectStorage interface + S3 implementation
        ├── rasteriser.go   # PDF → PNG pages
        ├── ocr.go          # PNG → raw text
        └── runlog.go       # Structured run logging
cmd/parser/
└── main.go                 # Calls app.Start() only — no logic here
```

---

## Deliverables

### 1. Config (`internal/parser/app/settings.go`)

```go
package app

type Settings struct {
    NATSUrl              string        `env:"NATS_URL,required"`
    S3Bucket             string        `env:"S3_BUCKET,required"`
    S3Region             string        `env:"S3_REGION,required"`
    BackendURL           string        `env:"BACKEND_URL,required"`
    BackendToken         string        `env:"BACKEND_SERVICE_TOKEN,required"`
    WorkerCount          int           `env:"PARSER_WORKER_COUNT" envDefault:"4"`
    LogLevel             string        `env:"LOG_LEVEL"           envDefault:"info"`
}
```

### 2. Parser Binary (`cmd/parser/main.go` + `internal/parser/app/service.go`)

`cmd/parser/main.go` calls `app.Start()` only — no logic in `cmd/`.

`internal/parser/app/service.go`:
- Load `Settings`
- Init logger via `github.com/WillerTravassos/showcase/pkg/log` — never zerolog directly
- Connect to NATS (`pkg/queue`); subscribe to `parser.inbound` with durable consumer `parser-worker`
- Worker pool: size from `Settings.WorkerCount`
- Each message dispatched via `messageingress.InboundConsumer`
- Graceful shutdown on SIGTERM: drain in-flight workers, ACK pending messages, exit

### 3. Inbound Message Schema

```go
// internal/parser/pkg/pipeline/pipeline.go
type InboundMessage struct {
    TraceID       string    `json:"trace_id"`
    OccurredAt    time.Time `json:"occurred_at"`
    SchemaVersion string    `json:"schema_version"`
    Payload       struct {
        AppointmentID uuid.UUID `json:"appointment_id"`
        UploadedBy    uuid.UUID `json:"uploaded_by"`
        FileReference string    `json:"file_reference"` // object storage key
    } `json:"payload"`
}
```

### 4. Object Storage (`internal/parser/pkg/pipeline/storage.go`)

```go
type ObjectStorage interface {
    Download(ctx context.Context, key string) ([]byte, error)
    Delete(ctx context.Context, key string) error
}

// S3Storage implements ObjectStorage using AWS SDK v2.
// Bucket name and region from config.
type S3Storage struct { /* ... */ }
```

After successful pipeline completion, the parser deletes the source PDF from the
inbound bucket (per `deploy/cnpg` bucket lifecycle policy — belt-and-suspenders).

### 5. PDF Rasteriser (`internal/parser/pkg/pipeline/rasteriser.go`)

```go
type Rasteriser interface {
    // Rasterise converts each page of a PDF to a 300 DPI PNG.
    // Returns one []byte per page. Order preserved.
    Rasterise(ctx context.Context, pdfBytes []byte) ([][]byte, error)
}

// PopplerRasteriser uses pdftoppm as a subprocess.
type PopplerRasteriser struct {
    BinPath string // default "pdftoppm" on PATH
}
```

Implementation:
- Write PDF to `os.CreateTemp`
- Run: `pdftoppm -r 300 -png {input} {output_prefix}`
- Read all `{prefix}-*.png` output files in numeric order
- Delete temp files unconditionally (defer)
- On non-zero exit: return error including stderr content
- Log at debug level: page count, duration (via `github.com/WillerTravassos/showcase/pkg/log` — not fmt.Println)

### 6. OCR Engine (`internal/parser/pkg/pipeline/ocr.go`)

```go
type OCREngine interface {
    // ExtractText runs Tesseract OCR on a single page image PNG.
    ExtractText(ctx context.Context, imageBytes []byte) (string, error)
}

// TesseractOCR uses the gosseract Go bindings.
// Languages: "eng+fra" — required for bilingual Canadian lab documents (TGV).
type TesseractOCR struct {
    Languages string // default "eng+fra"
}
```

### 7. Pipeline Stages 1–3 (`internal/parser/pkg/pipeline/pipeline.go`)

```go
type PageText struct {
    PageNumber int
    Text       string
}

type PipelineResult struct {
    AppointmentID uuid.UUID
    UploadedBy    uuid.UUID
    FileReference string
    RawPDFHash    string  // SHA-256 hex of original PDF bytes (for audit trail)
    Pages         []PageText
}

// RunOCRStages executes: download → rasterise → OCR.
// Returns PipelineResult with raw extracted text per page.
func RunOCRStages(
    ctx    context.Context,
    msg    InboundMessage,
    store  ObjectStorage,
    rast   Rasteriser,
    ocr    OCREngine,
) (PipelineResult, error)
```

`RawPDFHash` is computed immediately after download, before rasterisation, and included
in every RunLog for audit trail integrity.

### 8. Run Log (`internal/parser/pkg/pipeline/runlog.go`)

All runs produce a structured log entry via `github.com/WillerTravassos/showcase/pkg/log`. Not stored in DB —
captured by Loki via structured JSON output.

```go
type RunLog struct {
    TraceID       string
    AppointmentID uuid.UUID
    FileReference string
    PDFHash       string
    Stages        []StageLog
    Outcome       string        // success | needs_review | failed
    Duration      time.Duration
}

type StageLog struct {
    Stage    string
    Success  bool
    ErrorMsg string
    Duration time.Duration
}

func (r RunLog) Log(logger log.Logger)
```

---

## Tests Required

- `TestRasteriser_outputsOnePNGPerPage` — 2-page fixture PDF → 2 PNG byte slices
- `TestRasteriser_invalidPDF_returnsError`
- `TestRasteriser_tempFilesCleanedUp` — no temp files after success or failure
- `TestTesseractOCR_extractsKnownText` — fixture PNG with known text → correct string
- `TestRunOCRStages_threePageDocument` — mock store + rasteriser + OCR → PipelineResult with 3 pages
- `TestRunOCRStages_storageFailure_returnsError`
- `TestRunOCRStages_computesPDFHash`

---

## Done Criteria

- Parser binary starts, subscribes to NATS `parser.inbound` with durable consumer
- OCR pipeline extracts text from multi-page scanned PDFs
- No direct zerolog or fmt.Println in any file — only `github.com/WillerTravassos/showcase/pkg/log`
- `go build ./cmd/parser` and `go vet ./...` pass
