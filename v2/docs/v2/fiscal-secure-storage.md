# Almacenamiento fiscal seguro

La API, `fiscal-worker` y `fiscal-homologation` usan la misma selección de
adaptadores. No puede ocurrir que un proceso emita con una clave y otro intente
leer los artefactos desde un almacén distinto.

## Modos

`PYMES_FISCAL_STORAGE_BACKEND` admite:

- `local`: exclusivamente para `PYMES_ENVIRONMENT=development|test`. Usa un
  directorio privado y AES-256-GCM con la clave de desarrollo. El proceso
  rechaza este modo en producción.
- `aws`: usa APIs compatibles con AWS KMS y S3. Es el único modo aceptado
  cuando `PYMES_ENVIRONMENT=production`.

En modo `aws`, el proceso valida `HeadBucket` y `DescribeKey` durante el
arranque. La clave debe estar habilitada, ser simétrica y permitir
`ENCRYPT_DECRYPT`. Si KMS, S3, la configuración o los permisos no están
disponibles, el proceso no inicia.

## Claves y objetos

Al importar una clave fiscal:

1. el proceso normaliza la clave RSA/ECDSA;
2. KMS genera una data key AES-256;
3. la clave privada se cifra con AES-GCM y contexto autenticado;
4. sólo la data key cifrada por KMS, el ciphertext y la clave pública se
   guardan en S3;
5. PostgreSQL conserva únicamente una referencia opaca `kms://envelope/...`.

El contexto tenant se persiste sólo como hash. Durante WSAA, KMS descifra la
data key y la clave privada existe en memoria únicamente durante la firma; los
buffers de bytes se limpian al finalizar. Ni claves privadas, tickets WSAA ni
credenciales se registran en logs.

PDF, QR, certificados públicos y respuestas ARCA se escriben con:

- `If-None-Match: *`, por lo que una clave de objeto no puede sobrescribirse;
- SHA-256 en metadata y validación después de cada lectura;
- SSE-S3 AES-256, además del cifrado obligatorio definido por la política del
  bucket.

El bucket debe ser privado, bloquear acceso público y denegar `DeleteObject` al
rol normal de la aplicación. Versioning/Object Lock son defensas operativas
recomendadas, pero la aplicación no depende de ellas para su inmutabilidad.

## Configuración de producción

```text
PYMES_ENVIRONMENT=production
PYMES_FISCAL_STORAGE_BACKEND=aws
PYMES_FISCAL_AWS_REGION=us-east-1
PYMES_FISCAL_KMS_KEY_ID=alias/pymes-fiscal
PYMES_FISCAL_S3_BUCKET=pymes-fiscal-private
PYMES_FISCAL_S3_PREFIX=pymes-v2
PYMES_FISCAL_S3_FORCE_PATH_STYLE=false
```

`PYMES_FISCAL_KMS_ENDPOINT` y `PYMES_FISCAL_S3_ENDPOINT` son opcionales para
proveedores compatibles. Si se definen en producción deben usar HTTPS y no
pueden contener usuario o contraseña.

Las credenciales no forman parte de `.env.example` ni de Compose. El SDK usa su
cadena estándar: identidad de workload/instancia/rol o secretos inyectados por
el orquestador. El rol requiere, como mínimo:

- KMS: `DescribeKey`, `GenerateDataKey`, `Encrypt` y `Decrypt` sobre la clave
  fiscal;
- S3: `HeadBucket`, `PutObject` y `GetObject` sobre el prefijo fiscal.

No se concede `DeleteObject`. Una rotación administrada debe conservar la
capacidad de descifrar data keys históricas. Al pasar desde desarrollo a
producción se carga nuevamente el certificado: las referencias
`secret://local/...` no se migran ni se aceptan en producción.
