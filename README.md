# PII Redactor

A high-throughput, extensible PII redaction engine, split into two services:

- **Go engine** (`cmd/`, `internal/`) — the API gateway. Detects structured
  PII with regex + validity checks, generates deterministic fake
  replacements, and redacts `.docx` files in place.
- **Python NLP worker** (`python-worker/`) — detects unstructured PII
  (person names, company names, physical addresses) using Presidio/spaCy
  NER, served to the Go engine over gRPC.

## Architecture

Every PII category is a standalone `Detector` implementation (the Strategy
pattern). The core pipeline never branches on PII type — it runs every
registered detector concurrently and merges the results, whether that
detector is a regex check running in-process or a gRPC call to the Python
worker. Adding a new PII type means writing one new struct and registering
it; nothing else changes.

```go
type Detector interface {
    Detect(text string) []Match
    Type() PIIType
}
```

```
cmd/main.go                 Wires detectors + processor + API into an HTTP server
internal/
  detector/                  Strategy pattern: one file per PII type
    detector.go                Detector interface, PIIType, Match
    email.go, ip.go,           Structured detectors: regex + a validity check
    ssn.go, creditcard.go,     (Luhn, SSA rules, net.ParseIP, libphonenumber,
    phone.go, dob.go           calendar validation) — never NLP for these
    nlp.go                     Adapts a grpcclient.NLPClient to Detector, for
                               names/companies/addresses via the Python worker
  faker/                     Deterministic, thread-safe fake-value cache
  processor/                 Runs detectors concurrently, resolves overlaps,
                              applies replacements
  docxio/                    Redacts .docx files in place via docxgo
  grpcclient/                NLPClient interface: NoOpClient (structured-only
                              fallback) and GRPCClient (talks to the worker)
  api/                       HTTP handler: POST /redact/docx
  config/                    Environment-based configuration
proto/
  redactor.proto              gRPC contract between the Go engine and the worker
  gen/redactor/                Generated Go stubs
python-worker/
  redactor/
    analyzer.py                 Presidio AnalyzerEngine + custom recognizers
    recognizers.py               ORG (spaCy NER + legal-suffix pattern) and
                                 ADDRESS (street-type pattern) recognizers
    service.py                  gRPC service, translates Entity <-> protobuf
  main.py                     Worker entrypoint
  gen/                        Generated Python stubs
```

## Precision vs. recall

The nine PII types split into two pipelines, matched to how checkable
each one actually is.

**Structured data (Go, regex + a real validity check):**

| Type | Candidate scan | Validity check |
|---|---|---|
| Email | regex | — (email syntax *is* the check) |
| Phone | regex | [`nyaruka/phonenumbers`](https://github.com/nyaruka/phonenumbers) (Go port of libphonenumber) |
| SSN | `AAA-GG-SSSS` regex | SSA allocation rules (no area 000/666/900-999, no group/serial 00/0000) |
| Credit card | 13–19 digit regex | Luhn checksum |
| IP address | loose digit/hex regex | `net.ParseIP` |
| Date of birth | numeric + written-month regex | `time.Parse` against real calendar layouts, year within a plausible lifetime |

A 16-digit order number that isn't Luhn-valid, or a "555" area code that
doesn't exist in the NANP, is rejected — a regex alone would have kept it.

**Unstructured data (Python, NER + rule-based heuristics):** names,
company names, and physical addresses don't follow a checkable grammar, so
they go through spaCy's NER model via Presidio:

| Type | Signal |
|---|---|
| Person name | spaCy `PERSON` NER, filtered to require 2+ tokens (a full name, not a stray single-word false positive) |
| Company | spaCy `ORG` NER **and** a legal-suffix pattern (`Inc`, `LLC`, `Corp`, `GmbH`, ...) — either is enough, both boosts confidence |
| Address | number + capitalized words + street-type word (`Street`, `Ave`, `Suite`, ...) — spaCy has no native address entity, so this one is pure pattern matching |

The worker defaults to `en_core_web_sm` (small, fast, small Docker image).
Swapping in `en_core_web_lg` or a transformer pipeline for better recall is
a one-line change in `python-worker/redactor/analyzer.py` — nothing on the
Go side, or the gRPC contract, needs to change.

## Deterministic replacement

`internal/faker.Cache` maps each original PII value to a fake replacement,
generated once and reused for every subsequent occurrence — so "Rashi
Patil" or a repeated SSN always redacts to the same fake value throughout
a document, whether it was found by a Go regex detector or the Python NLP
worker. The mapping is also reproducible from a cold cache: the fake
generator is seeded from a hash of the original value, not from insertion
order.

Each HTTP request gets its own `faker.Cache`, so fake-value consistency is
scoped to one document — it never leaks across unrelated requests. Fake
values across different PII types are generated independently: a fake name
and a fake email in the same document are not linked to look like the same
fake person — that would require entity linking, which this engine doesn't
attempt.

## Running it

```bash
docker compose up --build
```

This builds and starts both services: the Python worker on `:50051`
(healthchecked — the Go container waits for it) and the Go API on
`:8080`, wired together via `NLP_WORKER_ADDR=python-worker:50051`.

To run the Go engine alone, with structured detection only:

```bash
go run ./cmd                     # listens on :8080, NLP_WORKER_ADDR unset -> NoOpClient
PORT=9090 go run ./cmd           # override the port
```

To run the Python worker alone:

```bash
cd python-worker
python3.12 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
python main.py                   # listens on :50051
```

Go engine environment variables: `PORT` (default `8080`),
`PHONE_DEFAULT_REGION` (default `US`, an ISO 3166-1 alpha-2 code used to
interpret phone numbers with no explicit country code), `NLP_WORKER_ADDR`
(the Python worker's `host:port`; unset falls back to `NoOpClient`, which
detects structured PII only).

Python worker environment variables: `GRPC_PORT` (default `50051`),
`SPACY_MODEL` (default `en_core_web_sm`), `GRPC_MAX_WORKERS` (default `10`).

### API

**Redact a `.docx` file** (paragraphs and table cells, formatting preserved):

```bash
curl -X POST localhost:8080/redact/docx \
  -F "file=@input.docx" -o redacted.docx
```

## Testing

Go engine:

```bash
go build ./...
go vet ./...
go test ./... -race
```

Every detector, the faker cache, the processor, `docxio`, the gRPC client,
and the API handler have unit tests, including concurrency (`-race`)
coverage for the faker cache and an in-process gRPC server (`bufconn`) for
the client, so none of it needs a live Python worker to test.

Python worker:

```bash
cd python-worker
source .venv/bin/activate
python -m pip install -r requirements-dev.txt
python -m pytest
```

Covers the analyzer (entity detection, offset correctness, the
single-token PERSON precision filter) and the gRPC service layer (via a
fake analyzer, so it doesn't need a real spaCy model loaded).

## Regenerating the gRPC contract

After editing `proto/redactor.proto`:

```bash
# Go stubs
protoc -I proto \
  --go_out=. --go_opt=module=github.com/kaizakin/PII-redactor \
  --go-grpc_out=. --go-grpc_opt=module=github.com/kaizakin/PII-redactor \
  proto/redactor.proto

# Python stubs
python-worker/scripts/generate_proto.sh
```
