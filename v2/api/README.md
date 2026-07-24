# Contrato HTTP de Pymes v2

`openapi.yaml` es la fuente de verdad para la API pública de v2. El nombre
interno del producto no cambia el prefijo público: las rutas de negocio viven
bajo `/api/v1`.

Reglas:

- la organización activa se obtiene exclusivamente del token verificado;
- ningún request acepta `org_id`, tenant headers ni tenant en la URL;
- las mutaciones IAM requieren `Idempotency-Key`;
- los tipos Go y TypeScript se regeneran desde este contrato;
- `GET /api/v1/runtime-config` es público;
- `/webhooks/clerk` exige la firma Clerk/Svix (`svix-id`,
  `svix-timestamp`, `svix-signature`) y no acepta autenticación bearer;
- los demás endpoints heredan autenticación bearer; `/organizations` y
  `/sessions` validan la identidad sin exigir organización activa;
- `owner` nunca es un rol asignable por invitación o edición: sólo puede
  obtenerse mediante `ownership-transfer`;
- el ciclo de vida local (`pending`, `accepted`, etc.) y el estado de
  sincronización (`queued`, `pending`, `synced`, `failed`) son conceptos
  separados.
