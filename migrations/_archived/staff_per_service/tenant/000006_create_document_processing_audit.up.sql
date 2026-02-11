-- Document processing audit log for DSGVO compliance
-- Records when documents were processed for employee onboarding
-- NOTE: No image data is ever stored - only metadata about the processing event
CREATE TABLE IF NOT EXISTS document_processing_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_type VARCHAR(50) NOT NULL,
    consent_timestamp TIMESTAMPTZ NOT NULL,
    consent_given_by UUID NOT NULL,
    fields_extracted TEXT[] NOT NULL DEFAULT '{}',
    processing_duration_ms INTEGER,
    image_deleted_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for audit queries by date
CREATE INDEX IF NOT EXISTS idx_document_processing_audit_created_at
    ON document_processing_audit (created_at DESC);
