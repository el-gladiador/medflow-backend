package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/medflow/medflow-backend/internal/docprocessing/domain"
)

// JPEG and PNG magic bytes for image detection
var (
	jpegMagic = []byte{0xFF, 0xD8, 0xFF}
	pngMagic  = []byte{0x89, 0x50, 0x4E, 0x47}
)

// VLMProcessor extracts document fields by sending images to an Ollama vision service.
type VLMProcessor struct {
	visionURL  string
	httpClient *http.Client
}

// NewVLMProcessor creates a new VLM processor that calls the given vision service URL.
func NewVLMProcessor(visionURL string) *VLMProcessor {
	return &VLMProcessor{
		visionURL: visionURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Vision inference can take 10-20s
		},
	}
}

func (p *VLMProcessor) Name() string { return "vlm" }

func (p *VLMProcessor) CanProcess(docType domain.DocumentType) bool {
	return docType == domain.DocumentTypePersonalausweis ||
		docType == domain.DocumentTypeReisepass ||
		docType == domain.DocumentTypeFuehrerschein
}

func (p *VLMProcessor) Process(ctx context.Context, imageData []byte, docType domain.DocumentType) (*domain.ExtractionResult, error) {
	if !isImageData(imageData) {
		return nil, fmt.Errorf("vlm: data is not a JPEG or PNG image, skipping")
	}

	// Build multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "document.bin")
	if err != nil {
		return nil, fmt.Errorf("vlm: create form file: %w", err)
	}
	if _, err := part.Write(imageData); err != nil {
		return nil, fmt.Errorf("vlm: write image data: %w", err)
	}
	if err := writer.WriteField("document_type", string(docType)); err != nil {
		return nil, fmt.Errorf("vlm: write document_type field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("vlm: close multipart writer: %w", err)
	}

	url := p.visionURL + "/api/v1/extract"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("vlm: create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vlm: vision service request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vlm: read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vlm: vision service returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the vision service response into domain.ExtractionResult
	var visionResp visionExtractionResponse
	if err := json.Unmarshal(respBody, &visionResp); err != nil {
		return nil, fmt.Errorf("vlm: parse response: %w", err)
	}

	fields := make([]domain.ExtractionField, len(visionResp.Fields))
	for i, f := range visionResp.Fields {
		fields[i] = domain.ExtractionField{
			Key:        f.Key,
			Value:      f.Value,
			Confidence: f.Confidence,
			Source:     docType,
		}
	}

	return &domain.ExtractionResult{
		DocumentType:     docType,
		Fields:           fields,
		Warnings:         visionResp.Warnings,
		ProcessingTimeMs: visionResp.ProcessingTimeMs,
	}, nil
}

// isImageData checks for JPEG or PNG magic bytes at the start of the data.
func isImageData(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return bytes.HasPrefix(data, jpegMagic) || bytes.HasPrefix(data, pngMagic)
}

// visionExtractionResponse mirrors the Python ExtractionResponse model.
type visionExtractionResponse struct {
	DocumentType     string               `json:"document_type"`
	Fields           []visionField        `json:"fields"`
	Warnings         []string             `json:"warnings"`
	ProcessingTimeMs int64                `json:"processing_time_ms"`
}

type visionField struct {
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}
