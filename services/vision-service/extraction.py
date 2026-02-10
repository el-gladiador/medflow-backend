"""Core extraction logic: sends image to Ollama and parses the response."""

import base64
import json
import logging
import os
import re
import time

import ollama

from models import ExtractionField, ExtractionResponse
from prompts import PROMPTS

logger = logging.getLogger(__name__)

OLLAMA_URL = os.getenv("OLLAMA_URL", "http://localhost:11434")
OLLAMA_MODEL = os.getenv("OLLAMA_MODEL", "moondream")

BASE_CONFIDENCE = 0.75
BOOSTED_CONFIDENCE = 0.85


def get_ollama_client() -> ollama.Client:
    return ollama.Client(host=OLLAMA_URL)


def extract_from_image(image_bytes: bytes, doc_type: str) -> ExtractionResponse:
    """Send image to Ollama vision model and parse the structured response."""
    start = time.monotonic()

    prompt = PROMPTS.get(doc_type)
    if prompt is None:
        return ExtractionResponse(
            document_type=doc_type,
            fields=[],
            warnings=[f"No prompt defined for document type: {doc_type}"],
            processing_time_ms=0,
        )

    client = get_ollama_client()
    image_b64 = base64.b64encode(image_bytes).decode()

    try:
        response = client.chat(
            model=OLLAMA_MODEL,
            messages=[
                {
                    "role": "user",
                    "content": prompt,
                    "images": [image_b64],
                }
            ],
        )
    except Exception as e:
        elapsed_ms = int((time.monotonic() - start) * 1000)
        logger.error("Ollama inference failed: %s", e)
        return ExtractionResponse(
            document_type=doc_type,
            fields=[],
            warnings=[f"Vision model inference failed: {e}"],
            processing_time_ms=elapsed_ms,
        )

    # ollama v0.4+ returns Pydantic ChatResponse — use attribute access
    raw_content = response.message.content or ""
    logger.info("Ollama raw response (%d chars): %s", len(raw_content), raw_content[:500])

    fields = parse_json_from_response(raw_content, doc_type)
    elapsed_ms = int((time.monotonic() - start) * 1000)

    warnings = []
    if not fields:
        warnings.append("Could not extract any fields from the image. The image may be unclear or the document type may not match.")

    return ExtractionResponse(
        document_type=doc_type,
        fields=fields,
        warnings=warnings,
        processing_time_ms=elapsed_ms,
    )


def parse_json_from_response(raw: str, doc_type: str) -> list[ExtractionField]:
    """Extract JSON from the model's response text and convert to ExtractionFields."""
    parsed = try_parse_json(raw)
    if parsed is None:
        return []

    # Known field keys per document type
    expected_keys = {
        "personalausweis": {"first_name", "last_name", "date_of_birth", "gender", "nationality", "document_number", "expiry_date", "document_type"},
        "reisepass": {"first_name", "last_name", "date_of_birth", "gender", "nationality", "document_number", "expiry_date", "document_type"},
        "fuehrerschein": {"first_name", "last_name", "date_of_birth", "document_number", "expiry_date", "license_classes", "document_type"},
    }

    known = expected_keys.get(doc_type, set())
    fields = []

    for key, value in parsed.items():
        if not isinstance(value, str) or not value.strip():
            continue

        # Boost confidence for keys we explicitly asked for
        confidence = BOOSTED_CONFIDENCE if key in known else BASE_CONFIDENCE

        fields.append(ExtractionField(
            key=key,
            value=value.strip(),
            confidence=confidence,
            source=doc_type,
        ))

    return fields


def try_parse_json(raw: str) -> dict | None:
    """Try to extract a JSON object from the model output."""
    # Try direct parse first
    try:
        result = json.loads(raw.strip())
        if isinstance(result, dict):
            return result
    except json.JSONDecodeError:
        pass

    # Try to find JSON block in markdown code fences
    match = re.search(r"```(?:json)?\s*\n?(.*?)\n?```", raw, re.DOTALL)
    if match:
        try:
            result = json.loads(match.group(1).strip())
            if isinstance(result, dict):
                return result
        except json.JSONDecodeError:
            pass

    # Try to find first { ... } block
    match = re.search(r"\{[^{}]*\}", raw, re.DOTALL)
    if match:
        try:
            result = json.loads(match.group(0))
            if isinstance(result, dict):
                return result
        except json.JSONDecodeError:
            pass

    logger.warning("Could not parse JSON from model response: %s", raw[:200])
    return None
