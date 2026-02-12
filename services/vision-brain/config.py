"""Environment-based configuration for the vision brain (CPU orchestrator)."""

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Vision brain settings, loaded from environment variables."""

    # Server
    PORT: int = 8091

    # GPU service connection (empty = GPU unavailable, local dev default)
    GPU_SERVICE_URL: str = ""

    # GPU service timeouts and retry
    GPU_TIMEOUT_SECONDS: int = 600  # 10 min (covers cold start model loading)
    GPU_CONNECT_TIMEOUT: int = 30
    GPU_RETRY_ATTEMPTS: int = 3
    GPU_RETRY_DELAY: float = 5.0
    GPU_RETRY_BACKOFF: float = 2.0

    model_config = {"env_prefix": "", "case_sensitive": True}


settings = Settings()
