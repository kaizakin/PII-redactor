package processor

import (
	"github.com/kaizakin/PII-redactor/internal/detector"
	"github.com/kaizakin/PII-redactor/internal/faker"
)

// DefaultGenerators returns the faker.Generator for every PII type this
// engine detects, structured and unstructured alike. Each generator is
// independent: a fake name and a fake email for the same person are not
// correlated with each other (that would need entity linking — knowing
// which email in a document belongs to which name — which this engine
// does not attempt). What every generator does guarantee, via faker.Cache,
// is that one entity's fake value is identical everywhere that exact
// original value appears in a document.
func DefaultGenerators() map[detector.PIIType]faker.Generator {
	return map[detector.PIIType]faker.Generator{
		detector.TypeEmail:      faker.Email,
		detector.TypePhone:      faker.Phone,
		detector.TypeSSN:        faker.SSN,
		detector.TypeCreditCard: faker.CreditCard,
		detector.TypeIPAddress:  faker.IPv4,
		detector.TypeDOB:        faker.DateOfBirth,
		detector.TypeName:       faker.PersonName,
		detector.TypeCompany:    faker.Company,
		detector.TypeAddress:    faker.Address,
	}
}
