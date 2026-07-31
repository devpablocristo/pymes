# ADR 0002: tenant explícito y credenciales cortas

**Estado:** aceptada.  
**Decisión:** cada organización tiene schema contable provisionado
explícitamente; ausencia significa `ORG_NOT_PROVISIONED`, nunca se deriva un
schema por concatenación. Workers usan JWT interno firmado, audience específica,
`org_id` obligatorio. Compose valida una clave pública Ed25519 fija de
desarrollo; producción publica/rota el material mediante JWKS y exige mTLS en
la red privada.

**Consecuencia:** se elimina el fallback de tenancy observado en OA y se añade
un plano de provisionamiento/auditoría antes de crear documentos.
