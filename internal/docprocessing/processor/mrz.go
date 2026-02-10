package processor

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/medflow/medflow-backend/internal/docprocessing/domain"
)

// MRZProcessor extracts data from Machine Readable Zone (MRZ) text.
// Supports ICAO 9303 format for:
// - Personalausweis (German ID card) - TD1 format (3 lines x 30 chars)
// - Reisepass (Passport) - TD3 format (2 lines x 44 chars)
//
// NOTE: This processor works on MRZ text strings, not on raw images.
// In the initial version, the frontend sends the MRZ text directly.
// A future VLM processor will handle image → MRZ OCR.
type MRZProcessor struct{}

func NewMRZProcessor() *MRZProcessor {
	return &MRZProcessor{}
}

func (p *MRZProcessor) Name() string {
	return "mrz"
}

func (p *MRZProcessor) CanProcess(docType domain.DocumentType) bool {
	return docType == domain.DocumentTypePersonalausweis || docType == domain.DocumentTypeReisepass
}

func (p *MRZProcessor) Process(ctx context.Context, imageData []byte, docType domain.DocumentType) (*domain.ExtractionResult, error) {
	start := time.Now()

	// Treat imageData as MRZ text for now (initial version)
	mrzText := strings.TrimSpace(string(imageData))
	lines := strings.Split(mrzText, "\n")

	// Clean lines
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}

	var fields []domain.ExtractionField
	var warnings []string
	var err error

	switch {
	case len(lines) == 3 && len(lines[0]) >= 30:
		// TD1 format (Personalausweis)
		fields, warnings, err = parseTD1(lines, docType)
	case len(lines) == 2 && len(lines[0]) >= 44:
		// TD3 format (Reisepass)
		fields, warnings, err = parseTD3(lines, docType)
	default:
		return &domain.ExtractionResult{
			DocumentType:     docType,
			Fields:           nil,
			Warnings:         []string{"Could not detect MRZ format. Expected TD1 (3 lines) or TD3 (2 lines)."},
			ProcessingTimeMs: time.Since(start).Milliseconds(),
		}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("MRZ parse error: %w", err)
	}

	return &domain.ExtractionResult{
		DocumentType:     docType,
		Fields:           fields,
		Warnings:         warnings,
		ProcessingTimeMs: time.Since(start).Milliseconds(),
	}, nil
}

// parseTD1 parses a TD1 MRZ (German Personalausweis)
// Line 1: I<UTOD<<DOCUMENT_NUMBER<CHECK_DIGIT...
// Line 2: DATE_OF_BIRTH<CHECK<GENDER<EXPIRY<CHECK<NATIONALITY...
// Line 3: LAST_NAME<<FIRST_NAME<MIDDLE_NAMES...
func parseTD1(lines []string, docType domain.DocumentType) ([]domain.ExtractionField, []string, error) {
	var fields []domain.ExtractionField
	var warnings []string

	line1 := padLine(lines[0], 30)
	line2 := padLine(lines[1], 30)
	line3 := padLine(lines[2], 30)

	// Line 1: Document type + issuing country + document number
	docTypeCode := string(line1[0])
	fields = append(fields, domain.ExtractionField{
		Key:        "document_type",
		Value:      docTypeCode,
		Confidence: 0.95,
		Source:     docType,
	})

	// Document number: positions 5-13 (9 chars)
	docNumber := cleanMRZ(line1[5:14])
	if docNumber != "" {
		fields = append(fields, domain.ExtractionField{
			Key:        "document_number",
			Value:      docNumber,
			Confidence: 0.90,
			Source:     docType,
		})
	}

	// Line 2: Date of birth, gender, expiry, nationality
	dob := line2[0:6]
	if isValidMRZDate(dob) {
		fields = append(fields, domain.ExtractionField{
			Key:        "date_of_birth",
			Value:      string(dob),
			Confidence: 0.92,
			Source:     docType,
		})
	}

	gender := string(line2[7])
	if gender == "M" || gender == "F" {
		fields = append(fields, domain.ExtractionField{
			Key:        "gender",
			Value:      gender,
			Confidence: 0.95,
			Source:     docType,
		})
	}

	expiry := line2[8:14]
	if isValidMRZDate(expiry) {
		fields = append(fields, domain.ExtractionField{
			Key:        "expiry_date",
			Value:      string(expiry),
			Confidence: 0.92,
			Source:     docType,
		})
	}

	nationality := cleanMRZ(line2[15:18])
	if nationality != "" {
		fields = append(fields, domain.ExtractionField{
			Key:        "nationality",
			Value:      nationality,
			Confidence: 0.90,
			Source:     docType,
		})
	}

	// Line 3: Name (LAST_NAME<<FIRST_NAME)
	nameParts := strings.SplitN(string(line3), "<<", 2)
	if len(nameParts) >= 1 {
		lastName := cleanMRZName(nameParts[0])
		if lastName != "" {
			fields = append(fields, domain.ExtractionField{
				Key:        "last_name",
				Value:      lastName,
				Confidence: 0.88,
				Source:     docType,
			})
		}
	}
	if len(nameParts) >= 2 {
		firstName := cleanMRZName(nameParts[1])
		if firstName != "" {
			fields = append(fields, domain.ExtractionField{
				Key:        "first_name",
				Value:      firstName,
				Confidence: 0.88,
				Source:     docType,
			})
		}
	}

	return fields, warnings, nil
}

// parseTD3 parses a TD3 MRZ (Passport)
// Line 1: P<UTOLAST_NAME<<FIRST_NAME<MIDDLE...
// Line 2: DOC_NUMBER<CHECK<NATIONALITY<DOB<CHECK<GENDER<EXPIRY<CHECK...
func parseTD3(lines []string, docType domain.DocumentType) ([]domain.ExtractionField, []string, error) {
	var fields []domain.ExtractionField
	var warnings []string

	line1 := padLine(lines[0], 44)
	line2 := padLine(lines[1], 44)

	// Line 1: Document type + Name
	docTypeCode := string(line1[0])
	fields = append(fields, domain.ExtractionField{
		Key:        "document_type",
		Value:      docTypeCode,
		Confidence: 0.95,
		Source:     docType,
	})

	// Name starts at position 5
	nameSection := string(line1[5:])
	nameParts := strings.SplitN(nameSection, "<<", 2)
	if len(nameParts) >= 1 {
		lastName := cleanMRZName(nameParts[0])
		if lastName != "" {
			fields = append(fields, domain.ExtractionField{
				Key:        "last_name",
				Value:      lastName,
				Confidence: 0.90,
				Source:     docType,
			})
		}
	}
	if len(nameParts) >= 2 {
		firstName := cleanMRZName(nameParts[1])
		if firstName != "" {
			fields = append(fields, domain.ExtractionField{
				Key:        "first_name",
				Value:      firstName,
				Confidence: 0.90,
				Source:     docType,
			})
		}
	}

	// Line 2: Document number (0-8), check (9), nationality (10-12),
	// DOB (13-18), check (19), gender (20), expiry (21-26), check (27)
	docNumber := cleanMRZ(line2[0:9])
	if docNumber != "" {
		fields = append(fields, domain.ExtractionField{
			Key:        "document_number",
			Value:      docNumber,
			Confidence: 0.92,
			Source:     docType,
		})
	}

	nationality := cleanMRZ(line2[10:13])
	if nationality != "" {
		fields = append(fields, domain.ExtractionField{
			Key:        "nationality",
			Value:      nationality,
			Confidence: 0.90,
			Source:     docType,
		})
	}

	dob := line2[13:19]
	if isValidMRZDate(dob) {
		fields = append(fields, domain.ExtractionField{
			Key:        "date_of_birth",
			Value:      string(dob),
			Confidence: 0.92,
			Source:     docType,
		})
	}

	gender := string(line2[20])
	if gender == "M" || gender == "F" {
		fields = append(fields, domain.ExtractionField{
			Key:        "gender",
			Value:      gender,
			Confidence: 0.95,
			Source:     docType,
		})
	}

	expiry := line2[21:27]
	if isValidMRZDate(expiry) {
		fields = append(fields, domain.ExtractionField{
			Key:        "expiry_date",
			Value:      string(expiry),
			Confidence: 0.92,
			Source:     docType,
		})
	}

	return fields, warnings, nil
}

// Helper functions

func padLine(line string, length int) string {
	if len(line) >= length {
		return line[:length]
	}
	return line + strings.Repeat("<", length-len(line))
}

func cleanMRZ(s string) string {
	return strings.TrimRight(strings.ReplaceAll(s, "<", ""), " ")
}

func cleanMRZName(s string) string {
	// Replace single < with space (name separator), remove trailing filler
	cleaned := strings.TrimRight(s, "< ")
	cleaned = strings.ReplaceAll(cleaned, "<", " ")
	return strings.TrimSpace(cleaned)
}

func isValidMRZDate(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}
