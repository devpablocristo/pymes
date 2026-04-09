-- Demo: varias notificaciones in-app para un miembro de la org sembrada (__SEED_ORG_ID__).
-- Idempotente; si la org no tiene miembros aún, crea un usuario demo local para poder mostrar la bandeja.

WITH ensure_demo_user AS (
    INSERT INTO users (
        id,
        external_id,
        email,
        name,
        avatar_url,
        created_at,
        updated_at,
        phone,
        given_name,
        family_name
    )
    VALUES (
        '00000000-0000-0000-0000-000000000002'::uuid,
        'user_local_demo_notifications',
        'demo.notifications@local.test',
        'Demo Notifications',
        '',
        now(),
        now(),
        '',
        'Demo',
        'Notifications'
    )
    ON CONFLICT (id) DO NOTHING
),
ensure_demo_member AS (
    INSERT INTO org_members (id, org_id, user_id, role, created_at)
    SELECT
        uuid_generate_v5('__SEED_ORG_ID__'::uuid, 'pymes-seed/v1/org-member/demo-notifications'),
        '__SEED_ORG_ID__'::uuid,
        '00000000-0000-0000-0000-000000000002'::uuid,
        'owner',
        now()
    WHERE EXISTS (SELECT 1 FROM orgs WHERE id = '__SEED_ORG_ID__'::uuid)
      AND NOT EXISTS (
        SELECT 1
        FROM org_members
        WHERE org_id = '__SEED_ORG_ID__'::uuid
          AND user_id = '00000000-0000-0000-0000-000000000002'::uuid
      )
),
recipient AS (
    SELECT '00000000-0000-0000-0000-000000000002'::uuid AS uid
),
fixtures AS (
    SELECT
        uuid_generate_v5('__SEED_ORG_ID__'::uuid, 'pymes-seed/v1/in-app-notif/demo-welcome') AS id,
        'Tip: Asistente Pymes'::text AS title,
        'Usá «Más información» en cada aviso para abrir el chat con el Asistente Pymes y ampliar el contexto.'::text AS body,
        'system'::text AS kind,
        ''::text AS entity_type,
        ''::text AS entity_id,
        '{"suggested_user_message": "¿Qué puedo preguntarte sobre las notificaciones y el día a día del negocio?"}'::jsonb AS chat_context,
        NULL::timestamptz AS read_at,
        now() - interval '4 hours' AS created_at

    UNION ALL

    SELECT
        uuid_generate_v5('__SEED_ORG_ID__'::uuid, 'pymes-seed/v1/in-app-notif/sales-weekly'),
        'Ventas de la semana',
        'Facturaste menos de lo esperado y el ticket promedio cayó frente al período anterior. Conviene revisar mix de productos y clientes frecuentes.',
        'insight',
        'insight',
        'sales-weekly',
        '{
          "scope": "sales",
          "routed_agent": "sales",
          "content_language": "es",
          "suggested_user_message": "Resumime cómo viene el negocio esta semana y decime 3 acciones concretas para vender más."
        }'::jsonb,
        NULL::timestamptz,
        now() - interval '95 minutes'

    UNION ALL

    SELECT
        uuid_generate_v5('__SEED_ORG_ID__'::uuid, 'pymes-seed/v1/in-app-notif/collections-followup'),
        'Cobros a seguir',
        'Hay comprobantes pendientes de cobro y riesgo de demorar caja esta semana.',
        'insight',
        'account_receivable',
        'collections-followup',
        '{
          "scope": "sales_collections",
          "routed_agent": "collections",
          "content_language": "es",
          "suggested_user_message": "Mostrame qué cobros debería perseguir primero y cómo impactan en la caja."
        }'::jsonb,
        NULL::timestamptz,
        now() - interval '50 minutes'

    UNION ALL

    SELECT
        uuid_generate_v5('__SEED_ORG_ID__'::uuid, 'pymes-seed/v1/in-app-notif/stock-alert'),
        'Stock crítico',
        'Hay productos con poco stock en categorías de alta rotación. Conviene decidir reposición antes de perder ventas.',
        'insight',
        'inventory',
        'stock-alert',
        '{
          "scope": "inventory",
          "routed_agent": "products",
          "content_language": "es",
          "suggested_user_message": "Decime qué productos debería reponer primero para no perder ventas esta semana."
        }'::jsonb,
        NULL::timestamptz,
        now() - interval '25 minutes'

    UNION ALL

    SELECT
        uuid_generate_v5('__SEED_ORG_ID__'::uuid, 'pymes-seed/v1/in-app-notif/review-approval'),
        'Aprobación pendiente',
        'Hay una acción sensible pendiente de decisión antes de continuar.',
        'approval',
        'review_approval',
        'approval-demo-1',
        '{
          "source": "review_approval",
          "approval": {
            "id": "approval-demo-1",
            "request_id": "request-demo-1",
            "action_type": "notification.bulk_send",
            "target_resource": "campaign:demo-whatsapp",
            "reason": "El envío masivo supera el umbral configurado y requiere aprobación.",
            "risk_level": "medium",
            "status": "pending",
            "ai_summary": "La campaña impacta a múltiples clientes y conviene validar mensaje y destinatarios antes de enviarla.",
            "created_at": "2026-04-09T03:00:00Z"
          },
          "suggested_user_message": "Explicame por qué esta aprobación está pendiente y qué riesgo tiene."
        }'::jsonb,
        NULL::timestamptz,
        now() - interval '10 minutes'

    UNION ALL

    SELECT
        uuid_generate_v5('__SEED_ORG_ID__'::uuid, 'pymes-seed/v1/in-app-notif/customer-winback'),
        'Clientes para reactivar',
        'Detectamos clientes que dejaron de comprar y conviene reactivarlos con una oferta puntual.',
        'insight',
        'customers',
        'customer-winback',
        '{
          "scope": "customers",
          "routed_agent": "customers",
          "content_language": "es",
          "suggested_user_message": "¿Qué clientes conviene reactivar primero y con qué propuesta comercial?"
        }'::jsonb,
        now() - interval '5 minutes',
        now() - interval '30 minutes'
)
INSERT INTO pymes_in_app_notifications (
    id,
    org_id,
    user_id,
    title,
    body,
    kind,
    entity_type,
    entity_id,
    chat_context,
    read_at,
    created_at
)
SELECT
    f.id,
    '__SEED_ORG_ID__'::uuid,
    r.uid,
    f.title,
    f.body,
    f.kind,
    f.entity_type,
    f.entity_id,
    f.chat_context,
    f.read_at,
    f.created_at
FROM fixtures f
CROSS JOIN recipient r
WHERE EXISTS (SELECT 1 FROM orgs WHERE id = '__SEED_ORG_ID__'::uuid)
ON CONFLICT (id) DO NOTHING;
