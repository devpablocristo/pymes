# ADR 0003: Pymes reserva el número; fiscal llama API ARCA de bajo nivel

**Estado:** aceptada para la integración futura; runtime actual mock.  
**Decisión:** dentro de la transacción de Pymes se reserva punto de venta, tipo y
número y se congela el snapshot. Hoy `FiscalAuthority` está implementado por un
mock determinista. Cuando se habilite ARCA, el companion real usará
`crearFactura(InvoiceRequest)` del SDK, que admite número explícito. Los helpers
auto-number no se usarán.

**Consecuencia:** tras un timeout el único paso permitido es consultar el mismo
número. El fork se extenderá sólo si falta una operación ARCA requerida tras una
prueba reproducible, no para reemplazar la semántica de Pymes.
