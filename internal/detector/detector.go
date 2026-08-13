// Package detector implements the Strategy pattern for PII detection.
//
// Every PII category (email, phone, SSN, credit card, ...) is a standalone
// type that implements the Detector interface. The core redaction pipeline
// (see internal/processor) never branches on PII type — it just calls
// Detect on every registered Detector and merges the results. Adding a new
// PII type never requires changing existing detectors or the pipeline: you
// write a new struct that implements Detector and register it.
package detector

// PIIType identifies the category of PII a Match belongs to.
type PIIType string

const (
	TypeEmail      PIIType = "EMAIL"
	TypePhone      PIIType = "PHONE_NUMBER"
	TypeSSN        PIIType = "SSN"
	TypeCreditCard PIIType = "CREDIT_CARD"
	TypeIPAddress  PIIType = "IP_ADDRESS"
	TypeDOB        PIIType = "DATE_OF_BIRTH"
	TypeName       PIIType = "PERSON_NAME"
	TypeAddress    PIIType = "PHYSICAL_ADDRESS"
	TypeCompany    PIIType = "COMPANY_NAME"
)

// Match describes a single PII occurrence found in a text payload.
// Start and End are byte offsets into the original text (End is exclusive),
// so callers can slice the source string directly (text[Start:End] == Value).
type Match struct {
	Type  PIIType
	Start int
	End   int
	Value string
}

// Detector finds occurrences of one PII category in a block of text.
// Implementations must be safe for concurrent use: the processor runs every
// registered detector against the same text concurrently from goroutines.
type Detector interface {
	// Detect scans text and returns every match it finds, in any order.
	Detect(text string) []Match
	// Type reports the PII category this detector is responsible for.
	Type() PIIType
}
