# PII Redactor

A high-throughput, extensible PII redaction engine, split into two services:

- **Go engine** (`cmd/`, `internal/`) — the API gateway. Detects structured
  PII with regex + validity checks, generates deterministic fake
  replacements, and redacts `.docx` files in place — both text and
  embedded images.
- **Python NLP worker** (`python-worker/`) — detects unstructured PII in
  text (person names, company names, physical addresses) via Presidio/spaCy
  NER, and PII embedded in images (scanned IDs, screenshots) via OCR —
  served to the Go engine over gRPC.

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

Images are a separate surface with no equivalent XML text to redact —
there is no `Detector` for them. Instead, `docxio` extracts every embedded
image, sends its raw bytes to the Python worker for OCR-based redaction,
and splices the result back into the `.docx` package directly at the ZIP
level (see [Image-embedded PII](#image-embedded-pii)).

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
  docxio/                    Redacts .docx files by splicing raw ZIP/XML
                              bytes directly — text (xmltext.go) and images
                              (images.go); see Text redaction fidelity below
                              for why this doesn't use docxgo's object model
  grpcclient/                NLPClient: NoOpClient (structured-only
                              fallback), GRPCClient (talks to the worker),
                              DedupingClient (collapses redundant calls —
                              see Performance below)
  api/                       HTTP handler: POST /redact/docx
  config/                    Environment-based configuration
proto/
  redactor.proto              gRPC contract: Analyze (text) + RedactImage
  gen/redactor/                Generated Go stubs
python-worker/
  redactor/
    analyzer.py                 Presidio AnalyzerEngine + custom recognizers
    recognizers.py               ORG (spaCy NER + legal-suffix pattern) and
                                 ADDRESS (street-type pattern) recognizers
    image_redactor.py           OCR (pytesseract) + pixel masking (Pillow)
    service.py                  gRPC service: Analyze + RedactImage
  main.py                     Worker entrypoint
  gen/                        Generated Python stubs
```

## Precision vs. recall

The PII types split into three pipelines, matched to how checkable each
one actually is.

**Structured text data (Go, regex + a real validity check):**

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

**Unstructured text data (Python, NER + rule-based heuristics):** names,
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

## Image-embedded PII

A `.docx` can carry PII inside a pixel array instead of text — a scanned
ID, a screenshot, a photographed form — where there's no XML string to
find or replace. That's a different pipeline end to end:

```
Go: extract word/media/*.png from the .docx ZIP
        │
        ▼
Python: OCR (pytesseract) — words + bounding boxes + confidence
        │
        ▼
Python: group words into lines, run PresidioAnalyzer over each line's text
        │
        ▼
Python: map matched character spans back to the word boxes they cover,
        union overlapping boxes, draw solid black rectangles (Pillow)
        │
        ▼
Go: splice the redacted image bytes back into the .docx ZIP,
    every other part of the package untouched
```

`docxio` extracts and replaces images at the raw ZIP layer rather than
through `docxgo`, because `docxgo` can read an image's bytes (`Data()`)
but has no method to replace them. An image's relationship ID and target
path never change — only its pixel content — so this never needs to touch
or understand OOXML relationships.

Redaction is a **solid black fill**, never blur or pixelation: both of
those are frequently reconstructible with the right tooling, a solid fill
is not.

OCR text is also checked against Presidio's built-in structured
recognizers (email, phone, credit card, SSN, IP) in addition to
PERSON/ORG/ADDRESS — Go's regex detectors never see image bytes at all,
since they never round-trip through the Go text pipeline, so this is the
only place that catches structured PII embedded in an image.

## Text redaction fidelity

`docxio` redacts the visible text of a `.docx` (the document body and
every header/footer) by splicing directly into the raw bytes of
`word/document.xml`/`word/header*.xml`/`word/footer*.xml` at the exact
byte range of each `<w:t>` element's content — never through docxgo's
typed object model, and never by decoding into generic XML tokens and
re-encoding them either. Both alternatives were tried and both corrupt
real documents, for different reasons:

- **docxgo's object model** represents bold, italic, color, underline,
  and highlight, and text redacted through it preserves all of those
  correctly. But paragraph/run shading (`<w:shd>`, e.g. a colored
  background) and any other OOXML feature outside that model has no field
  to round-trip through — it's silently dropped on **any** read+write pass,
  even for a paragraph redaction never touches, because the reader has
  nowhere to put it and the writer has nothing to serialize it from.
- **Go's `encoding/xml`**, used the "obvious" way — decode the whole
  document into generic tokens, mutate the ones you care about, re-encode
  the rest — has its own failure mode: `xml.Encoder` does not preserve the
  original namespace prefixes of tokens it re-serializes. It invents its
  own, differently for every element, turning `<w:t xml:space="preserve">`
  into something like `<t xmlns="..." xmlns:main="...">` — a different,
  likely Word-incompatible, and definitely far uglier document.

Splicing into the original bytes at exactly the offsets `<w:t>` content
occupies (found via `xml.Decoder`, used purely as a tokenizer — never via
`xml.Encoder`) sidesteps both failure modes: every byte outside a value
actually being redacted, known formatting or not, passes through
unchanged, because nothing is ever reconstructed.

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

## Performance

A real `.docx` can have hundreds of runs (Word splits paragraphs into many
runs across edits, spell-check, and formatting changes), and each
unstructured-PII run triggers three calls to the NLP worker — one per
entity type (PERSON, ORG, ADDRESS) — for the exact same text. Naively,
that's `runs × 3` network round trips.

`internal/docxio` redacts every run (and every embedded image) **concurrently**,
bounded by `maxConcurrentRedactions` (16 in flight), rather than one at a
time. `internal/grpcclient.DedupingClient` then eliminates the ×3
redundancy two ways:

- `singleflight` collapses calls that are genuinely concurrent.
- An LRU cache (separate ones for text and images, images keyed by a hash
  of their bytes) catches everything singleflight can't — a real RPC round
  trip is often faster than the scheduling jitter between three goroutines,
  so in practice they rarely overlap enough for singleflight alone to
  merge them. The image cache also means a repeated letterhead or logo
  across every page of a scanned document is only ever OCR'd once.

Measured on a synthetic 4800-run stress document: **56.5s → 11.7s** from
concurrency + caching combined, with real gRPC calls to the worker dropping
from ~15,000 to ~1,600.

## Running it

```bash
docker compose up --build
```

This builds and starts both services: the Python worker on `:50051`
(healthchecked — the Go container waits for it) and the Go API on
`:8080`, wired together via `NLP_WORKER_ADDR=python-worker:50051`.

To run the Go engine alone, with structured text detection only (no
unstructured text PII, no image redaction):

```bash
go run ./cmd                     # listens on :8080, NLP_WORKER_ADDR unset -> NoOpClient
PORT=9090 go run ./cmd           # override the port
```

To run the Python worker alone (requires the `tesseract` binary installed
locally — `tesseract-ocr` on Debian/Ubuntu, `brew install tesseract` on
macOS):

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
detects structured text PII only and leaves images untouched).

Python worker environment variables: `GRPC_PORT` (default `50051`),
`SPACY_MODEL` (default `en_core_web_sm`), `GRPC_MAX_WORKERS` (default `10`).

### API

**Redact a `.docx` file** (paragraphs, table cells, and embedded images —
formatting preserved):

```bash
curl -X POST localhost:8080/redact/docx \
  -F "file=@input.docx" -o redacted.docx
```

Every stage logs progress — file received, redaction started, each gRPC
call to the worker with entity/region counts and timing, done — so
`docker compose logs -f` shows exactly where a request is at any moment.

## Testing

Go engine:

```bash
go build ./...
go vet ./...
go test ./... -race
```

Every detector, the faker cache, the processor, `docxio` (text and image
ZIP surgery), the gRPC client (including the dedup/cache layer, proven
under `-race`), and the API handler have unit tests. The gRPC client tests
run against an in-process server (`bufconn`), so none of it needs a live
Python worker to test.

Python worker:

```bash
cd python-worker
source .venv/bin/activate
python -m pip install -r requirements-dev.txt
python -m pytest
```

Covers the analyzer (entity detection, offset correctness, the
single-token PERSON precision filter), the OCR image redactor (real
rendered-text images redacted and re-OCR'd to confirm the PII is actually
gone while surrounding text survives), and the gRPC service layer (via
fakes, so it doesn't need a real spaCy model or Tesseract for most cases).

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
