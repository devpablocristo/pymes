# Verticals

Punto de extension para verticales del producto, no para modulos internos del `control-plane`.

Uso esperado:

- una `vertical` tiene ownership funcional propio
- cada vertical mantiene sus piezas desplegables separadas (`backend`, `frontend`, `AI`) cuando aplica
- la integracion con `control-plane` se hace por contratos HTTP, no por imports de dominio interno
