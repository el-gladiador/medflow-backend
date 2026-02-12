# CPU-only Dockerfile for vision brain (orchestrator)
FROM python:3.12-slim

WORKDIR /app

# Install system deps for OpenCV and healthcheck
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8091

# 2 workers — CPU-only, no GPU contention
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8091", "--workers", "2"]
