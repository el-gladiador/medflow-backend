"""Tests for the extraction orchestrator and JSON parsing."""

import sys
from pathlib import Path
from unittest.mock import MagicMock

sys.path.insert(0, str(Path(__file__).parent.parent))

from extraction import EXPECTED_KEYS, extract_from_image, parse_json_from_response, try_parse_json
from gpu_client import GPUServiceError, GPUServiceUnavailable


class TestTryParseJSON:
    def test_direct_json(self):
        raw = '{"first_name": "Max", "last_name": "Mustermann"}'
        result = try_parse_json(raw)
        assert result == {"first_name": "Max", "last_name": "Mustermann"}

    def test_markdown_fence(self, mock_markdown_response: str):
        result = try_parse_json(mock_markdown_response)
        assert result is not None
        assert result["first_name"] == "Max"

    def test_preamble_text(self, mock_preamble_response: str):
        result = try_parse_json(mock_preamble_response)
        assert result is not None
        assert result["first_name"] == "Max"

    def test_whitespace_padded(self):
        raw = '  \n  {"key": "value"}  \n  '
        result = try_parse_json(raw)
        assert result == {"key": "value"}

    def test_not_json(self):
        result = try_parse_json("This is just plain text with no JSON at all.")
        assert result is None

    def test_array_not_dict(self):
        result = try_parse_json('[1, 2, 3]')
        assert result is None

    def test_empty_string(self):
        result = try_parse_json("")
        assert result is None

    def test_think_block_stripped(self):
        """Qwen3-VL outputs <think>...</think> blocks before JSON."""
        raw = '<think>\nLet me analyze this document...\nI can see the name field.\n</think>\n{"first_name": "Max", "last_name": "Mustermann"}'
        result = try_parse_json(raw)
        assert result is not None
        assert result["first_name"] == "Max"
        assert result["last_name"] == "Mustermann"

    def test_think_block_with_json_inside(self):
        """Ensure JSON inside <think> blocks is ignored, only the final JSON is used."""
        raw = '<think>\nThe document shows {"wrong": "data"} but let me extract properly.\n</think>\n{"first_name": "Anna"}'
        result = try_parse_json(raw)
        assert result is not None
        assert result["first_name"] == "Anna"
        assert "wrong" not in result

    def test_think_block_empty_after_strip(self):
        """If <think> block contains the only text, return None."""
        raw = '<think>\nJust thinking, no output.\n</think>'
        result = try_parse_json(raw)
        assert result is None


class TestParseJSONFromResponse:
    def test_personalausweis_fields(self, mock_personalausweis_response: str):
        fields = parse_json_from_response(mock_personalausweis_response, "personalausweis")
        keys = {f.key for f in fields}
        # Should include address fields
        assert "street" in keys
        assert "house_number" in keys
        assert "postal_code" in keys
        assert "city" in keys
        assert "birth_place" in keys
        assert "first_name" in keys

    def test_reisepass_fields(self, mock_reisepass_response: str):
        fields = parse_json_from_response(mock_reisepass_response, "reisepass")
        keys = {f.key for f in fields}
        assert "birth_place" in keys
        assert "first_name" in keys

    def test_confidence_boosted_for_known_keys(self, mock_personalausweis_response: str):
        fields = parse_json_from_response(mock_personalausweis_response, "personalausweis")
        for field in fields:
            if field.key in EXPECTED_KEYS["personalausweis"]:
                assert field.confidence == 0.85
            else:
                assert field.confidence == 0.75

    def test_empty_values_skipped(self):
        raw = '{"first_name": "Max", "last_name": "", "empty_field": "   "}'
        fields = parse_json_from_response(raw, "personalausweis")
        keys = {f.key for f in fields}
        assert "first_name" in keys
        assert "last_name" not in keys
        assert "empty_field" not in keys

    def test_non_string_values_skipped(self):
        raw = '{"first_name": "Max", "count": 42, "valid": true}'
        fields = parse_json_from_response(raw, "personalausweis")
        assert len(fields) == 1
        assert fields[0].key == "first_name"

    def test_source_set_to_doc_type(self, mock_personalausweis_response: str):
        fields = parse_json_from_response(mock_personalausweis_response, "personalausweis")
        for field in fields:
            assert field.source == "personalausweis"

    def test_unparseable_returns_empty(self):
        fields = parse_json_from_response("no json here", "personalausweis")
        assert fields == []


class TestExtractFromImage:
    """Tests for the full extraction pipeline with mocked GPU client."""

    def test_unknown_doc_type_returns_warning(self, sample_image_bytes: bytes):
        mock_client = MagicMock()
        result = extract_from_image(sample_image_bytes, "unknown_type", mock_client)
        assert result.fields == []
        assert len(result.warnings) == 1
        assert "No prompt defined" in result.warnings[0]
        mock_client.infer.assert_not_called()

    def test_successful_extraction(self, sample_image_bytes: bytes, mock_personalausweis_response: str):
        mock_client = MagicMock()
        mock_client.infer.return_value = (mock_personalausweis_response, 5000)

        result = extract_from_image(sample_image_bytes, "personalausweis", mock_client)
        assert len(result.fields) > 0
        assert result.document_type == "personalausweis"
        assert result.processing_time_ms > 0
        keys = {f.key for f in result.fields}
        assert "first_name" in keys
        assert "last_name" in keys

    def test_gpu_unavailable_returns_warning(self, sample_image_bytes: bytes):
        mock_client = MagicMock()
        mock_client.infer.side_effect = GPUServiceUnavailable("GPU cold starting")

        result = extract_from_image(sample_image_bytes, "personalausweis", mock_client)
        assert result.fields == []
        assert len(result.warnings) == 1
        assert "unavailable" in result.warnings[0].lower()

    def test_gpu_error_returns_warning(self, sample_image_bytes: bytes):
        mock_client = MagicMock()
        mock_client.infer.side_effect = GPUServiceError("Inference failed")

        result = extract_from_image(sample_image_bytes, "personalausweis", mock_client)
        assert result.fields == []
        assert len(result.warnings) == 1
        assert "failed" in result.warnings[0].lower()

    def test_empty_model_response_returns_warning(self, sample_image_bytes: bytes):
        mock_client = MagicMock()
        mock_client.infer.return_value = ("no json here", 1000)

        result = extract_from_image(sample_image_bytes, "personalausweis", mock_client)
        assert result.fields == []
        assert len(result.warnings) == 1
        assert "Could not extract" in result.warnings[0]


class TestExpectedKeysCompleteness:
    """Verify EXPECTED_KEYS covers all fields from the prompts."""

    def test_personalausweis_has_address_fields(self):
        keys = EXPECTED_KEYS["personalausweis"]
        assert "street" in keys
        assert "house_number" in keys
        assert "postal_code" in keys
        assert "city" in keys
        assert "birth_place" in keys

    def test_reisepass_has_birth_place(self):
        keys = EXPECTED_KEYS["reisepass"]
        assert "birth_place" in keys

    def test_fuehrerschein_has_license_classes(self):
        keys = EXPECTED_KEYS["fuehrerschein"]
        assert "license_classes" in keys

    def test_all_doc_types_have_common_fields(self):
        common = {"first_name", "last_name", "date_of_birth", "document_type"}
        for doc_type, keys in EXPECTED_KEYS.items():
            for field in common:
                assert field in keys, f"{field} missing from {doc_type}"
