# PII Redactor — Go Engine

A high-throughput, extensible PII redaction engine. This phase covers the
**Go engine**: structured PII detection, deterministic fake-value
replacement, plain-text and `.docx` redaction, and an HTTP API. Unstructured
PII (names, company names, physical addresses) is architected for but not
yet wired to a live NLP backend — see [What's not here yet](#whats-not-here-yet).

## Architecture

Every PII category is a standalone `Detector` implementation (the Strategy
pattern). The core pipeline never branches on PII type — it runs every
registered detector concurrently and merges the results. Adding a new PII
type means writing one new struct and registering it; nothing else changes.

```go
type Detector interface {
    Detect(text string) []Match
    Type() PIIType
}
```

```
cmd/server/main.go        Wires detectors + processor + API into an HTTP server
internal/
  detector/                Strategy pattern: one file per PII type
    detector.go              Detector interface, PIIType, Match
    email.go, ip.go,         Structured detectors: regex + a validity check
    ssn.go, creditcard.go,   (Luhn, SSA rules, net.ParseIP, libphonenumber,
    phone.go, dob.go         calendar validation) — never NLP for these
    nlp.go                   Adapts a grpcclient.NLPClient to Detector, for
                             names/companies/addresses once the Python
                             worker exists
  faker/                   Deterministic, thread-safe fake-value cache
  processor/               Runs detectors concurrently, resolves overlaps,
                            applies replacements
  docxio/                  Redacts .docx files in place via docxgo
  grpcclient/              NLPClient interface + NoOpClient stand-in for
                            the future Python worker
  api/                     HTTP handlers: /api/v1/redact, /api/v1/redact/docx
  config/                  Environment-based configuration
proto/redactor.proto       Reference gRPC schema for the Go <-> Python contract
```

## Precision vs. recall

Structured PII follows strict, checkable rules, so every structured
detector pairs a regex *candidate* scan with a real validity check —
that's where precision comes from:

| Type | Candidate scan | Validity check |
|---|---|---|
| Email | regex | — (email syntax *is* the check) |
| Phone | regex | [`nyaruka/phonenumbers`](https://github.com/nyaruka/phonenumbers) (Go port of libphonenumber) |
| SSN | `AAA-GG-SSSS` regex | SSA allocation rules (no area 000/666/900-999, no group/serial 00/0000) |
| Credit card | 13–19 digit regex | Luhn checksum |
| IP address | loose digit/hex regex | `net.ParseIP` |
| Date of birth | numeric + written-month regex | `time.Parse` against real calendar layouts, year within a plausible lifetime |

A 16-digit order number that isn't Luhn-valid, or a "555" area code that
doesn't exist in the NANP, is rejected — the regex alone would have kept it.

## Deterministic replacement

`internal/faker.Cache` maps each original PII value to a fake replacement,
generated once and reused for every subsequent occurrence — so "Rashi
Patil" (once name detection is live) or a repeated SSN always redacts to
the same fake value throughout a document. The mapping is also
reproducible from a cold cache: the fake generator is seeded from a hash of
the original value, not from insertion order.

Each HTTP request gets its own `faker.Cache`, so fake-value consistency is
scoped to one document — it never leaks across unrelated requests.

## Running it

```bash
go run ./cmd/server              # listens on :8080 by default
PORT=9090 go run ./cmd/server    # override the port

docker build -t pii-redactor .
docker run -p 8080:8080 pii-redactor
```

Environment variables: `PORT` (default `8080`), `PHONE_DEFAULT_REGION`
(default `US`, an ISO 3166-1 alpha-2 code used to interpret phone numbers
with no explicit country code), `NLP_WORKER_ADDR` (reserved for the
Python worker, unused today).

### API

**Redact plain text:**

```bash
curl -X POST localhost:8080/api/v1/redact \
  -H 'Content-Type: application/json' \
  -d '{"text": "Reach Jane at jane@example.com or 523-45-6789."}'
```

```json
{
  "redacted_text": "Reach Jane at aidalarson@cummings.com or 438-77-2390.",
  "matches": [
    {"type": "EMAIL", "start": 14, "end": 30},
    {"type": "SSN", "start": 34, "end": 45}
  ]
}
```

Match offsets refer to the *original* text; the original PII value itself
is never echoed back.

**Redact a `.docx` file** (paragraphs and table cells, formatting preserved):

```bash
curl -X POST localhost:8080/api/v1/redact/docx \
  -F "file=@input.docx" -o redacted.docx
```

**Health check:** `GET /healthz`

## Testing

```bash
go build ./...
go vet ./...
go test ./... -race
```

Every detector, the faker cache, the processor, `docxio`, and the API
handlers have unit tests, including concurrency (`-race`) coverage for the
faker cache and end-to-end coverage for both the text and `.docx` API
paths.

## What's not here yet

Unstructured PII — person names, company names, physical addresses —
needs NER, not regex, so it's routed through `internal/grpcclient.NLPClient`
to a separate Python worker (Microsoft Presidio or a HuggingFace model)
over gRPC. `proto/redactor.proto` defines that contract, and
`internal/detector.NLPDetector` already adapts any `NLPClient` to the same
`Detector` interface every structured detector implements. Today it's
wired up with `grpcclient.NoOpClient`, which reports no entities — so the
engine runs correctly end to end with structured detection only. Standing
up the Python worker and pointing `NLP_WORKER_ADDR` at it is the only
remaining step; no Go code changes.
