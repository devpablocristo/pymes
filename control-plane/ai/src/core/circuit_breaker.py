from __future__ import annotations

import asyncio
from dataclasses import dataclass
from enum import Enum
from typing import Awaitable, Callable, TypeVar

T = TypeVar("T")


class CircuitBreakerState(str, Enum):
    CLOSED = "closed"
    OPEN = "open"
    HALF_OPEN = "half_open"


class CircuitBreakerOpenError(RuntimeError):
    pass


@dataclass(slots=True)
class CircuitBreaker:
    failure_threshold: int = 3
    recovery_timeout_seconds: float = 30.0

    def __post_init__(self) -> None:
        self._state = CircuitBreakerState.CLOSED
        self._failure_count = 0
        self._opened_at = 0.0
        self._lock = asyncio.Lock()

    @property
    def state(self) -> CircuitBreakerState:
        return self._state

    async def call(self, func: Callable[..., Awaitable[T]], *args, **kwargs) -> T:
        async with self._lock:
            if self._state == CircuitBreakerState.OPEN:
                now = asyncio.get_running_loop().time()
                if now - self._opened_at < self.recovery_timeout_seconds:
                    raise CircuitBreakerOpenError("llm circuit breaker is open")
                self._state = CircuitBreakerState.HALF_OPEN

        try:
            result = await func(*args, **kwargs)
        except Exception:
            async with self._lock:
                self._failure_count += 1
                if self._failure_count >= self.failure_threshold:
                    self._state = CircuitBreakerState.OPEN
                    self._opened_at = asyncio.get_running_loop().time()
            raise

        async with self._lock:
            self._failure_count = 0
            self._state = CircuitBreakerState.CLOSED
        return result
