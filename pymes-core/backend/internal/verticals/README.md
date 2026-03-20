# Verticals

Punto de extension para verticales del producto, no para modulos internos del `pymes-core`.

Uso esperado:

- una `vertical` tiene ownership funcional propio
- cada vertical mantiene sus piezas desplegables separadas (`backend`, `frontend`, `AI`) cuando aplica
- la integracion con `pymes-core` se hace por contratos HTTP, no por imports de dominio interno
