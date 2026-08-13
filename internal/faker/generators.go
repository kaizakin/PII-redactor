package faker

import (
	"fmt"

	"github.com/brianvoe/gofakeit/v6"
)

// Email generates a fake email address.
func Email(f *gofakeit.Faker) string { return f.Email() }

// Phone generates a fake phone number formatted like "(415) 555-0132",
// matching the shape most detected phone numbers are written in.
func Phone(f *gofakeit.Faker) string { return f.PhoneFormatted() }

// SSN generates a fake Social Security Number in AAA-GG-SSSS form, the
// only shape SSNDetector matches in the first place.
func SSN(f *gofakeit.Faker) string {
	raw := f.SSN() // always exactly 9 digits (range is 100000000-999999999)
	return fmt.Sprintf("%s-%s-%s", raw[0:3], raw[3:5], raw[5:9])
}

// CreditCard generates a fake, Luhn-valid credit card number.
func CreditCard(f *gofakeit.Faker) string { return f.CreditCardNumber(nil) }

// IPv4 generates a fake IPv4 address.
func IPv4(f *gofakeit.Faker) string { return f.IPv4Address() }

// DateOfBirth generates a fake date formatted as MM/DD/YYYY.
func DateOfBirth(f *gofakeit.Faker) string { return f.Date().Format("01/02/2006") }
