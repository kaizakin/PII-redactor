package faker

import "github.com/brianvoe/gofakeit/v6"

// Email generates a fake email address.
func Email(f *gofakeit.Faker) string { return f.Email() }

// Phone generates a fake phone number.
func Phone(f *gofakeit.Faker) string { return f.Phone() }

// SSN generates a fake Social Security Number.
func SSN(f *gofakeit.Faker) string { return f.SSN() }

// CreditCard generates a fake, Luhn-valid credit card number.
func CreditCard(f *gofakeit.Faker) string { return f.CreditCardNumber(nil) }

// IPv4 generates a fake IPv4 address.
func IPv4(f *gofakeit.Faker) string { return f.IPv4Address() }

// DateOfBirth generates a fake date formatted as MM/DD/YYYY.
func DateOfBirth(f *gofakeit.Faker) string { return f.Date().Format("01/02/2006") }
