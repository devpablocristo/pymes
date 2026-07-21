# Pymes v1 — archivo congelado

Este directorio conserva el árbol completo de Pymes v1 como material de
consulta. La fotografía canónica es el tag anotado `v1-final`, commit
`152730abbb16a50b8b9ddc8837965bb633f9413c`.

## Política

- No recibe correcciones, dependencias nuevas, CI, deploys ni soporte.
- Los workflows históricos están preservados en `v1/.github/workflows/`, una
  ubicación que GitHub Actions no ejecuta.
- Las migraciones y esquemas de v1 no se aplican en v2.
- Los comportamientos útiles se reconstruyen en v2 mediante contratos y casos
  de aceptación; no se copia deuda o compatibilidad legacy.

La integridad del archivo se verifica comparando cada blob original del tag
`v1-final` con su equivalente bajo `v1/`.

## Baseline conocido

- Compilación y pruebas Go acotadas pasaban al congelar el árbol, con cobertura
  de integración PostgreSQL insuficiente.
- El typecheck web pasaba. La suite completa presentaba timeouts por contención
  que pasaban al ejecutarse de forma aislada.
- Mobile era un prototipo y no tenía su toolchain Expo disponible localmente.
- La deuda financiera incluía uso extensivo de `float64`, ausencia de una
  transacción común para venta/stock/caja, idempotencia incompleta y ausencia
  de outbox general.
