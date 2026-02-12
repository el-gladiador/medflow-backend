#!/bin/bash
# One-time setup: create GCS bucket, download model from HuggingFace, upload to GCS.
# Requires: gcloud CLI, huggingface-cli (pip install huggingface_hub)
#
# Usage: bash scripts/upload-model.sh
#        make model-upload
set -euo pipefail

GCP_PROJECT=$(gcloud config get-value project)
BUCKET="gs://${GCP_PROJECT}-vision-models"
MODEL_HF_ID="Qwen/Qwen3-VL-8B-Instruct"
MODEL_GCS_PATH="qwen3-vl-8b-instruct"

echo "=== GCS FUSE Model Upload ==="
echo "Project:  ${GCP_PROJECT}"
echo "Bucket:   ${BUCKET}"
echo "Model:    ${MODEL_HF_ID}"
echo ""

# 1. Create bucket (europe-west1, Standard, uniform access)
echo "--- Creating bucket (if not exists) ---"
gcloud storage buckets create "${BUCKET}" \
    --location=europe-west1 \
    --default-storage-class=STANDARD \
    --uniform-bucket-level-access \
    --quiet 2>/dev/null || echo "Bucket already exists"

# 2. Download model from HuggingFace
echo "--- Downloading model from HuggingFace ---"
huggingface-cli download "${MODEL_HF_ID}" --local-dir "/tmp/${MODEL_GCS_PATH}"

# 3. Upload to GCS
echo "--- Uploading model to GCS ---"
gcloud storage cp -r "/tmp/${MODEL_GCS_PATH}" "${BUCKET}/${MODEL_GCS_PATH}/"

# 4. IAM: Grant Cloud Run default SA read access
echo "--- Granting Cloud Run SA read access ---"
PROJECT_NUMBER=$(gcloud projects describe "${GCP_PROJECT}" --format='value(projectNumber)')
gcloud storage buckets add-iam-policy-binding "${BUCKET}" \
    --member="serviceAccount:${PROJECT_NUMBER}-compute@developer.gserviceaccount.com" \
    --role="roles/storage.objectViewer"

# 5. Lifecycle rule (30-day cleanup for tmp/ prefix)
echo "--- Setting lifecycle rules ---"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
gcloud storage buckets update "${BUCKET}" --lifecycle-file="${SCRIPT_DIR}/gcs-lifecycle.json"

echo ""
echo "=== Done ==="
echo "Model uploaded to: ${BUCKET}/${MODEL_GCS_PATH}/"
echo "Cloud Run SA granted objectViewer access."
echo "You can now deploy with GCS FUSE mount via: make cloud-submit"
