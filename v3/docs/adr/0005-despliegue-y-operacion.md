# ADR 0005: bases separadas, secreto centralizado y recuperación verificable

**Estado:** aceptada.  
**Decisión:** Pymes, Fiscal y Accounting tienen despliegues y bases separados.
Accounting utiliza schemas por organización provisionados. Cuando se habilite
ARCA, sus secretos se resolverán desde KMS/secret manager sólo en Fiscal;
backups, migraciones y restores ya se prueban por servicio.

La clave KMS de identidad interna es independiente de la futura clave fiscal:
el worker recibe `roles/cloudkms.signer` y
`roles/cloudkms.publicKeyViewer` únicamente sobre
`pymes-v3-{env}/internal-jwt-signing`. API, Fiscal y Accounting no pueden
firmar; validan el JWKS activo y solapado. Producción fija una
`CryptoKeyVersion` numérica y nunca monta una semilla JWT desde Secret Manager.

**Consecuencia:** cada despliegue configura startup/readiness contra `/readyz`
y liveness contra `/healthz`. Worker publica `/metrics` en red privada y emite
cada minuto un heartbeat JSON agregado, sin organización ni PII. Un script
idempotente crea métricas basadas en logs, alertas y dashboard por entorno.
Los replay DLQ son explícitos, tenant-scoped e insertan una auditoría inmutable
antes de mover el evento. La expiración de certificados se agrega con ARCA
real. Logs y trazas no pueden incluir PII, XML ni secretos. El procedimiento de
reconciliación, restore, caída y rollback vive en
[`10-runbook-operacion.md`](../10-runbook-operacion.md).
