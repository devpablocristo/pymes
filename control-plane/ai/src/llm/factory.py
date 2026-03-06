from src.config import Settings
from src.llm.base import EchoProvider, LLMProvider
from src.llm.gemini import GeminiProvider


def create_provider(config: Settings) -> LLMProvider:
    provider = config.llm_provider.strip().lower()
    if provider == "gemini":
        if not config.gemini_api_key:
            return EchoProvider()
        return GeminiProvider(api_key=config.gemini_api_key, model=config.gemini_model)
    raise ValueError(f"LLM provider desconocido: {config.llm_provider}")
