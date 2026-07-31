# Pymes v3: diseño e implementación

Este directorio conserva el dossier y la evidencia de la implementación activa
de Pymes v3. v1 y v2 siguen siendo referencias inmutables. La arquitectura
implementada es:

- **Pymes v3** es el producto, BFF, IAM, dueño de documentos comerciales y
  orquestador de procesos.
- **Open Accounting** se adapta como servicio contable privado y *headless*.
- **arca-facturacion** queda detrás de un adaptador fiscal privado; se usa su
  API de bajo nivel con numeración proporcionada por Pymes.
- **Pymes v2**, **pyafipws** y **LedgerSMB** son referencias de comportamiento,
  no dependencias de ejecución ni fuentes de código copiadas.

## Lectura sugerida

1. [Evidencia e inventario](01-evidencia.md)
2. [Arquitectura y contratos de comportamiento](02-arquitectura.md)
3. [Destino de cada módulo](03-disposicion-modulos.md)
4. [Validación y spikes descartables](04-validacion.md)
5. [Roadmap](05-roadmap.md)
6. [Plan de ejecución](06-plan-ejecucion.md)
7. [Estado verificable de implementación](07-estado-implementacion.md)
8. [ADRs](adr/)
9. [Contratos OpenAPI internos](../contracts/)

Los diagramas Mermaid describen la arquitectura objetivo. Toda API marcada
`internal` sólo admite credenciales de servicio: ningún navegador llega a los
servicios contable o fiscal.
