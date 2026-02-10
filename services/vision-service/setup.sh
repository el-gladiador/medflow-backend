#!/bin/bash
# Setup script for the vision service.
# Pulls the Ollama vision model (one-time download).

set -e

MODEL="${OLLAMA_MODEL:-moondream}"

echo "Pulling Ollama model: $MODEL"
ollama pull "$MODEL"

echo "Model $MODEL is ready."
echo ""
echo "To start the vision service:"
echo "  cd medflow-backend/services/vision-service"
echo "  pip install -r requirements.txt"
echo "  python main.py"
