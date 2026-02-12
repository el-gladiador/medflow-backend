"""Image preprocessing pipeline for document extraction.

Optimizes document images before vision model inference:
1. Decode image bytes
2. Document border detection & perspective crop
3. Deskew via Hough line detection
4. CLAHE lighting normalization
5. Resize to optimal dimensions for VLM token efficiency
6. Encode as JPEG

Each step degrades gracefully — if it fails, the original image continues.
"""

import logging

import cv2
import numpy as np

logger = logging.getLogger(__name__)

# Target dimensions for VLM input
TARGET_PORTRAIT = (768, 1024)  # width, height
TARGET_LANDSCAPE = (1024, 768)


def preprocess(image_bytes: bytes) -> bytes:
    """Run the full preprocessing pipeline on raw image bytes.

    Returns optimized JPEG bytes. If all steps fail, returns the original bytes.
    """
    img = _decode(image_bytes)
    if img is None:
        logger.warning("preprocessing: could not decode image, returning original")
        return image_bytes

    img = _crop_document(img)
    img = _deskew(img)
    img = _clahe_normalize(img)
    img = _resize(img)
    return _encode(img, fallback=image_bytes)


def _decode(image_bytes: bytes) -> np.ndarray | None:
    """Decode raw bytes into an OpenCV BGR array."""
    arr = np.frombuffer(image_bytes, dtype=np.uint8)
    img = cv2.imdecode(arr, cv2.IMREAD_COLOR)
    return img


def _crop_document(img: np.ndarray) -> np.ndarray:
    """Detect the largest quadrilateral contour and perspective-crop to it."""
    try:
        gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
        blurred = cv2.GaussianBlur(gray, (5, 5), 0)
        edges = cv2.Canny(blurred, 50, 150)

        # Dilate to close gaps in edges
        kernel = cv2.getStructuringElement(cv2.MORPH_RECT, (3, 3))
        edges = cv2.dilate(edges, kernel, iterations=1)

        contours, _ = cv2.findContours(edges, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
        if not contours:
            return img

        # Find largest contour by area
        contours = sorted(contours, key=cv2.contourArea, reverse=True)

        for contour in contours[:5]:
            peri = cv2.arcLength(contour, True)
            approx = cv2.approxPolyDP(contour, 0.02 * peri, True)

            if len(approx) == 4:
                # Check that the contour covers at least 20% of the image
                area_ratio = cv2.contourArea(approx) / (img.shape[0] * img.shape[1])
                if area_ratio < 0.2:
                    continue

                pts = approx.reshape(4, 2).astype(np.float32)
                rect = _order_points(pts)
                dst = _perspective_transform(img, rect)
                if dst is not None:
                    logger.debug("preprocessing: document border cropped (area=%.1f%%)", area_ratio * 100)
                    return dst

    except Exception as e:
        logger.warning("preprocessing: border detection failed: %s", e)

    return img


def _order_points(pts: np.ndarray) -> np.ndarray:
    """Order 4 points as: top-left, top-right, bottom-right, bottom-left."""
    rect = np.zeros((4, 2), dtype=np.float32)
    s = pts.sum(axis=1)
    rect[0] = pts[np.argmin(s)]
    rect[2] = pts[np.argmax(s)]
    d = np.diff(pts, axis=1)
    rect[1] = pts[np.argmin(d)]
    rect[3] = pts[np.argmax(d)]
    return rect


def _perspective_transform(img: np.ndarray, rect: np.ndarray) -> np.ndarray | None:
    """Apply perspective transform to extract the document region."""
    (tl, tr, br, bl) = rect

    width_a = np.linalg.norm(br - bl)
    width_b = np.linalg.norm(tr - tl)
    max_width = int(max(width_a, width_b))

    height_a = np.linalg.norm(tr - br)
    height_b = np.linalg.norm(tl - bl)
    max_height = int(max(height_a, height_b))

    if max_width < 100 or max_height < 100:
        return None

    dst = np.array([
        [0, 0],
        [max_width - 1, 0],
        [max_width - 1, max_height - 1],
        [0, max_height - 1],
    ], dtype=np.float32)

    matrix = cv2.getPerspectiveTransform(rect, dst)
    return cv2.warpPerspective(img, matrix, (max_width, max_height))


def _deskew(img: np.ndarray) -> np.ndarray:
    """Correct rotation if > 5 degrees detected via Hough line transform."""
    try:
        gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
        edges = cv2.Canny(gray, 50, 150, apertureSize=3)
        lines = cv2.HoughLinesP(edges, 1, np.pi / 180, threshold=100, minLineLength=50, maxLineGap=10)

        if lines is None or len(lines) < 3:
            return img

        angles = []
        for line in lines:
            x1, y1, x2, y2 = line[0]
            angle = np.degrees(np.arctan2(y2 - y1, x2 - x1))
            # Only consider near-horizontal lines (within 45 degrees)
            if abs(angle) < 45:
                angles.append(angle)

        if not angles:
            return img

        median_angle = float(np.median(angles))

        if abs(median_angle) < 5.0:
            return img

        logger.debug("preprocessing: deskewing by %.1f degrees", median_angle)
        h, w = img.shape[:2]
        center = (w // 2, h // 2)
        matrix = cv2.getRotationMatrix2D(center, median_angle, 1.0)
        return cv2.warpAffine(img, matrix, (w, h), flags=cv2.INTER_LINEAR, borderMode=cv2.BORDER_REPLICATE)

    except Exception as e:
        logger.warning("preprocessing: deskew failed: %s", e)
        return img


def _clahe_normalize(img: np.ndarray) -> np.ndarray:
    """Apply CLAHE to the L channel in LAB color space for lighting normalization."""
    try:
        lab = cv2.cvtColor(img, cv2.COLOR_BGR2LAB)
        l_channel, a_channel, b_channel = cv2.split(lab)

        clahe = cv2.createCLAHE(clipLimit=2.0, tileGridSize=(8, 8))
        l_channel = clahe.apply(l_channel)

        lab = cv2.merge([l_channel, a_channel, b_channel])
        return cv2.cvtColor(lab, cv2.COLOR_LAB2BGR)

    except Exception as e:
        logger.warning("preprocessing: CLAHE failed: %s", e)
        return img


def _resize(img: np.ndarray) -> np.ndarray:
    """Resize to optimal dimensions for VLM token efficiency."""
    try:
        h, w = img.shape[:2]
        is_landscape = w > h
        target_w, target_h = TARGET_LANDSCAPE if is_landscape else TARGET_PORTRAIT

        # Only resize if significantly different (>10% off)
        if abs(w - target_w) / target_w < 0.1 and abs(h - target_h) / target_h < 0.1:
            return img

        return cv2.resize(img, (target_w, target_h), interpolation=cv2.INTER_AREA)

    except Exception as e:
        logger.warning("preprocessing: resize failed: %s", e)
        return img


def _encode(img: np.ndarray, fallback: bytes) -> bytes:
    """Encode image as JPEG bytes."""
    try:
        success, buf = cv2.imencode(".jpg", img, [cv2.IMWRITE_JPEG_QUALITY, 95])
        if success:
            return buf.tobytes()
    except Exception as e:
        logger.warning("preprocessing: JPEG encode failed: %s", e)

    return fallback
