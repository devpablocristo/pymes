# ADR 0011: Arquitectura Go vertical y adapters completos

**Estado:** aceptada.
**Fecha:** 2026-08-02.

## Contexto

Pymes v3 necesita conservar límites de negocio visibles, dirección de
dependencias verificable y adapters que no filtren payloads de infraestructura
al dominio. Una organización horizontal por capas globales vuelve implícito el
ownership, facilita accesos entre repositories de contextos distintos y
convierte los contratos externos en modelos compartidos.

El árbol observado históricamente en otro proyecto sirvió sólo para identificar
el patrón estructural. Pymes debe ser autosuficiente: no puede importar, copiar,
montar, ejecutar ni necesitar ese proyecto en desarrollo, CI o runtime.

## Decisión

Cada bounded context vive como paquete vertical bajo
`backend/internal/<contexto>`. Sus casos de uso y puertos consumer-owned se
declaran en `usecases*.go`; entidades, estados e invariantes viven únicamente
en `usecases/domain`.

`handler`, `repository`, `worker` y cada integración externa son adapters. Todo
adapter posee:

- un archivo raíz con constructor, implementación del puerto y coordinación;
- un directorio `models` o `dto` con estructuras exclusivas del transporte,
  proveedor o persistencia;
- un directorio `helpers` con mapeos, codecs, firmas, normalización y traducción
  de errores.

Las dependencias concretas se construyen sólo en `backend/wire`. Los binarios
en `backend/cmd` cargan configuración, delegan en `wire` y controlan el ciclo
de vida. No existen capas horizontales globales `ports`, `domain`, `handler` o
`repository`, ni un package Go transversal derivado de OpenAPI.

Los OpenAPI siguen siendo canónicos. Los handlers y clientes Go son adapters
locales de cada consumidor; la generación TypeScript puede producir el cliente
Web sin imponer tipos compartidos al backend.

## Verificación

`make architecture-check` inspecciona árbol, AST, imports y dependencias
compiladas. Falla ante:

- adapters incompletos o payloads declarados en su archivo raíz;
- dominio dependiente de HTTP, SQL, pgx, cloud o SDKs externos;
- interfaces ubicadas en el proveedor;
- construcción concreta fuera de `wire`;
- acceso directo al repository o handler de otro contexto;
- capas horizontales prohibidas;
- cualquier import, ruta, checkout, mount, URL o dependencia compilada hacia el
  proyecto usado sólo como referencia.

El gate forma parte de `make ci`, por lo que Pymes compila y se prueba aunque la
referencia no exista en filesystem ni red.

## Consecuencias

El ownership y la dirección de dependencias quedan explícitos y comprobables.
Cada integración paga el costo deliberado de sus propios modelos y mapeos; no
se comparten DTOs para reducir líneas a cambio de acoplamiento.

Agregar un adapter requiere estructura y pruebas completas. Esta fricción es
intencional: hace visibles los límites y evita que infraestructura o contratos
externos se conviertan en el dominio de Pymes.
