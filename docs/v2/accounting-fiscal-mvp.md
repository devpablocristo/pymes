# Contabilidad y Fiscal Argentina

Este módulo implementa el alcance contable necesario para operar una PyME
argentina sin acoplar el libro mayor a ARCA. No reutiliza tablas ni procesos de
v1: conserva sus reglas útiles y las vuelve a implementar sobre contratos,
aislamiento tenant y dinero decimal exacto de v2.

## Límites de los módulos

- `accounting` es neutral al país. Contiene plan de cuentas, mappings, asientos,
  partidas abiertas, períodos, conciliación, multimoneda, inflación y reportes.
- `fiscal` administra el ciclo durable e inmutable de documentos fiscales.
- `fiscal/ar` implementa CUIT, condiciones IVA, WSAA, WSFEv1, CAE, QR y los
  catálogos argentinos.
- `fiscalaccounting` convierte un snapshot fiscal autorizado en un plan
  contable. El worker confirma asiento, partidas abiertas y aplicaciones en una
  única transacción.

La organización tiene país y moneda funcional. El único adaptador habilitado
en este MVP es `AR`; agregar otro país no requiere cambiar el modelo del libro
mayor.

## Garantías contables

- Los importes y cotizaciones son `numeric` en PostgreSQL y strings decimales
  en JSON. El dominio financiero no usa `float64`.
- PostgreSQL impide contabilizar líneas inválidas o un asiento con
  `Debe != Haber`.
- Un asiento contabilizado y sus líneas son inmutables. Una corrección crea una
  reversa enlazada y conserva intacto el original.
- La numeración se asigna dentro de la transacción que contabiliza.
- El evento fuente y su fingerprint quedan persistidos. Un replay idéntico
  devuelve el mismo resultado y reutilizar la clave para otra intención falla.
- Las facturas a crédito y compras generan partidas abiertas. Cobros, pagos y
  notas de crédito las aplican sin mantener una segunda tabla editable de
  saldos.
- Las diferencias de cambio se calculan contra el valor contable remanente de
  la partida, no contra una cotización histórica reconstruida.
- Un período `locked` bloquea posteos; `soft_closed` sólo admite ajustes
  autorizados. Cierre y reapertura quedan auditados.
- Todas las tablas operativas usan RLS forzado y reciben el tenant desde la
  transacción de sesión verificada. Los contratos nunca aceptan ni exponen
  `org_id`.

## Flujo fiscal

1. La API valida la configuración, el punto de venta, el certificado y congela
   un snapshot fiscal.
2. La solicitud queda encolada con idempotencia durable.
3. `fiscal-worker` reserva la serie por punto de venta y tipo, obtiene el TA de
   WSAA y llama WSFEv1 sin mantener una transacción abierta durante la red.
4. Una respuesta perdida deja el comprobante `uncertain`; el worker consulta
   el mismo número con `FECompConsultar` antes de permitir avanzar la serie.
5. Una autorización persiste CAE, respuesta, QR y PDF inmutables con hash.
6. `fiscal-accounting-worker` contabiliza el snapshot autorizado y registra el
   vínculo exacto con el asiento.

Producción sólo puede habilitarse cuando una ejecución de homologación exitosa
coincide con la configuración, el certificado y los puntos de venta actuales.
Modificar cualquiera de esos datos invalida automáticamente la habilitación.

## Operación

Los roles `owner` y `admin` reciben gestión contable y fiscal. Un miembro puede
recibir de forma explícita sólo `accounting:manage` o `fiscal:manage`. Las
consultas usan los permisos `accounting:view` y `fiscal:view`.

Los procesos de larga duración usan roles de base separados:

- API: operaciones interactivas bajo la sesión verificada;
- worker ARCA: leasing y autorización fiscal;
- worker de posteo fiscal: descubrimiento de autorizaciones y escritura
  contable, sin permisos para emitir ante ARCA.

La homologación real es opt-in y nunca forma parte de `make ci`:

```bash
make fiscal-homologation ORG_ID="<uuid>"
```

Antes de habilitar producción se debe configurar el almacenamiento KMS/S3
descrito en [fiscal-secure-storage.md](./fiscal-secure-storage.md), cargar un
certificado de producción, registrar los puntos de venta y completar el flujo
guiado de homologación.

## Verificación

La entrega local se valida con:

```bash
make api-check
make backend-test
make db-test
make db-integration
make web-ci
make ci
```

La suite cubre aislamiento entre tenants, RLS, inmutabilidad, balance,
concurrencia, replays, recuperación de CAE, cierres, partidas históricas y
consistencia entre IVA y libro mayor. La llamada real a ARCA requiere
credenciales suministradas por el operador.

## Fuera del MVP

No se implementan nómina completa, bienes de uso, manufactura, agro,
consolidación, APIs bancarias, exportación E/WSFEXv1, CAEA, presentación
automática de IVA ni liquidación operativa de IIBB. Sus efectos pueden
registrarse mediante asientos manuales y agregarse como módulos posteriores.
