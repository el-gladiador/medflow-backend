"""HTTP client for communicating with the GPU inference service.

Uses httpx with configurable timeouts and tenacity for retry with
exponential backoff on 503 (cold start) and connection errors.
"""

import base64
import logging

import httpx
from tenacity import (
    retry,
    retry_if_exception_type,
    stop_after_attempt,
    wait_exponential,
)

from config import settings

logger = logging.getLogger(__name__)


class GPUServiceUnavailable(Exception):
    """GPU service is temporarily unavailable (retryable — 503, connection error)."""


class GPUServiceError(Exception):
    """GPU service returned a non-retryable error (400, 500)."""


class GPUClient:
    """HTTP client for the GPU inference service with retry and backoff."""

    def __init__(
        self,
        base_url: str | None = None,
        timeout: int | None = None,
        connect_timeout: int | None = None,
        retry_attempts: int | None = None,
        retry_delay: float | None = None,
        retry_backoff: float | None = None,
    ):
        self._base_url = (base_url or settings.GPU_SERVICE_URL).rstrip("/")
        self._retry_attempts = retry_attempts if retry_attempts is not None else settings.GPU_RETRY_ATTEMPTS
        self._retry_delay = retry_delay if retry_delay is not None else settings.GPU_RETRY_DELAY
        self._retry_backoff = retry_backoff if retry_backoff is not None else settings.GPU_RETRY_BACKOFF

        read_timeout = timeout if timeout is not None else settings.GPU_TIMEOUT_SECONDS
        conn_timeout = connect_timeout if connect_timeout is not None else settings.GPU_CONNECT_TIMEOUT

        self._client = httpx.Client(
            base_url=self._base_url,
            timeout=httpx.Timeout(
                connect=float(conn_timeout),
                read=float(read_timeout),
                write=30.0,
                pool=30.0,
            ),
        )

    def close(self):
        self._client.close()

    def infer(self, preprocessed_bytes: bytes, prompt: str) -> tuple[str, int]:
        """Send preprocessed image to GPU service for inference.

        Returns (raw_text, inference_time_ms).
        Raises GPUServiceUnavailable (retryable) or GPUServiceError (non-retryable).
        """
        image_b64 = base64.b64encode(preprocessed_bytes).decode()
        payload = {"image_b64": image_b64, "prompt": prompt}

        return self._infer_with_retry(payload)

    def _infer_with_retry(self, payload: dict) -> tuple[str, int]:
        """Retry wrapper — configured dynamically based on settings."""

        @retry(
            retry=retry_if_exception_type(GPUServiceUnavailable),
            stop=stop_after_attempt(self._retry_attempts),
            wait=wait_exponential(
                multiplier=self._retry_delay,
                exp_base=self._retry_backoff,
                max=120,
            ),
            reraise=True,
            before_sleep=lambda state: logger.warning(
                "GPU service unavailable, retrying in %.1fs (attempt %d/%d)",
                state.next_action.sleep,  # type: ignore[union-attr]
                state.attempt_number,
                self._retry_attempts,
            ),
        )
        def _do_infer() -> tuple[str, int]:
            return self._send_infer(payload)

        return _do_infer()

    def _send_infer(self, payload: dict) -> tuple[str, int]:
        """Send a single inference request to the GPU service."""
        try:
            resp = self._client.post("/infer", json=payload)
        except (httpx.ConnectError, httpx.ConnectTimeout) as e:
            logger.warning("GPU service connection failed: %s", e)
            raise GPUServiceUnavailable(f"Cannot connect to GPU service: {e}") from e
        except httpx.ReadTimeout as e:
            logger.warning("GPU service read timeout: %s", e)
            raise GPUServiceUnavailable(f"GPU service read timeout: {e}") from e
        except httpx.HTTPError as e:
            logger.error("GPU service HTTP error: %s", e)
            raise GPUServiceError(f"GPU service HTTP error: {e}") from e

        if resp.status_code == 503:
            detail = resp.json().get("detail", "Service unavailable")
            logger.warning("GPU service returned 503: %s", detail)
            raise GPUServiceUnavailable(detail)

        if resp.status_code != 200:
            detail = resp.json().get("detail", f"HTTP {resp.status_code}")
            logger.error("GPU service error %d: %s", resp.status_code, detail)
            raise GPUServiceError(detail)

        data = resp.json()
        return data["text"], data.get("inference_time_ms", 0)

    def health(self) -> dict:
        """Check GPU service health. Returns health dict or raises."""
        try:
            resp = self._client.get("/health", timeout=10.0)
            return resp.json()
        except Exception as e:
            logger.warning("GPU health check failed: %s", e)
            return {"status": "unreachable", "error": str(e)}
