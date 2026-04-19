INSERT INTO action_types (name, risk_class, enabled) VALUES
  ('appointment.book', 'low', true),
  ('appointment.reschedule', 'low', true),
  ('appointment.cancel', 'medium', true),
  ('discount.apply', 'medium', true),
  ('payment_link.generate', 'low', true),
  ('refund.create', 'high', true),
  ('notification.send', 'low', true),
  ('notification.bulk_send', 'medium', true),
  ('sale.create', 'low', true),
  ('quote.create', 'low', true),
  ('cashflow.movement', 'medium', true),
  ('work_order.delay_notify', 'low', true),
  ('vehicle.service_reminder', 'low', true),
  ('purchase.draft', 'low', true),
  ('procurement.request', 'medium', true),
  ('procurement.submit', 'medium', true)
ON CONFLICT (name) DO UPDATE
SET risk_class = EXCLUDED.risk_class,
    enabled = EXCLUDED.enabled,
    updated_at = now();

DELETE FROM policies
WHERE name IN (
  'auto-allow-appointment-book',
  'auto-allow-appointment-reschedule',
  'require-approval-appointment-cancel',
  'auto-allow-small-discount',
  'require-approval-large-discount',
  'deny-refund',
  'auto-allow-payment-link',
  'auto-allow-notification',
  'require-approval-bulk-notification',
  'auto-allow-sale',
  'auto-allow-quote'
);

INSERT INTO policies (name, action_type, expression, effect, mode, enabled) VALUES
  ('auto-allow-appointment-book', 'appointment.book', 'request.action_type == ""appointment.book""', 'allow', 'enforced', true),
  ('auto-allow-appointment-reschedule', 'appointment.reschedule', 'request.action_type == ""appointment.reschedule""', 'allow', 'enforced', true),
  ('require-approval-appointment-cancel', 'appointment.cancel', 'request.action_type == ""appointment.cancel""', 'require_approval', 'enforced', true),
  ('auto-allow-small-discount', 'discount.apply', 'request.action_type == ""discount.apply"" && double(request.params.percentage) <= 10.0', 'allow', 'enforced', true),
  ('require-approval-large-discount', 'discount.apply', 'request.action_type == ""discount.apply"" && double(request.params.percentage) > 10.0', 'require_approval', 'enforced', true),
  ('deny-refund', 'refund.create', 'request.action_type == ""refund.create""', 'deny', 'enforced', true),
  ('auto-allow-payment-link', 'payment_link.generate', 'request.action_type == ""payment_link.generate""', 'allow', 'enforced', true),
  ('auto-allow-notification', 'notification.send', 'request.action_type == ""notification.send""', 'allow', 'enforced', true),
  ('require-approval-bulk-notification', 'notification.bulk_send', 'request.action_type == ""notification.bulk_send""', 'require_approval', 'enforced', true),
  ('auto-allow-sale', 'sale.create', 'request.action_type == ""sale.create""', 'allow', 'enforced', true),
  ('auto-allow-quote', 'quote.create', 'request.action_type == ""quote.create""', 'allow', 'enforced', true);

DELETE FROM delegations
WHERE owner_id = 'pymes-platform'
  AND owner_type = 'service'
  AND agent_id = 'pymes-ai'
  AND agent_type = 'service';

INSERT INTO delegations (
  owner_id, owner_type, agent_id, agent_type,
  allowed_action_types, allowed_resources, purpose, max_risk_class, enabled
) VALUES (
  'pymes-platform', 'service', 'pymes-ai', 'service',
  '[]'::jsonb, '[]'::jsonb, 'Pymes AI Service - atencion al cliente y operaciones gobernadas', 'high', true
);
