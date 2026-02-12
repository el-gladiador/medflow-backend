"""Shared test fixtures for vision brain tests."""

import json
import sys
from pathlib import Path

import numpy as np
import pytest

# Add parent directory to path so we can import the modules
sys.path.insert(0, str(Path(__file__).parent.parent))


@pytest.fixture
def sample_image_bytes() -> bytes:
    """Generate a minimal valid JPEG image for testing."""
    import cv2

    # Create a 200x300 image with some text-like features
    img = np.zeros((300, 200, 3), dtype=np.uint8)
    img[:] = (240, 240, 240)  # Light gray background

    # Add some dark rectangles to simulate text regions
    cv2.rectangle(img, (20, 30), (180, 50), (30, 30, 30), -1)
    cv2.rectangle(img, (20, 70), (160, 90), (30, 30, 30), -1)
    cv2.rectangle(img, (20, 110), (140, 130), (30, 30, 30), -1)

    _, buf = cv2.imencode(".jpg", img, [cv2.IMWRITE_JPEG_QUALITY, 90])
    return buf.tobytes()


@pytest.fixture
def large_image_bytes() -> bytes:
    """Generate a larger image that will trigger resize."""
    import cv2

    img = np.zeros((2000, 3000, 3), dtype=np.uint8)
    img[:] = (200, 200, 200)
    _, buf = cv2.imencode(".jpg", img)
    return buf.tobytes()


@pytest.fixture
def invalid_bytes() -> bytes:
    """Non-image bytes for testing graceful degradation."""
    return b"this is not an image file at all"


@pytest.fixture
def mock_personalausweis_response() -> str:
    """Mock VLM response for a Personalausweis extraction."""
    return json.dumps({
        "first_name": "Max",
        "last_name": "Mustermann",
        "date_of_birth": "1990-01-15",
        "birth_place": "Berlin",
        "gender": "M",
        "nationality": "DEU",
        "document_number": "T220001293",
        "expiry_date": "2031-01-14",
        "street": "Musterstraße",
        "house_number": "42",
        "postal_code": "10115",
        "city": "Berlin",
        "document_type": "personalausweis",
    })


@pytest.fixture
def mock_reisepass_response() -> str:
    """Mock VLM response for a Reisepass extraction."""
    return json.dumps({
        "first_name": "Anna",
        "last_name": "Schmidt",
        "date_of_birth": "1985-06-20",
        "birth_place": "München",
        "gender": "F",
        "nationality": "DEU",
        "document_number": "C01X00T47",
        "expiry_date": "2030-06-19",
        "document_type": "reisepass",
    })


@pytest.fixture
def mock_markdown_response() -> str:
    """Mock VLM response wrapped in markdown code fence."""
    return '```json\n{"first_name": "Max", "last_name": "Mustermann", "document_type": "personalausweis"}\n```'


@pytest.fixture
def mock_preamble_response() -> str:
    """Mock VLM response with text before JSON."""
    return 'Here is the extracted data:\n\n{"first_name": "Max", "last_name": "Mustermann", "document_type": "personalausweis"}'
