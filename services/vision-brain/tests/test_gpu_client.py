"""Tests for GPU client retry/timeout behavior."""

import json
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import httpx
import pytest

sys.path.insert(0, str(Path(__file__).parent.parent))

from gpu_client import GPUClient, GPUServiceError, GPUServiceUnavailable


@pytest.fixture
def gpu_client():
    """Create a GPU client with fast retry settings for testing."""
    client = GPUClient(
        base_url="http://fake-gpu:8090",
        timeout=5,
        connect_timeout=2,
        retry_attempts=3,
        retry_delay=0.01,  # Fast retries for tests
        retry_backoff=1.0,  # No backoff for tests
    )
    yield client
    client.close()


class TestInfer:
    def test_successful_inference(self, gpu_client: GPUClient):
        response_data = {"text": '{"first_name": "Max"}', "model_id": "test-model", "inference_time_ms": 5000}
        mock_response = httpx.Response(200, json=response_data)

        with patch.object(gpu_client._client, "post", return_value=mock_response):
            text, ms = gpu_client.infer(b"fake-image", "extract fields")
            assert text == '{"first_name": "Max"}'
            assert ms == 5000

    def test_503_triggers_retry_then_succeeds(self, gpu_client: GPUClient):
        """503 should trigger retry; succeed on second attempt."""
        response_503 = httpx.Response(503, json={"detail": "Model loading"})
        response_200 = httpx.Response(200, json={"text": "ok", "model_id": "m", "inference_time_ms": 100})

        call_count = 0

        def mock_post(*args, **kwargs):
            nonlocal call_count
            call_count += 1
            if call_count == 1:
                return response_503
            return response_200

        with patch.object(gpu_client._client, "post", side_effect=mock_post):
            text, ms = gpu_client.infer(b"fake-image", "prompt")
            assert text == "ok"
            assert call_count == 2

    def test_503_exhausts_retries(self, gpu_client: GPUClient):
        """All 503s should exhaust retries and raise GPUServiceUnavailable."""
        response_503 = httpx.Response(503, json={"detail": "Model loading"})

        with patch.object(gpu_client._client, "post", return_value=response_503):
            with pytest.raises(GPUServiceUnavailable):
                gpu_client.infer(b"fake-image", "prompt")

    def test_500_raises_gpu_error_no_retry(self, gpu_client: GPUClient):
        """500 should raise GPUServiceError immediately (no retry)."""
        response_500 = httpx.Response(500, json={"detail": "Internal error"})

        with patch.object(gpu_client._client, "post", return_value=response_500):
            with pytest.raises(GPUServiceError, match="Internal error"):
                gpu_client.infer(b"fake-image", "prompt")

    def test_400_raises_gpu_error(self, gpu_client: GPUClient):
        """400 should raise GPUServiceError (non-retryable)."""
        response_400 = httpx.Response(400, json={"detail": "Bad base64"})

        with patch.object(gpu_client._client, "post", return_value=response_400):
            with pytest.raises(GPUServiceError, match="Bad base64"):
                gpu_client.infer(b"fake-image", "prompt")

    def test_connection_error_triggers_retry(self, gpu_client: GPUClient):
        """Connection errors should trigger retry."""
        call_count = 0
        response_200 = httpx.Response(200, json={"text": "ok", "model_id": "m", "inference_time_ms": 100})

        def mock_post(*args, **kwargs):
            nonlocal call_count
            call_count += 1
            if call_count <= 2:
                raise httpx.ConnectError("Connection refused")
            return response_200

        with patch.object(gpu_client._client, "post", side_effect=mock_post):
            text, ms = gpu_client.infer(b"fake-image", "prompt")
            assert text == "ok"
            assert call_count == 3

    def test_read_timeout_triggers_retry(self, gpu_client: GPUClient):
        """Read timeout should trigger retry."""
        call_count = 0
        response_200 = httpx.Response(200, json={"text": "ok", "model_id": "m", "inference_time_ms": 100})

        def mock_post(*args, **kwargs):
            nonlocal call_count
            call_count += 1
            if call_count == 1:
                raise httpx.ReadTimeout("Read timed out")
            return response_200

        with patch.object(gpu_client._client, "post", side_effect=mock_post):
            text, ms = gpu_client.infer(b"fake-image", "prompt")
            assert text == "ok"
            assert call_count == 2


class TestHealth:
    def test_health_success(self, gpu_client: GPUClient):
        mock_response = httpx.Response(200, json={"status": "healthy", "ready": True})

        with patch.object(gpu_client._client, "get", return_value=mock_response):
            result = gpu_client.health()
            assert result["status"] == "healthy"
            assert result["ready"] is True

    def test_health_failure_returns_error(self, gpu_client: GPUClient):
        with patch.object(gpu_client._client, "get", side_effect=httpx.ConnectError("refused")):
            result = gpu_client.health()
            assert result["status"] == "unreachable"
            assert "error" in result
