#!/bin/bash
# One-time setup: create GCS bucket, download model from HuggingFace via Cloud Build,
# grant Cloud Run SA access, and set lifecycle rules.
#
# The model is downloaded directly from HuggingFace to GCS on GCP infrastructure
# (no local bandwidth needed). Uses Cloud Build with /workspace shared volume.
#
# Requires: gcloud CLI (authenticated)
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

# 2. Generate Cloud Build config for downloading model on GCP infra
#    Step 1: python downloads from HuggingFace to /workspace/model/
#    Step 2: gcloud uploads from /workspace/model/ to GCS bucket
CLOUDBUILD_FILE=$(mktemp /tmp/cloudbuild-model-XXXXXX.yaml)
cat > "${CLOUDBUILD_FILE}" <<YAML
timeout: 1800s
steps:
  - id: download-model
    name: python:3.12-slim
    entrypoint: bash
    args:
      - -c
      - |
        pip install --no-cache-dir huggingface_hub
        python3 -c "
        from huggingface_hub import snapshot_download
        snapshot_download('${MODEL_HF_ID}', local_dir='/workspace/model')
        "
        echo "--- Download complete ---"
        ls -lh /workspace/model/

  - id: upload-to-gcs
    name: gcr.io/cloud-builders/gcloud
    entrypoint: bash
    args:
      - -c
      - |
        gcloud storage cp /workspace/model/*.json /workspace/model/*.txt /workspace/model/*.safetensors ${BUCKET}/${MODEL_GCS_PATH}/
        echo "--- Upload complete ---"
    waitFor: [download-model]
YAML

echo "--- Submitting Cloud Build (HuggingFace -> GCS) ---"
gcloud builds submit --no-source \
    --region=europe-west1 \
    --config="${CLOUDBUILD_FILE}"

rm -f "${CLOUDBUILD_FILE}"

# 3. IAM: Grant Cloud Run default SA read access
echo "--- Granting Cloud Run SA read access ---"
PROJECT_NUMBER=$(gcloud projects describe "${GCP_PROJECT}" --format='value(projectNumber)')
gcloud storage buckets add-iam-policy-binding "${BUCKET}" \
    --member="serviceAccount:${PROJECT_NUMBER}-compute@developer.gserviceaccount.com" \
    --role="roles/storage.objectViewer"

# 4. Lifecycle rule (30-day cleanup for tmp/ prefix)
echo "--- Setting lifecycle rules ---"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
gcloud storage buckets update "${BUCKET}" --lifecycle-file="${SCRIPT_DIR}/gcs-lifecycle.json"

echo ""
echo "=== Done ==="
echo "Model uploaded to: ${BUCKET}/${MODEL_GCS_PATH}/"
echo "Cloud Run SA granted objectViewer access."
echo "You can now deploy with GCS FUSE mount via: make cloud-submit"
