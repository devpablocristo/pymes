from __future__ import annotations

from typing import Any

from pymes_core_shared.ai_runtime import get_logger

logger = get_logger(__name__)


def configure_opentelemetry(app: Any, settings: Any, backend_client: Any | None = None) -> None:
    endpoint = getattr(settings, "otel_exporter_otlp_endpoint", "").strip()

    try:
        from opentelemetry import trace
        from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
        from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
        from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor
        from opentelemetry.sdk.resources import Resource
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import BatchSpanProcessor, ConsoleSpanExporter
    except Exception as exc:  # noqa: BLE001
        logger.warning("otel_unavailable", error=str(exc))
        return

    resource = Resource.create(
        {
            "service.name": getattr(settings, "otel_service_name", "pymes-ai"),
            "deployment.environment": getattr(settings, "ai_environment", "development"),
        }
    )
    provider = TracerProvider(resource=resource)
    if endpoint:
        provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter(endpoint=endpoint)))
    else:
        provider.add_span_processor(BatchSpanProcessor(ConsoleSpanExporter()))

    trace.set_tracer_provider(provider)
    FastAPIInstrumentor.instrument_app(app, tracer_provider=provider)

    if backend_client is not None and getattr(backend_client, "_client", None) is not None:
        HTTPXClientInstrumentor().instrument_client(backend_client._client)  # noqa: SLF001

    logger.info("otel_configured", endpoint=endpoint or "console")
