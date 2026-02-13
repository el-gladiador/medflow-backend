# GPU Dockerfile for vision model inference via vLLM
# Model weights are baked into the image at build time for fast cold starts.

FROM vllm/vllm-openai:v0.15.1

WORKDIR /app

# Install additional Python dependencies (vLLM + CUDA already in base image)
COPY requirements.txt .
RUN pip install --no-cache-dir pydantic-settings>=2.0.0

# Download model weights at build time (no runtime GCS dependency)
RUN pip install --no-cache-dir huggingface_hub && \
    huggingface-cli download Qwen/Qwen3-VL-8B-Instruct \
      --local-dir /model/qwen3-vl-8b-instruct && \
    pip uninstall -y huggingface_hub

# Copy application code
COPY . .

# GDPR: prevent any runtime downloads or telemetry
ENV HF_HUB_OFFLINE=1
ENV TRANSFORMERS_OFFLINE=1
ENV TMPDIR=/tmp
ENV MODEL_ID=/model/qwen3-vl-8b-instruct
ENV PORT=8090

EXPOSE 8090

# Reset base image entrypoint (vllm-openai sets ENTRYPOINT to api_server.py)
ENTRYPOINT []

# Single worker — GPU inference is sequential
# Cloud Run sets PORT env var; use shell form so $PORT is expanded at runtime
CMD ["sh", "-c", "uvicorn main:app --host 0.0.0.0 --port $PORT --workers 1"]
