# ADR 0004: outbox transaccional, inbox e inmutabilidad contable

**Estado:** aceptada.  
**Decisión:** toda transición local y su evento se escriben en una transacción.
Cada comando usa `idempotency_key`, `command_id`, fuente y hash de payload. Los
asientos son inmutables y se corrigen con reversa.

**Consecuencia:** no hay transacción distribuida. Las respuestas perdidas
convergen por consulta fiscal o deduplicación contable; reutilizar una clave con
otro payload falla de forma visible.
