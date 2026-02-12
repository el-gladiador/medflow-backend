"""Tests for the image preprocessing pipeline."""

import sys
from pathlib import Path

import cv2
import numpy as np
import pytest

# Add parent directory to path so we can import the modules
sys.path.insert(0, str(Path(__file__).parent.parent))

from preprocessing import (
    _clahe_normalize,
    _crop_document,
    _decode,
    _deskew,
    _encode,
    _resize,
    preprocess,
)


class TestDecode:
    def test_valid_jpeg(self, sample_image_bytes: bytes):
        img = _decode(sample_image_bytes)
        assert img is not None
        assert img.ndim == 3
        assert img.shape[2] == 3  # BGR

    def test_invalid_bytes(self, invalid_bytes: bytes):
        img = _decode(invalid_bytes)
        assert img is None


class TestCropDocument:
    def test_image_with_clear_border(self):
        """Create an image with a white card on black background."""
        img = np.zeros((500, 700, 3), dtype=np.uint8)
        # Draw a white rectangle (simulating a document)
        cv2.rectangle(img, (100, 80), (600, 420), (255, 255, 255), -1)

        result = _crop_document(img)
        # Should crop to approximately the white rectangle
        assert result.shape[0] < img.shape[0]
        assert result.shape[1] < img.shape[1]

    def test_image_without_clear_border(self):
        """Uniform image should return original."""
        img = np.ones((300, 400, 3), dtype=np.uint8) * 128
        result = _crop_document(img)
        assert result.shape == img.shape


class TestDeskew:
    def test_straight_image_unchanged(self):
        """An already-straight image should not be modified."""
        img = np.ones((300, 400, 3), dtype=np.uint8) * 200
        # Draw horizontal lines
        for y in range(50, 250, 30):
            cv2.line(img, (20, y), (380, y), (0, 0, 0), 2)

        result = _deskew(img)
        assert result.shape == img.shape

    def test_returns_same_shape(self):
        """Even if deskewed, output shape should match input."""
        img = np.ones((300, 400, 3), dtype=np.uint8) * 200
        result = _deskew(img)
        assert result.shape == img.shape


class TestCLAHE:
    def test_dark_image_brightened(self):
        """CLAHE should increase contrast on a dark image."""
        img = np.ones((200, 300, 3), dtype=np.uint8) * 40  # Very dark
        result = _clahe_normalize(img)
        # Mean brightness should increase
        assert result.mean() >= img.mean()

    def test_preserves_dimensions(self):
        img = np.ones((200, 300, 3), dtype=np.uint8) * 128
        result = _clahe_normalize(img)
        assert result.shape == img.shape


class TestResize:
    def test_landscape_resize(self):
        """Landscape image should resize to 1024x768."""
        img = np.ones((1500, 2000, 3), dtype=np.uint8)
        result = _resize(img)
        assert result.shape == (768, 1024, 3)

    def test_portrait_resize(self):
        """Portrait image should resize to 768x1024 (h=1024, w=768)."""
        img = np.ones((2000, 1500, 3), dtype=np.uint8)
        result = _resize(img)
        assert result.shape == (1024, 768, 3)

    def test_already_correct_size(self):
        """Image close to target size should not be resized."""
        img = np.ones((768, 1024, 3), dtype=np.uint8)
        result = _resize(img)
        assert result.shape == img.shape


class TestEncode:
    def test_encode_success(self):
        img = np.ones((100, 100, 3), dtype=np.uint8) * 128
        result = _encode(img, fallback=b"fallback")
        # Should be JPEG bytes (starts with FF D8 FF)
        assert result[:3] == b"\xff\xd8\xff"

    def test_encode_fallback_on_failure(self):
        """Empty array should fail to encode, falling back."""
        img = np.array([], dtype=np.uint8)
        result = _encode(img, fallback=b"fallback")
        assert result == b"fallback"


class TestFullPipeline:
    def test_valid_image_processed(self, sample_image_bytes: bytes):
        result = preprocess(sample_image_bytes)
        assert isinstance(result, bytes)
        assert len(result) > 0
        # Result should be valid JPEG
        assert result[:3] == b"\xff\xd8\xff"

    def test_invalid_bytes_returns_original(self, invalid_bytes: bytes):
        result = preprocess(invalid_bytes)
        assert result == invalid_bytes

    def test_large_image_resized(self, large_image_bytes: bytes):
        result = preprocess(large_image_bytes)
        # Decode result and check dimensions
        img = _decode(result)
        assert img is not None
        # Should be resized to target dimensions
        h, w = img.shape[:2]
        assert max(h, w) <= 1024
