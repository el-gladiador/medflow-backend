package processor_test

import (
	"context"
	"testing"

	"github.com/medflow/medflow-backend/internal/docprocessing/domain"
	"github.com/medflow/medflow-backend/internal/docprocessing/processor"
)

func TestMRZProcessor_CanProcess(t *testing.T) {
	p := processor.NewMRZProcessor()

	tests := []struct {
		docType domain.DocumentType
		want    bool
	}{
		{domain.DocumentTypePersonalausweis, true},
		{domain.DocumentTypeReisepass, true},
		{domain.DocumentTypeFuehrerschein, false},
		{domain.DocumentTypeLebenslauf, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.docType), func(t *testing.T) {
			if got := p.CanProcess(tt.docType); got != tt.want {
				t.Errorf("CanProcess(%s) = %v, want %v", tt.docType, got, tt.want)
			}
		})
	}
}

func TestMRZProcessor_TD1_Personalausweis(t *testing.T) {
	p := processor.NewMRZProcessor()

	// Sample German Personalausweis MRZ (TD1 format)
	// Line 1: Type + Country + Doc Number
	// Line 2: DOB + Gender + Expiry + Nationality
	// Line 3: Name
	mrz := "IDD<<T220001293<<<<<<<<<<<<<<<\n" +
		"9301015M3112315D<<<<<<<<<<<<<8\n" +
		"MUSTERMANN<<MAX<ALEXANDER<<<<<<"

	result, err := p.Process(context.Background(), []byte(mrz), domain.DocumentTypePersonalausweis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DocumentType != domain.DocumentTypePersonalausweis {
		t.Errorf("DocumentType = %v, want personalausweis", result.DocumentType)
	}

	// Check extracted fields
	fields := make(map[string]string)
	for _, f := range result.Fields {
		fields[f.Key] = f.Value
	}

	tests := []struct {
		key  string
		want string
	}{
		{"document_type", "I"},
		{"document_number", "T22000129"},
		{"date_of_birth", "930101"},
		{"gender", "M"},
		{"expiry_date", "311231"},
		{"nationality", "D"},
		{"last_name", "MUSTERMANN"},
		{"first_name", "MAX ALEXANDER"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, ok := fields[tt.key]
			if !ok {
				t.Errorf("field %q not found in extraction results", tt.key)
				return
			}
			if got != tt.want {
				t.Errorf("field %q = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestMRZProcessor_TD3_Passport(t *testing.T) {
	p := processor.NewMRZProcessor()

	// Sample German Reisepass MRZ (TD3 format)
	mrz := "P<D<<MUSTERMANN<<ERIKA<<<<<<<<<<<<<<<<<<<<<<\n" +
		"C01X00T478D<<8510126F3101013<<<<<<<<<<<<<<<8"

	result, err := p.Process(context.Background(), []byte(mrz), domain.DocumentTypeReisepass)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fields := make(map[string]string)
	for _, f := range result.Fields {
		fields[f.Key] = f.Value
	}

	tests := []struct {
		key  string
		want string
	}{
		{"document_type", "P"},
		{"last_name", "MUSTERMANN"},
		{"first_name", "ERIKA"},
		{"document_number", "C01X00T47"},
		{"nationality", "D"},
		{"date_of_birth", "851012"},
		{"gender", "F"},
		{"expiry_date", "310101"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, ok := fields[tt.key]
			if !ok {
				t.Errorf("field %q not found in extraction results", tt.key)
				return
			}
			if got != tt.want {
				t.Errorf("field %q = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestMRZProcessor_InvalidFormat(t *testing.T) {
	p := processor.NewMRZProcessor()

	result, err := p.Process(context.Background(), []byte("not a valid MRZ"), domain.DocumentTypePersonalausweis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Fields) != 0 {
		t.Errorf("expected no fields for invalid MRZ, got %d", len(result.Fields))
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warnings for invalid MRZ format")
	}
}

func TestMRZProcessor_Name(t *testing.T) {
	p := processor.NewMRZProcessor()
	if p.Name() != "mrz" {
		t.Errorf("Name() = %q, want %q", p.Name(), "mrz")
	}
}
