"""FastAPI vision brain service — CPU orchestrator for document extraction.

Handles preprocessing, prompt selection, JSON parsing.
Delegates GPU inference to the separate vision-gpu service.
GDPR: No image logging, no disk writes — images are processed in-memory only.
"""

import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, File, Form, UploadFile
from fastapi.responses import JSONResponse

from config import settings
from extraction import extract_from_image
from gpu_client import GPUClient
from models import ExtractionResponse

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
logger = logging.getLogger(__name__)

_gpu_client: GPUClient | None = None
_gpu_available: bool = False


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Initialize GPU client on startup if configured."""
    global _gpu_client, _gpu_available

    if not settings.GPU_SERVICE_URL:
        logger.info("GPU service not configured (GPU_SERVICE_URL is empty) — AI extraction disabled")
        _gpu_available = False
    else:
        logger.info("Connecting to GPU service at %s", settings.GPU_SERVICE_URL)
        _gpu_client = GPUClient()
        _gpu_available = True

        # Non-blocking health probe at startup (log only)
        health = _gpu_client.health()
        if health.get("ready"):
            logger.info("GPU service is ready: %s", health)
        else:
            logger.warning("GPU service not yet ready (may be cold starting): %s", health)

    yield

    if _gpu_client is not None:
        _gpu_client.close()


app = FastAPI(title="MedFlow Vision Brain", version="1.0.0", lifespan=lifespan)


@app.post("/api/v1/extract", response_model=ExtractionResponse)
async def extract(
    file: UploadFile = File(...),
    document_type: str = Form(...),
):
    """Extract structured fields from a document image."""
    if not _gpu_available or _gpu_client is None:
        return JSONResponse(
            status_code=503,
            content={"detail": "AI document extraction is not available - no GPU service configured"},
        )

    image_bytes = await file.read()

    if not image_bytes:
        return JSONResponse(
            status_code=400,
            content={"detail": "Empty file uploaded"},
        )

    # GDPR: log byte count only, never image content
    logger.info(
        "Processing extraction: type=%s size=%d bytes",
        document_type,
        len(image_bytes),
    )

    result = extract_from_image(image_bytes, document_type, _gpu_client)
    return result


@app.get("/health")
async def health():
    """Return service status and GPU availability."""
    base = {
        "status": "healthy",
        "gpu_available": _gpu_available,
    }

    if _gpu_available and _gpu_client is not None:
        gpu_health = _gpu_client.health()
        base["gpu_health"] = gpu_health

    return base


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=settings.PORT)
