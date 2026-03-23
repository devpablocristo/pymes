import { useCallback, useEffect, useState } from 'react';
import { listPolicies, createPolicy, deletePolicy, type PolicyResponse } from '../lib/reviewApi';
import './AutomationRulesPage.css';

type Effect = 'allow' | 'deny' | 'require_approval';

interface RuleTemplate {
  actionType: string;
  displayName: string;
  category: string;
  riskClass: string;
  hasThreshold: boolean;
  thresholdLabel?: string;
  thresholdUnit?: string;
  thresholdPattern?: string;
  defaultThreshold?: number;
}

const RULE_TEMPLATES: RuleTemplate[] = [
  { actionType: 'appointment.book', displayName: 'Agendar turno', category: 'Turnos', riskClass: 'low', hasThreshold: false },
  { actionType: 'appointment.reschedule', displayName: 'Reagendar turno', category: 'Turnos', riskClass: 'low', hasThreshold: false },
  { actionType: 'appointment.cancel', displayName: 'Cancelar turno', category: 'Turnos', riskClass: 'medium', hasThreshold: false },
  { actionType: 'discount.apply', displayName: 'Aplicar descuento', category: 'Descuentos', riskClass: 'medium', hasThreshold: true, thresholdLabel: '%', thresholdUnit: 'percentage_lte', thresholdPattern: 'percentage_gt', defaultThreshold: 10 },
  { actionType: 'payment_link.generate', displayName: 'Generar link de pago', category: 'Pagos', riskClass: 'low', hasThreshold: false },
  { actionType: 'refund.create', displayName: 'Reembolso', category: 'Pagos', riskClass: 'high', hasThreshold: false },
  { actionType: 'sale.create', displayName: 'Crear venta', category: 'Ventas', riskClass: 'medium', hasThreshold: false },
  { actionType: 'quote.create', displayName: 'Crear presupuesto', category: 'Ventas', riskClass: 'low', hasThreshold: false },
  { actionType: 'notification.bulk_send', displayName: 'Envio masivo', category: 'Notificaciones', riskClass: 'medium', hasThreshold: false },
];

interface RuleState {
  effect: Effect;
  threshold: number;
  policyId?: string;
}

const EFFECT_LABELS: Record<Effect, string> = {
  allow: 'Automatico',
  require_approval: 'Pedirme',
  deny: 'No permitir',
};

const DEFAULT_EFFECTS: Record<string, Effect> = {
  'appointment.book': 'allow',
  'appointment.reschedule': 'allow',
  'appointment.cancel': 'require_approval',
  'discount.apply': 'allow',
  'payment_link.generate': 'allow',
  'refund.create': 'deny',
  'sale.create': 'require_approval',
  'quote.create': 'allow',
  'notification.bulk_send': 'require_approval',
};

export default function AutomationRulesPage() {
  const [rules, setRules] = useState<Record<string, RuleState>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [statusMsg, setStatusMsg] = useState<{ text: string; type: 'success' | 'error' } | null>(null);

  const loadPolicies = useCallback(async () => {
    try {
      const resp = await listPolicies();
      const policies = resp.policies || [];
      const state: Record<string, RuleState> = {};

      for (const tpl of RULE_TEMPLATES) {
        const matching = policies.filter((p: PolicyResponse) => p.action_type === tpl.actionType);
        const policy = matching.length > 0 ? matching[0] : null;
        state[tpl.actionType] = {
          effect: (policy?.effect as Effect) || DEFAULT_EFFECTS[tpl.actionType] || 'require_approval',
          threshold: tpl.defaultThreshold || 0,
          policyId: policy?.id,
        };
      }
      setRules(state);
    } catch {
      // Si Review no está disponible, usar defaults
      const state: Record<string, RuleState> = {};
      for (const tpl of RULE_TEMPLATES) {
        state[tpl.actionType] = {
          effect: DEFAULT_EFFECTS[tpl.actionType] || 'require_approval',
          threshold: tpl.defaultThreshold || 0,
        };
      }
      setRules(state);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadPolicies();
  }, [loadPolicies]);

  const handleEffectChange = (actionType: string, effect: Effect) => {
    setRules((prev) => ({
      ...prev,
      [actionType]: { ...prev[actionType], effect },
    }));
    setStatusMsg(null);
  };

  const handleThresholdChange = (actionType: string, threshold: number) => {
    setRules((prev) => ({
      ...prev,
      [actionType]: { ...prev[actionType], threshold },
    }));
    setStatusMsg(null);
  };

  const handleSave = async () => {
    setSaving(true);
    setStatusMsg(null);
    try {
      for (const tpl of RULE_TEMPLATES) {
        const rule = rules[tpl.actionType];
        if (!rule) continue;

        // Si ya existe, eliminar y recrear
        if (rule.policyId) {
          await deletePolicy(rule.policyId);
        }

        let condition: string | undefined;
        if (tpl.hasThreshold && tpl.thresholdPattern && rule.threshold > 0) {
          // Crear dos reglas: una para <= umbral (allow) y otra para > umbral
          if (rule.effect === 'allow') {
            condition = `${tpl.thresholdUnit}:${rule.threshold}`;
            await createPolicy({
              name: `${tpl.actionType}-auto-lte-${rule.threshold}`,
              action_type: tpl.actionType,
              effect: 'allow',
              condition,
            });
            await createPolicy({
              name: `${tpl.actionType}-approval-gt-${rule.threshold}`,
              action_type: tpl.actionType,
              effect: 'require_approval',
              condition: `${tpl.thresholdPattern}:${rule.threshold}`,
            });
            continue;
          }
        }

        await createPolicy({
          name: `${tpl.actionType}-${rule.effect}`,
          action_type: tpl.actionType,
          effect: rule.effect,
          condition,
        });
      }
      setStatusMsg({ text: 'Reglas guardadas', type: 'success' });
      await loadPolicies();
    } catch {
      setStatusMsg({ text: 'Error al guardar las reglas', type: 'error' });
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <div className="automation-rules-page"><div className="loading">Cargando reglas...</div></div>;
  }

  const categories = [...new Set(RULE_TEMPLATES.map((t) => t.category))];

  return (
    <div className="automation-rules-page">
      <h1>Atencion automatica</h1>
      <p className="subtitle">
        Configura que puede hacer el asistente sin consultarte
      </p>

      {categories.map((cat) => (
        <div key={cat} className="rules-category">
          <h2>{cat}</h2>
          {RULE_TEMPLATES.filter((t) => t.category === cat).map((tpl) => {
            const rule = rules[tpl.actionType];
            if (!rule) return null;
            const effectClass =
              rule.effect === 'allow'
                ? 'effect-allow'
                : rule.effect === 'require_approval'
                  ? 'effect-require'
                  : 'effect-deny';
            return (
              <div key={tpl.actionType} className="rule-card">
                <div className="rule-info">
                  <div className="rule-name">{tpl.displayName}</div>
                  <div className="rule-risk">Riesgo: {tpl.riskClass}</div>
                </div>
                <div className="rule-controls">
                  {tpl.hasThreshold && rule.effect === 'allow' && (
                    <div className="threshold-input">
                      <span>{'<='}</span>
                      <input
                        type="number"
                        min={0}
                        value={rule.threshold}
                        onChange={(e) =>
                          handleThresholdChange(tpl.actionType, Number(e.target.value))
                        }
                      />
                      <span>{tpl.thresholdLabel}</span>
                    </div>
                  )}
                  <select
                    className={effectClass}
                    value={rule.effect}
                    onChange={(e) =>
                      handleEffectChange(tpl.actionType, e.target.value as Effect)
                    }
                  >
                    {Object.entries(EFFECT_LABELS).map(([val, label]) => (
                      <option key={val} value={val}>
                        {label}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            );
          })}
        </div>
      ))}

      <div className="save-bar">
        <button className="save-btn" onClick={handleSave} disabled={saving}>
          {saving ? 'Guardando...' : 'Guardar cambios'}
        </button>
      </div>

      {statusMsg && (
        <p className={`status-msg ${statusMsg.type}`}>{statusMsg.text}</p>
      )}
    </div>
  );
}
