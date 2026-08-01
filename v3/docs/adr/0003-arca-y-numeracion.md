# ADR 0003: Pymes reserva el número; fiscal llama API ARCA de bajo nivel

**Estado:** aceptada e implementada; activación real pendiente de homologación.
**Decisión:** dentro de la transacción de Pymes se reserva punto de venta, tipo y
número y se congela el snapshot. `FiscalAuthority` puede resolverse a un mock
determinista o al adapter ARCA. El adapter real consume la versión publicada de
`arca-facturacion`, usa sólo la operación de bajo nivel con número explícito y
prohíbe helpers de autonumeración.

Cada organización configura CUIT, ambiente, puntos de venta y credenciales. El
servicio Fiscal genera la clave privada y CSR, devuelve únicamente el CSR,
valida el certificado cargado y cifra clave/certificado/tickets con la clave
KMS `fiscal-vault` del entorno. Pymes no posee un certificado fiscal global.

**Consecuencia:** tras un timeout el único paso permitido es consultar el mismo
número. Nunca se emite otro comprobante ni se contabiliza antes del CAE. El mock
permanece para CI y rollout por feature flag; la ruta real sólo se habilita
después del onboarding y homologación del tenant.
