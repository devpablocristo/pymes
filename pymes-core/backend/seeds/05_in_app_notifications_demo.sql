-- Demo: una notificación in-app para un miembro de la org sembrada (__SEED_ORG_ID__).
-- Idempotente; el destinatario es el primer org_member por created_at.

INSERT INTO pymes_in_app_notifications (id, org_id, user_id, title, body, kind, entity_type, entity_id, chat_context)
SELECT
    uuid_generate_v5('__SEED_ORG_ID__'::uuid, 'pymes-seed/v1/in-app-notif/demo-welcome'),
    '__SEED_ORG_ID__'::uuid,
    sub.uid,
    'Tip: Asistente Pymes',
    'Usá «Más información» en cada aviso para abrir el chat con el Asistente Pymes y ampliar el contexto.',
    'system',
    '',
    '',
    '{"suggested_user_message": "¿Qué puedo preguntarte sobre las notificaciones y el día a día del negocio?"}'::jsonb
FROM (
    SELECT user_id AS uid
    FROM org_members
    WHERE org_id = '__SEED_ORG_ID__'::uuid
    ORDER BY created_at ASC, user_id ASC
    LIMIT 1
) sub
WHERE EXISTS (SELECT 1 FROM orgs WHERE id = '__SEED_ORG_ID__'::uuid)
ON CONFLICT (id) DO NOTHING;
