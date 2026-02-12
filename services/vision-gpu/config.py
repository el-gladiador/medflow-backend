"""Environment-based configuration for the GPU inference service."""

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """GPU inference service settings, loaded from environment variables."""

    # Model (any vLLM-compatible vision model)
    MODEL_ID: str = "Qwen/Qwen3-VL-8B-Instruct"

    # vLLM engine settings
    GPU_MEMORY_UTILIZATION: float = 0.85
    MAX_MODEL_LEN: int = 4096
    QUANTIZATION: str = ""  # Empty = auto-detect from model ID (AWQ/GPTQ)

    # Sampling parameters
    TEMPERATURE: float = 0.1
    MAX_TOKENS: int = 1024
    TOP_P: float = 0.95

    # Server
    PORT: int = 8090

    model_config = {"env_prefix": "", "case_sensitive": True}


settings = Settings()
