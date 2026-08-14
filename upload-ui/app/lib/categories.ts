export interface PIICategoryItem {
  id: string;
  name: string;
  pipeline: "Structured (Go)" | "NLP (Python)" | "Visual OCR";
  verification: string;
  exampleRaw: string;
  exampleFake: string;
  description: string;
  tags: string[];
}

export const PII_CATEGORIES: PIICategoryItem[] = [
  {
    id: "ssn",
    name: "Social Security Numbers",
    pipeline: "Structured (Go)",
    verification: "SSA Allocation Rules Check",
    exampleRaw: "123-45-6789",
    exampleFake: "987-65-4321",
    description: "Matches SSN syntax, validates SSA area codes (no 000, 666, 900-999), and swaps with deterministic pseudonyms.",
    tags: ["Go", "Regex", "SSA Check", "Deterministic"],
  },
  {
    id: "creditcard",
    name: "Credit & Debit Cards",
    pipeline: "Structured (Go)",
    verification: "Luhn Checksum Algorithm (13–19 digits)",
    exampleRaw: "4532 0150 2831 9283",
    exampleFake: "4111 2948 1029 8472",
    description: "Rejects arbitrary 16-digit order numbers that fail Luhn. Valid cards are deterministically replaced.",
    tags: ["Go", "Luhn Checksum", "Financial"],
  },
  {
    id: "phone",
    name: "Phone Numbers",
    pipeline: "Structured (Go)",
    verification: "Google libphonenumber Parser",
    exampleRaw: "+1 (415) 555-0192",
    exampleFake: "+1 (415) 555-0188",
    description: "Validates national and international area codes. Non-conforming numbers are ignored.",
    tags: ["Go", "libphonenumber", "NANP"],
  },
  {
    id: "email",
    name: "Email Addresses",
    pipeline: "Structured (Go)",
    verification: "RFC 5322 Syntax Rules",
    exampleRaw: "j.vance@acme-health.org",
    exampleFake: "m.sterling@apex-clinical.org",
    description: "Full mailbox and domain validation. Substitutes realistic pseudonym email formatting.",
    tags: ["Go", "RFC 5322", "Regex"],
  },
  {
    id: "ip",
    name: "IP Addresses",
    pipeline: "Structured (Go)",
    verification: "net.ParseIP Subnet Parser",
    exampleRaw: "192.168.1.104 / 2001:db8::1",
    exampleFake: "10.0.4.82 / 2001:db8:85a3::1",
    description: "Detects and validates IPv4 and IPv6 addresses, preserving valid subnet notation.",
    tags: ["Go", "net.ParseIP", "Networking"],
  },
  {
    id: "dob",
    name: "Dates of Birth",
    pipeline: "Structured (Go)",
    verification: "time.Parse + Lifespan Validity",
    exampleRaw: "November 23, 1984",
    exampleFake: "April 16, 1982",
    description: "Distinguishes birthdates from invoice and contract dates using calendar layouts and age bounds.",
    tags: ["Go", "time.Parse", "Calendar"],
  },
  {
    id: "name",
    name: "Full Person Names",
    pipeline: "NLP (Python)",
    verification: "spaCy PERSON NER (2+ Tokens Required)",
    exampleRaw: "Jonathan Vance",
    exampleFake: "Marcus Sterling",
    description: "spaCy NER model filtered to multi-token names to eliminate false positives on common solitary nouns.",
    tags: ["Python", "gRPC", "spaCy", "Presidio"],
  },
  {
    id: "org",
    name: "Organizations & Companies",
    pipeline: "NLP (Python)",
    verification: "spaCy ORG NER + Legal Suffixes",
    exampleRaw: "Metro Health Partners LLC",
    exampleFake: "Beacon Health Network LLC",
    description: "Dual-signal matching: contextual NER combined with entity suffix patterns (Inc, LLC, Corp, GmbH).",
    tags: ["Python", "spaCy ORG", "Regex"],
  },
  {
    id: "address",
    name: "Physical Street Addresses",
    pipeline: "NLP (Python)",
    verification: "Street-Type Pattern Rules",
    exampleRaw: "742 Evergreen Terrace, Suite 400",
    exampleFake: "104 Meadow Lane, Suite 210",
    description: "Matches street number, name, unit/suite suffixes, and postal boundary heuristics.",
    tags: ["Python", "Heuristics", "Location"],
  },
  {
    id: "images",
    name: "Embedded Image OCR",
    pipeline: "Visual OCR",
    verification: "Tesseract OCR + Pillow Masking",
    exampleRaw: "Scanned State ID / Photo",
    exampleFake: "Blackout Bounding Box Mask",
    description: "Extracts ZIP media, runs OCR character recognition, and splices masked PNG/JPEGs back into the DOCX archive.",
    tags: ["Python", "pytesseract", "Pillow", "Media"],
  },
];

export const EXAMPLE_DOCUMENT_DIFF = {
  title: "Medical Intake & Financial Record Preview",
  meta: "Example DOCX Structure • Direct OpenXML Byte Splicing",
  originalLines: [
    { label: "Document Header", text: "CONFIDENTIAL MEDICAL INTAKE — HOSPITAL ADMISSION REPORT" },
    { label: "Patient Full Name", text: "Patient Name: Jonathan Vance", entity: "PERSON", pii: "Jonathan Vance" },
    { label: "Identifier", text: "Social Security: 123-45-6789", entity: "SSN", pii: "123-45-6789" },
    { label: "Contact Email", text: "Primary Contact: j.vance@acme-health.org", entity: "EMAIL", pii: "j.vance@acme-health.org" },
    { label: "Mobile Phone", text: "Emergency Contact: +1 (415) 555-0192", entity: "PHONE", pii: "+1 (415) 555-0192" },
    { label: "Billing Residence", text: "Residence: 742 Evergreen Terrace, Suite 400, Springfield, OR", entity: "ADDRESS", pii: "742 Evergreen Terrace, Suite 400" },
    { label: "Payment Method", text: "Card on File: 4532 0150 2831 9283 (Visa)", entity: "CREDIT_CARD", pii: "4532 0150 2831 9283" },
    { label: "Birth Date", text: "Date of Birth: November 23, 1984", entity: "DOB", pii: "November 23, 1984" },
    { label: "Healthcare Provider", text: "Assigned Facility: Metro Health Partners LLC", entity: "ORG", pii: "Metro Health Partners LLC" },
    { label: "Embedded Attachments", text: "[Attachment 1: Scanned Driver License — OCR Redacted]", entity: "IMAGE", pii: "Embedded Graphic OCR" },
  ],
  redactedLines: [
    { label: "Document Header", text: "CONFIDENTIAL MEDICAL INTAKE — HOSPITAL ADMISSION REPORT (SANITIZED)" },
    { label: "Patient Full Name", text: "Patient Name: Marcus Sterling", entity: "PERSON", pii: "Marcus Sterling", wasRedacted: true },
    { label: "Identifier", text: "Social Security: 987-65-4321", entity: "SSN", pii: "987-65-4321", wasRedacted: true },
    { label: "Contact Email", text: "Primary Contact: m.sterling@apex-clinical.org", entity: "EMAIL", pii: "m.sterling@apex-clinical.org", wasRedacted: true },
    { label: "Mobile Phone", text: "Emergency Contact: +1 (415) 555-0188", entity: "PHONE", pii: "+1 (415) 555-0188", wasRedacted: true },
    { label: "Billing Residence", text: "Residence: 104 Meadow Lane, Suite 210, Portland, OR", entity: "ADDRESS", pii: "104 Meadow Lane, Suite 210", wasRedacted: true },
    { label: "Payment Method", text: "Card on File: 4111 2948 1029 8472 (Visa)", entity: "CREDIT_CARD", pii: "4111 2948 1029 8472", wasRedacted: true },
    { label: "Birth Date", text: "Date of Birth: April 16, 1982", entity: "DOB", pii: "April 16, 1982", wasRedacted: true },
    { label: "Healthcare Provider", text: "Assigned Facility: Beacon Health Network LLC", entity: "ORG", pii: "Beacon Health Network LLC", wasRedacted: true },
    { label: "Embedded Attachments", text: "[Attachment 1: Scanned Driver License — Blackout Masked]", entity: "IMAGE", pii: "Visual Bounding Box Masked", wasRedacted: true },
  ],
};
