"""Extraction orchestrator — preprocess, call GPU service, parse JSON.

CPU-only pipeline that delegates inference to the GPU service.
"""

import json
import logging
import re
import time

from gpu_client import GPUClient, GPUServiceError, GPUServiceUnavailable
from models import ExtractionField, ExtractionResponse
from preprocessing import preprocess
from prompts import PROMPTS

logger = logging.getLogger(__name__)

BASE_CONFIDENCE = 0.75
BOOSTED_CONFIDENCE = 0.85

# Known field keys per document type (must match prompts.py)
EXPECTED_KEYS: dict[str, set[str]] = {
    "personalausweis": {
        "first_name", "last_name", "date_of_birth", "birth_place",
        "gender", "nationality", "document_number", "expiry_date",
        "street", "house_number", "postal_code", "city",
        "document_type",
    },
    "reisepass": {
        "first_name", "last_name", "date_of_birth", "birth_place",
        "gender", "nationality", "document_number", "expiry_date",
        "document_type",
    },
    "fuehrerschein": {
        "first_name", "last_name", "date_of_birth",
        "document_number", "expiry_date", "license_classes",
        "document_type",
    },
}


def extract_from_image(
    image_bytes: bytes,
    doc_type: str,
    gpu_client: GPUClient,
) -> ExtractionResponse:
    """Run extraction pipeline: preprocess -> GPU inference -> parse."""
    start = time.monotonic()

    prompt = PROMPTS.get(doc_type)
    if prompt is None:
        return ExtractionResponse(
            document_type=doc_type,
            fields=[],
            warnings=[f"No prompt defined for document type: {doc_type}"],
            processing_time_ms=0,
        )

    # Preprocess image (crop, deskew, CLAHE, resize)
    preprocessed = preprocess(image_bytes)
    logger.info(
        "Preprocessed image: %d bytes -> %d bytes",
        len(image_bytes), len(preprocessed),
    )

    # Call GPU service for inference
    warnings: list[str] = []

    try:
        raw_text, inference_ms = gpu_client.infer(preprocessed, prompt)
        logger.info("GPU inference completed in %dms", inference_ms)
    except GPUServiceUnavailable as e:
        elapsed_ms = int((time.monotonic() - start) * 1000)
        logger.error("GPU service unavailable after retries: %s", e)
        return ExtractionResponse(
            document_type=doc_type,
            fields=[],
            warnings=[f"GPU inference service unavailable: {e}"],
            processing_time_ms=elapsed_ms,
        )
    except GPUServiceError as e:
        elapsed_ms = int((time.monotonic() - start) * 1000)
        logger.error("GPU service error: %s", e)
        return ExtractionResponse(
            document_type=doc_type,
            fields=[],
            warnings=[f"GPU inference failed: {e}"],
            processing_time_ms=elapsed_ms,
        )

    # Parse JSON from model output
    fields = parse_json_from_response(raw_text, doc_type)
    elapsed_ms = int((time.monotonic() - start) * 1000)

    if not fields:
        warnings.append(
            "Could not extract any fields from the image. "
            "The image may be unclear or the document type may not match."
        )

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

    known = EXPECTED_KEYS.get(doc_type, set())
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
    """Try to extract a JSON object from the model output.

    Handles: direct JSON, markdown fences, preamble text, and
    Qwen3-VL <think>...</think> blocks.
    """
    if not raw:
        return None

    # Strip <think>...</think> blocks (Qwen3-VL outputs these by default)
    cleaned = re.sub(r"<think>.*?</think>", "", raw, flags=re.DOTALL).strip()

    # Try direct parse first
    try:
        result = json.loads(cleaned)
        if isinstance(result, dict):
            return result
    except json.JSONDecodeError:
        pass

    # Try to find JSON block in markdown code fences
    match = re.search(r"```(?:json)?\s*\n?(.*?)\n?```", cleaned, re.DOTALL)
    if match:
        try:
            result = json.loads(match.group(1).strip())
            if isinstance(result, dict):
                return result
        except json.JSONDecodeError:
            pass

    # Try to find first { ... } block
    match = re.search(r"\{[^{}]*\}", cleaned, re.DOTALL)
    if match:
        try:
            result = json.loads(match.group(0))
            if isinstance(result, dict):
                return result
        except json.JSONDecodeError:
            pass

    logger.warning("Could not parse JSON from model response: %s", cleaned[:200])
    return None
