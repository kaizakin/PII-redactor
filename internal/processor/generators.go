package processor

import (
	"github.com/kaizakin/PII-redactor/internal/detector"
	"github.com/kaizakin/PII-redactor/internal/faker"
)

// DefaultGenerators returns the faker.Generator for every structured PII
// type this engine currently detects. Unstructured types (name, company,
// address) are intentionally absent: their fake replacements need to stay
// consistent with each other (a fake name and its fake email should look
// like they belong to the same person), which the Python NLP worker's
// replacement logic — not this map — will be responsible for once it's
// wired up.
func DefaultGenerators() map[detector.PIIType]faker.Generator {
	return map[detector.PIIType]faker.Generator{
		detector.TypeEmail:      faker.Email,
		detector.TypePhone:      faker.Phone,
		detector.TypeSSN:        faker.SSN,
		detector.TypeCreditCard: faker.CreditCard,
		detector.TypeIPAddress:  faker.IPv4,
		detector.TypeDOB:        faker.DateOfBirth,
	}
}
