"""FastAPI vision service for document extraction via Ollama."""

import logging
import os

from fastapi import FastAPI, File, Form, UploadFile
from fastapi.responses import JSONResponse

from extraction import OLLAMA_MODEL, extract_from_image, get_ollama_client
from models import ExtractionResponse

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
logger = logging.getLogger(__name__)

app = FastAPI(title="MedFlow Vision Service", version="1.0.0")


@app.post("/api/v1/extract", response_model=ExtractionResponse)
async def extract(
    file: UploadFile = File(...),
    document_type: str = Form(...),
):
    """Extract structured fields from a document image using Ollama vision."""
    image_bytes = await file.read()

    if not image_bytes:
        return JSONResponse(
            status_code=400,
            content={"detail": "Empty file uploaded"},
        )

    logger.info(
        "Processing extraction: type=%s size=%d bytes filename=%s",
        document_type,
        len(image_bytes),
        file.filename,
    )

    result = extract_from_image(image_bytes, document_type)
    return result


@app.get("/health")
async def health():
    """Check Ollama reachability and model availability."""
    try:
        client = get_ollama_client()
        models = client.list()
        # ollama v0.4+ returns Pydantic models — use attribute access
        model_names = [m.model for m in models.models]
        model_available = any(OLLAMA_MODEL in name for name in model_names)

        return {
            "status": "healthy" if model_available else "degraded",
            "ollama_reachable": True,
            "model": OLLAMA_MODEL,
            "model_available": model_available,
            "available_models": model_names,
        }
    except Exception as e:
        logger.error("Health check failed: %s", e)
        return JSONResponse(
            status_code=503,
            content={
                "status": "unhealthy",
                "ollama_reachable": False,
                "model": OLLAMA_MODEL,
                "error": str(e),
            },
        )


if __name__ == "__main__":
    import uvicorn

    port = int(os.getenv("PORT", "8090"))
    uvicorn.run(app, host="0.0.0.0", port=port)
