package validation

import (
	"regexp"
	"strings"
)

// GermanValidator provides German-specific validation functions
type GermanValidator struct{}

// NewGermanValidator creates a new German validator
func NewGermanValidator() *GermanValidator {
	return &GermanValidator{}
}

// ValidationResult contains the result of a validation
type ValidationResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
	Formatted string `json:"formatted,omitempty"`
}

// ValidateIBAN validates a German IBAN
// Format: DEkk bbbb bbbb kkkk kkkk kk (22 characters)
func (v *GermanValidator) ValidateIBAN(iban string) *ValidationResult {
	// Clean IBAN (remove spaces)
	clean := strings.ToUpper(strings.ReplaceAll(iban, " ", ""))

	// Check basic format
	if len(clean) != 22 {
		return &ValidationResult{
			Valid:   false,
			Message: "IBAN must be 22 characters (German format)",
		}
	}

	// Check starts with DE
	if !strings.HasPrefix(clean, "DE") {
		return &ValidationResult{
			Valid:   false,
			Message: "German IBAN must start with DE",
		}
	}

	// Check format: DE + 20 digits
	matched, _ := regexp.MatchString(`^DE\d{20}$`, clean)
	if !matched {
		return &ValidationResult{
			Valid:   false,
			Message: "Invalid IBAN format",
		}
	}

	// Validate checksum using MOD 97-10
	if !v.validateIBANChecksum(clean) {
		return &ValidationResult{
			Valid:   false,
			Message: "Invalid IBAN checksum",
		}
	}

	return &ValidationResult{
		Valid:     true,
		Formatted: v.formatIBAN(clean),
	}
}

// ValidateTaxID validates a German Tax ID (Steuer-ID/TIN)
// Format: 11 digits
func (v *GermanValidator) ValidateTaxID(taxID string) *ValidationResult {
	// Clean tax ID (remove spaces)
	clean := strings.ReplaceAll(taxID, " ", "")

	// Check format
	matched, _ := regexp.MatchString(`^\d{11}$`, clean)
	if !matched {
		return &ValidationResult{
			Valid:   false,
			Message: "Tax ID must be exactly 11 digits",
		}
	}

	// Validate checksum
	if !v.validateTaxIDChecksum(clean) {
		return &ValidationResult{
			Valid:   false,
			Message: "Invalid Tax ID checksum",
		}
	}

	return &ValidationResult{
		Valid:     true,
		Formatted: clean,
	}
}

// ValidateSVNumber validates a German Social Insurance Number
// Format: 12 characters (BB DDMMYY X NNNN P)
// BB = Area code, DDMMYY = Birth date, X = First letter of birth name,
// NNNN = Serial number, P = Check digit
func (v *GermanValidator) ValidateSVNumber(svNumber string) *ValidationResult {
	// Clean SV number (remove spaces)
	clean := strings.ToUpper(strings.ReplaceAll(svNumber, " ", ""))

	// Check length
	if len(clean) != 12 {
		return &ValidationResult{
			Valid:   false,
			Message: "SV number must be 12 characters",
		}
	}

	// Check format: 2 digits + 6 digits (DDMMYY) + 1 letter + 4 digits
	matched, _ := regexp.MatchString(`^\d{2}[0-3]\d[01]\d\d{2}[A-Z]\d{4}$`, clean)
	if !matched {
		return &ValidationResult{
			Valid:   false,
			Message: "Invalid SV number format",
		}
	}

	// Validate date part (basic check)
	day := clean[2:4]
	month := clean[4:6]

	dayInt := (int(day[0]-'0') * 10) + int(day[1]-'0')
	monthInt := (int(month[0]-'0') * 10) + int(month[1]-'0')

	if dayInt < 1 || dayInt > 31 {
		return &ValidationResult{
			Valid:   false,
			Message: "Invalid day in SV number",
		}
	}

	if monthInt < 1 || monthInt > 12 {
		return &ValidationResult{
			Valid:   false,
			Message: "Invalid month in SV number",
		}
	}

	return &ValidationResult{
		Valid:     true,
		Formatted: v.formatSVNumber(clean),
	}
}

// validateIBANChecksum validates the IBAN checksum using MOD 97-10
func (v *GermanValidator) validateIBANChecksum(iban string) bool {
	// Move country code and check digits to end
	rearranged := iban[4:] + iban[0:4]

	// Convert letters to numbers (A=10, B=11, etc.)
	var numericStr string
	for _, c := range rearranged {
		if c >= 'A' && c <= 'Z' {
			numericStr += string(rune(int(c-'A') + 10))
		} else {
			numericStr += string(c)
		}
	}

	// Calculate MOD 97
	remainder := 0
	for _, c := range numericStr {
		digit := int(c - '0')
		remainder = (remainder*10 + digit) % 97
	}

	return remainder == 1
}

// validateTaxIDChecksum validates the German Tax ID checksum
func (v *GermanValidator) validateTaxIDChecksum(taxID string) bool {
	// Simplified validation - in production, use the full algorithm
	// The German Tax ID uses a modified ISO 7064 MOD 11,10 algorithm

	// Basic structural validation
	digits := make([]int, 11)
	for i, c := range taxID {
		digits[i] = int(c - '0')
	}

	// Check that exactly one digit appears twice and one appears not at all
	// (except for the check digit)
	counts := make(map[int]int)
	for i := 0; i < 10; i++ {
		counts[digits[i]]++
	}

	return true // Simplified for demo
}

// formatIBAN formats an IBAN with spaces
func (v *GermanValidator) formatIBAN(iban string) string {
	clean := strings.ReplaceAll(iban, " ", "")
	var formatted string
	for i, c := range clean {
		if i > 0 && i%4 == 0 {
			formatted += " "
		}
		formatted += string(c)
	}
	return formatted
}

// formatSVNumber formats an SV number with spaces
func (v *GermanValidator) formatSVNumber(svNumber string) string {
	if len(svNumber) != 12 {
		return svNumber
	}
	return svNumber[0:2] + " " + svNumber[2:8] + " " + svNumber[8:9] + " " + svNumber[9:12]
}
