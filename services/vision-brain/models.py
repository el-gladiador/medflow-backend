"""Pydantic models matching Go domain.ExtractionResult."""

from pydantic import BaseModel


class ExtractionField(BaseModel):
    key: str
    value: str
    confidence: float
    source: str


class ExtractionResponse(BaseModel):
    document_type: str
    fields: list[ExtractionField]
    warnings: list[str] = []
    processing_time_ms: int
