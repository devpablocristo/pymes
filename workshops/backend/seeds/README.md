# Seeds — workshops (auto_repair)

Demo: vehículo `AB 123 CD`, servicios `SRV-OIL` / `SRV-BRAKE`, órdenes `OT-SEED-001` y `OT-SEED-002`.

Requiere **misma base** que el control plane y **seeds del core** ya aplicados (cliente `c1`, producto `p1`).

- **Compose:** `PYMES_SEED_DEMO=true` en `work-backend` (tras `cp-backend` healthy).
- **Docker, todo en uno:** desde la raíz del repo, `make seed-docker-all` (core `01`–`04` + este SQL).
- **Manual host:** `make seed-workshops-demo` con `DATABASE_URL` (después de `make seed-core-demo`).
