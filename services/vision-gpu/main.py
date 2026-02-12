"""Minimal GPU inference service — receives preprocessed images, returns raw text.

GDPR: No image logging, no disk writes — images are processed in-memory only.
This service only runs in production on GPU instances.
"""

import base64
import logging
import threading
import time
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.responses import JSONResponse
from pydantic import BaseModel

from config import settings

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
logger = logging.getLogger(__name__)

_model_ready = False


def _load_model():
    """Load the vLLM engine in a background thread."""
    global _model_ready
    from engine import init_engine

    try:
        init_engine()
        _model_ready = True
        logger.info("GPU inference service ready")
    except Exception:
        logger.exception("Failed to load model")


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Start model loading in a background thread so the server binds immediately."""
    logger.info("Initializing vLLM engine (model=%s)...", settings.MODEL_ID)
    thread = threading.Thread(target=_load_model, daemon=True)
    thread.start()
    yield


app = FastAPI(title="MedFlow Vision GPU", version="1.0.0", lifespan=lifespan)


class InferRequest(BaseModel):
    image_b64: str
    prompt: str


class InferResponse(BaseModel):
    text: str
    model_id: str
    inference_time_ms: int


@app.post("/infer", response_model=InferResponse)
async def infer(req: InferRequest):
    """Run vision inference on a preprocessed image."""
    if not _model_ready:
        return JSONResponse(
            status_code=503,
            content={"detail": "Model is still loading, please retry"},
        )

    try:
        image_bytes = base64.b64decode(req.image_b64)
    except Exception:
        return JSONResponse(
            status_code=400,
            content={"detail": "Invalid base64 image data"},
        )

    # GDPR: log byte count only, never image content
    logger.info("Inference request: image=%d bytes, prompt=%d chars", len(image_bytes), len(req.prompt))

    from engine import get_engine

    engine = get_engine()
    start = time.monotonic()

    try:
        raw_text = engine.extract(image_bytes, req.prompt)
    except Exception as e:
        logger.error("Inference failed: %s", e)
        return JSONResponse(
            status_code=500,
            content={"detail": f"Inference failed: {e}"},
        )

    elapsed_ms = int((time.monotonic() - start) * 1000)
    logger.info("Inference completed in %dms (%d chars)", elapsed_ms, len(raw_text))

    return InferResponse(
        text=raw_text,
        model_id=engine.model_id,
        inference_time_ms=elapsed_ms,
    )


@app.get("/health")
async def health():
    """Return engine status and readiness."""
    if not _model_ready:
        return JSONResponse(
            status_code=503,
            content={
                "status": "loading",
                "model_id": settings.MODEL_ID,
                "ready": False,
            },
        )

    try:
        from engine import get_engine

        engine = get_engine()
        return {
            "status": "healthy",
            "model_id": engine.model_id,
            "ready": True,
        }
    except Exception as e:
        logger.error("Health check failed: %s", e)
        return JSONResponse(
            status_code=503,
            content={
                "status": "unhealthy",
                "model_id": settings.MODEL_ID,
                "ready": False,
                "error": str(e),
            },
        )


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=settings.PORT)
