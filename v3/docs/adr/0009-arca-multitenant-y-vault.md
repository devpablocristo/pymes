# ADR 0009: ARCA y vault fiscal son por organización

## Decisión

Fiscal Adapter conserva el material ARCA en su propia base, cifrado mediante
envelope encryption y una clave Cloud KMS separada por entorno. Cada
organización genera su clave privada dentro de Fiscal, recibe sólo el CSR y
carga su certificado. No existe una credencial ni un CUIT global de Pymes.

El servicio consume una versión publicada de `arca-facturacion` y sólo su API
de numeración explícita. Pymes reserva punto de venta, tipo y número; Fiscal
autoriza o consulta exactamente esa referencia.

## Consecuencias

- BFF y navegador nunca manipulan claves privadas, tickets WSAA ni XML.
- RLS y AAD aíslan material aun cuando dos tenants reutilicen IDs externos.
- Homologación se habilita por organización antes de producción.
- Una respuesta perdida se reconcilia; no se reemite ni reutiliza el número.
- STG y PRD requieren service accounts, bases y claves KMS distintas.
- El secreto histórico `fiscal-credential` no se usa para clientes.

Padrón, FCE, WSFEX y CAEA quedan fuera de esta decisión inicial.
