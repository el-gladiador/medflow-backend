# GPU Dockerfile for vision model inference via vLLM
# Model weights loaded at runtime via GCS FUSE volume mount (not baked into image).
# See scripts/upload-model.sh for one-time model upload to GCS.

FROM vllm/vllm-openai:v0.8.5

WORKDIR /app

# Install additional Python dependencies (vLLM + CUDA already in base image)
COPY requirements.txt .
RUN pip install --no-cache-dir pydantic-settings>=2.0.0

# Copy application code
COPY . .

# GDPR: prevent any runtime downloads or telemetry
ENV HF_HUB_OFFLINE=1
ENV TRANSFORMERS_OFFLINE=1
ENV TMPDIR=/tmp
# Model path set by Cloud Run GCS FUSE volume mount
ENV MODEL_ID=/model/qwen3-vl-8b-instruct
ENV PORT=8090

EXPOSE 8090

# Single worker — GPU inference is sequential
# Cloud Run sets PORT env var; use shell form so $PORT is expanded at runtime
CMD uvicorn main:app --host 0.0.0.0 --port $PORT --workers 1
