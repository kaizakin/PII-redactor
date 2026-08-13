# PII Redactor Upload UI

Minimal Next.js upload page for the Go redaction service.

## Run

```bash
cp .env.example .env.local
npm install
npm run dev
```

`GO_SERVICE_URL` should point at the Go service base URL. The UI posts to
`/redact/docx` through the Next.js server route, so the browser does not need
direct access to the Go service.
