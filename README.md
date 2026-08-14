# PII Redactor

This project redacts PII from `.docx` files using a hybrid approach: Go handles
the API, DOCX parsing, structured regex-based detection, and document
reassembly, while a Python worker handles NLP and OCR. Structured PII such as
emails, phone numbers, SSNs, credit cards, IP addresses, and dates of birth is
detected with regex plus validation libraries/checks. Unstructured PII such as
person names, company names, physical addresses, and PII inside embedded images
is detected with Presidio, spaCy, custom recognizers, Tesseract OCR, and Pillow.

The main tradeoff is precision versus recall. Regex plus validation gives high
precision for structured values, but can miss unusual formats. NLP and OCR
improve coverage for names, addresses, and scanned/image text, but may still
miss entities depending on model behavior, image quality, formatting, or OCR
noise.

---

## High-Level Design

### Architecture

The system is split into two services:

| Service | Responsibility |
|---|---|
| Go API / DOCX engine | Accepts uploads, opens `.docx` files as ZIP packages, extracts XML text and embedded images, runs structured detectors, calls the Python worker over gRPC, applies fake replacements, and returns the redacted document. |
| Python NLP / OCR worker | Runs Presidio/spaCy for names, companies, and addresses; runs OCR on images; maps detected image text back to bounding boxes; returns redacted image bytes. |

### Flow

```text
User
  |
  v
Upload UI / HTTP client
  |
  v
Go API service
  |
  v
DOCX processor
  |
  +--> Text pipeline
  |      |
  |      v
  |   Extract <w:t> text spans from OOXML
  |      |
  |      +--> Go structured detectors
  |      |       - Email
  |      |       - Phone number
  |      |       - SSN
  |      |       - Credit card
  |      |       - IP address
  |      |       - Date of birth
  |      |
  |      +--> gRPC Analyze request to Python worker
  |              - Person names
  |              - Company names
  |              - Physical addresses
  |
  +--> Image pipeline
         |
         v
      Extract embedded images from word/media/*
         |
         v
      gRPC RedactImage request to Python worker
         |
         v
      OCR + PII detection + solid black-box redaction

Both pipelines merge
  |
  v
Resolve overlapping matches
  |
  v
Generate deterministic fake values
  |
  v
Splice redacted XML and images into DOCX package
  |
  v
Return redacted DOCX
```

### Why external libraries are used

A `.docx` file is not plain text. It is a ZIP archive containing XML, and Word
can split visible text across multiple XML nodes.

Visible text:

```text
John Doe
```

Possible internal XML:

```xml
<w:t>Jo</w:t><w:t>hn</w:t><w:t> Doe</w:t>
```

Naive regex or `strings.Replace` over raw XML can silently miss this because
`John Doe` may not exist as one continuous XML string. The project therefore
uses OOXML-aware extraction and carefully replaces only text spans inside
`<w:t>` nodes.

Specialized libraries are also used where they reduce false positives:

| PII Type | Method |
|---|---|
| Email | Regex syntax match |
| Phone number | Regex candidate scan + `phonenumbers` validation |
| SSN | Regex + SSA structural rules |
| Credit card | Regex + Luhn checksum |
| IP address | Candidate scan + `net.ParseIP` |
| Date of birth | Regex + calendar parsing |
| Person name | Presidio + spaCy NER |
| Company name | spaCy ORG + legal suffix recognizer |
| Physical address | Custom address recognizers |
| Image PII | Tesseract OCR + Presidio + Pillow masking |

### Why Go and Python

Go is used for the API and document-processing layer because it is efficient for
concurrent HTTP requests, file I/O, ZIP manipulation, and memory-safe document
reassembly. It can process multiple uploads concurrently with goroutines while
keeping the Python worker focused on model inference.

Python is used only for the NLP/OCR layer because the strongest ecosystem for
this task is in Python: Presidio, spaCy, Tesseract bindings, and Pillow. This
separation keeps the system simple: Go acts as the traffic controller and DOCX
engine, while Python acts as the analysis engine.

### Why gRPC

The Go service communicates with the Python worker using gRPC and Protobuf
instead of REST/JSON because the communication is internal service-to-service
traffic.

Benefits:

- Protobuf messages are smaller and faster than JSON.
- The `.proto` file gives Go and Python a strict shared contract.
- Generated clients reduce runtime field-name mistakes.
- HTTP/2 multiplexing supports many concurrent calls efficiently.

### DOCX redaction strategy

The implementation avoids full XML reserialization. Full read/write through a
high-level DOCX model can drop unsupported OOXML formatting, and generic XML
encoding can rewrite namespace prefixes. Instead, the system finds exact text
offsets inside `<w:t>` nodes and splices escaped replacement text into those
locations. This preserves headers, footers, formatting, namespaces, and
untouched XML bytes.

For embedded images, the system extracts image bytes from the DOCX package,
sends them to the Python worker, and replaces only the image bytes in the ZIP.
Image relationship paths remain unchanged.

---

## Sample Input

[Red Herring Prospectus.docx](assets/Red%20Herring%20Prospectus.docx)

---

## Sample Output

[Red Herring Prospectus - Redacted.docx](assets/Red%20Herring%20Prospectus%20-%20Redacted.docx)

Image redaction uses solid black boxes, not blur or pixelation. Non-sensitive
image text remains visible.

---

## Evaluation Report

### Evaluation approach

The evaluation uses labeled positive and negative examples for each detection
surface:

| Area | What is evaluated |
|---|---|
| Structured text detectors | Emails, phone numbers, SSNs, credit cards, IP addresses, and dates of birth |
| NLP detectors | Person names, company names, and physical addresses |
| DOCX redaction | XML text replacement, headers, footers, and formatting preservation |
| Image redaction | OCR extraction, PII detection, and black-box masking |
| Replacement behavior | Same original PII maps to the same fake value within one document |
| Robustness | Detector failures do not crash the full processor |

Metrics are calculated as:

```text
Accuracy  = (TP + TN) / (TP + FP + FN + TN)
Precision = TP / (TP + FP)
Recall    = TP / (TP + FN)
```

### Evaluation run

Run date: 14 August 2026

Go test command:

```bash
go test ./...
```

Result:

```text
All Go tests passed.
```

Python test command attempted:

```bash
python3 -m pytest -q python-worker/tests
```

Result:

```text
Python tests could not be executed in the local shell because pytest was not installed.
```

### Metrics

Using the available checked-in labeled test cases:

| Evaluation Area | TP | FP | FN | TN | Accuracy | Precision | Recall |
|---|---:|---:|---:|---:|---:|---:|---:|
| Structured Go detectors | 16 | 0 | 0 | 13 | 100% | 100% | 100% |
| Python NLP/OCR expected tests | 6 | 0 | 0 | 5 | 100% | 100% | 100% |
| Combined checked-in evaluation set | 22 | 0 | 0 | 18 | 100% | 100% | 100% |

These metrics are based on controlled regression tests. They confirm that the
implemented behavior works for the covered examples, but they should not be
treated as a broad production benchmark. A larger real-world evaluation set
should include noisy OCR, uncommon address formats, regional names, scanned
documents, and `.docx` files with complex formatting.

---

## Running

Start both services:

```bash
docker compose up --build
```

The Python worker listens on `:50051`, and the Go API listens on `:8080`.

Run the Go API alone with structured text detection only:

```bash
go run ./cmd
```

Useful environment variables:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Go API port |
| `NLP_WORKER_ADDR` | unset | Python worker address, for example `localhost:50051` |
| `PHONE_DEFAULT_REGION` | `US` | Default region for phone-number parsing |
| `GRPC_PORT` | `50051` | Python worker gRPC port |
| `SPACY_MODEL` | `en_core_web_sm` | spaCy model used by the Python worker |
