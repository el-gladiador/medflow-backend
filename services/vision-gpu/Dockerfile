# Multi-stage GPU Dockerfile for vision model inference via vLLM
# Stage 1: Download model weights (cached layer)
# Stage 2: vLLM runtime with app code

# ---- Stage 1: Download model weights ----
FROM python:3.12-slim AS model-downloader

RUN pip install --no-cache-dir huggingface_hub

ARG MODEL_ID=Qwen/Qwen3-VL-8B-Instruct
RUN python -c "from huggingface_hub import snapshot_download; snapshot_download('${MODEL_ID}', local_dir='/model')"

# ---- Stage 2: Runtime ----
FROM vllm/vllm-openai:v0.8.5

WORKDIR /app

# Install additional Python dependencies (vLLM + CUDA already in base image)
COPY requirements.txt .
RUN pip install --no-cache-dir pydantic-settings>=2.0.0

# Copy model weights from stage 1 (baked into image, no HF download at runtime)
COPY --from=model-downloader /model /model

# Copy application code
COPY . .

# GDPR: prevent any runtime downloads or telemetry
ENV HF_HUB_OFFLINE=1
ENV TRANSFORMERS_OFFLINE=1
ENV TMPDIR=/tmp
ENV MODEL_ID=/model
ENV PORT=8090

EXPOSE 8090

# Single worker — GPU inference is sequential
# Cloud Run sets PORT env var; use shell form so $PORT is expanded at runtime
CMD uvicorn main:app --host 0.0.0.0 --port $PORT --workers 1
