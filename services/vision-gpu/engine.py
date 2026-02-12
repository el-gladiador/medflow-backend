"""Unified vLLM engine wrapper for vision model inference.

Single engine, no provider abstraction. Works identically for any
vLLM-compatible vision model (Qwen3-VL, Pixtral, etc.).
"""

import base64
import logging
import re

from config import settings

logger = logging.getLogger(__name__)

_engine: "VisionEngine | None" = None


class VisionEngine:
    """Singleton wrapper around vLLM's LLM for vision inference."""

    def __init__(self) -> None:
        from vllm import LLM

        quantization = settings.QUANTIZATION
        if not quantization:
            quantization = _detect_quantization(settings.MODEL_ID)

        logger.info(
            "Loading model: %s (quantization=%s, gpu_mem=%.2f)",
            settings.MODEL_ID,
            quantization or "none",
            settings.GPU_MEMORY_UTILIZATION,
        )

        kwargs: dict = {
            "model": settings.MODEL_ID,
            "gpu_memory_utilization": settings.GPU_MEMORY_UTILIZATION,
            "max_model_len": settings.MAX_MODEL_LEN,
            "trust_remote_code": True,
        }
        if quantization:
            kwargs["quantization"] = quantization

        self._llm = LLM(**kwargs)
        self._model_id = settings.MODEL_ID
        logger.info("Model loaded successfully")

    @property
    def model_id(self) -> str:
        return self._model_id

    def extract(self, image_bytes: bytes, prompt: str) -> str:
        """Run vision inference on an image with the given prompt.

        Returns the raw text response from the model.
        """
        from vllm import SamplingParams

        image_b64 = base64.b64encode(image_bytes).decode()
        image_url = f"data:image/jpeg;base64,{image_b64}"

        messages = [
            {
                "role": "user",
                "content": [
                    {"type": "image_url", "image_url": {"url": image_url}},
                    {"type": "text", "text": prompt},
                ],
            }
        ]

        sampling_params = SamplingParams(
            temperature=settings.TEMPERATURE,
            max_tokens=settings.MAX_TOKENS,
            top_p=settings.TOP_P,
        )

        outputs = self._llm.chat(
            messages=[messages],
            sampling_params=sampling_params,
        )

        raw_text = outputs[0].outputs[0].text
        logger.info("Model response (%d chars): %s", len(raw_text), raw_text[:500])
        return raw_text


def _detect_quantization(model_id: str) -> str:
    """Auto-detect quantization from model ID patterns."""
    model_lower = model_id.lower()
    if "awq" in model_lower:
        return "awq"
    if "gptq" in model_lower:
        return "gptq"
    return ""


def init_engine() -> None:
    """Initialize the singleton engine at startup."""
    global _engine
    _engine = VisionEngine()


def get_engine() -> VisionEngine:
    """Get the initialized engine. Raises if not initialized."""
    if _engine is None:
        raise RuntimeError("Engine not initialized. Call init_engine() first.")
    return _engine
