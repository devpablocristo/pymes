# Verticals

Punto de extension para verticales del producto, no para modulos internos del `pymes-core`.

Uso esperado:

- Una **vertical** tiene ownership funcional propio.
- En este monorepo las verticales desplegables en Go son: **`professionals/`**, **`workshops/`**, **`beauty/`**, **`restaurants/`** (cada una con su `backend/`; `frontend/` y **`ai/`** son únicos y compartidos).
- La integración con `pymes-core` es por **HTTP** y contratos estables, sin imports de dominio interno entre servicios.
