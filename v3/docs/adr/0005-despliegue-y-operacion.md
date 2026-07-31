# ADR 0005: bases separadas, secreto centralizado y recuperación verificable

**Estado:** aceptada.  
**Decisión:** Pymes, Fiscal y Accounting tienen despliegues y bases separados.
Accounting utiliza schemas por organización provisionados. Cuando se habilite
ARCA, sus secretos se resolverán desde KMS/secret manager sólo en Fiscal;
backups, migraciones y restores ya se prueban por servicio.

**Consecuencia:** cada despliegue publica health y readiness; Worker y Fiscal
publican métricas de outbox/lease, circuitos e incertidumbres. La expiración de
certificados se agrega con ARCA real. Logs y trazas no pueden incluir PII, XML
ni secretos.
