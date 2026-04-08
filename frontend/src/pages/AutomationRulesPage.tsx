import { useCallback, useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { useSearch } from '@devpablocristo/modules-search';
import { listPolicies, createPolicy, deletePolicy, type PolicyResponse } from '../lib/reviewApi';
import { queryKeys } from '../lib/queryKeys';
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
  {
    actionType: 'scheduling.book',
    displayName: 'Agendar turno',
    category: 'Turnos',
    riskClass: 'low',
    hasThreshold: false,
  },
  {
    actionType: 'scheduling.reschedule',
    displayName: 'Reagendar turno',
    category: 'Turnos',
    riskClass: 'low',
    hasThreshold: false,
  },
  {
    actionType: 'scheduling.cancel',
    displayName: 'Cancelar turno',
    category: 'Turnos',
    riskClass: 'medium',
    hasThreshold: false,
  },
  {
    actionType: 'discount.apply',
    displayName: 'Aplicar descuento',
    category: 'Descuentos',
    riskClass: 'medium',
    hasThreshold: true,
    thresholdLabel: '%',
    thresholdUnit: 'percentage_lte',
    thresholdPattern: 'percentage_gt',
    defaultThreshold: 10,
  },
  {
    actionType: 'payment_link.generate',
    displayName: 'Generar link de pago',
    category: 'Pagos',
    riskClass: 'low',
    hasThreshold: false,
  },
  { actionType: 'refund.create', displayName: 'Reembolso', category: 'Pagos', riskClass: 'high', hasThreshold: false },
  {
    actionType: 'sale.create',
    displayName: 'Crear venta',
    category: 'Ventas',
    riskClass: 'medium',
    hasThreshold: false,
  },
  {
    actionType: 'quote.create',
    displayName: 'Crear presupuesto',
    category: 'Ventas',
    riskClass: 'low',
    hasThreshold: false,
  },
  {
    actionType: 'notification.bulk_send',
    displayName: 'Envío masivo',
    category: 'Notificaciones',
    riskClass: 'medium',
    hasThreshold: false,
  },
];

interface RuleState {
  effect: Effect;
  threshold: number;
  policyId?: string;
}

const EFFECT_LABELS: Record<Effect, string> = {
  allow: 'Automático',
  require_approval: 'Pedirme',
  deny: 'No permitir',
};

const DEFAULT_EFFECTS: Record<string, Effect> = {
  'scheduling.book': 'allow',
  'scheduling.reschedule': 'allow',
  'scheduling.cancel': 'require_approval',
  'discount.apply': 'allow',
  'payment_link.generate': 'allow',
  'refund.create': 'deny',
  'sale.create': 'require_approval',
  'quote.create': 'allow',
  'notification.bulk_send': 'require_approval',
};

export default function AutomationRulesPage() {
  const [rules, setRules] = useState<Record<string, RuleState>>({});
  const [statusMsg, setStatusMsg] = useState<{ text: string; type: 'success' | 'error' } | null>(null);
  const queryClient = useQueryClient();
  const policiesQuery = useQuery({
    queryKey: queryKeys.review.policies,
    queryFn: listPolicies,
    retry: false,
  });

  const buildRulesState = useCallback((policies: PolicyResponse[] | undefined): Record<string, RuleState> => {
    const state: Record<string, RuleState> = {};
    for (const tpl of RULE_TEMPLATES) {
      const matching = (policies ?? []).filter((p) => p.action_type === tpl.actionType);
      const policy = matching.length > 0 ? matching[0] : null;
      state[tpl.actionType] = {
        effect: (policy?.effect as Effect) || DEFAULT_EFFECTS[tpl.actionType] || 'require_approval',
        threshold: tpl.defaultThreshold || 0,
        policyId: policy?.id,
      };
    }
    return state;
  }, []);

  useEffect(() => {
    setRules(buildRulesState(policiesQuery.data?.policies));
  }, [buildRulesState, policiesQuery.data]);

  const saveMutation = useMutation({
    mutationFn: async (draft: Record<string, RuleState>) => {
      for (const tpl of RULE_TEMPLATES) {
        const rule = draft[tpl.actionType];
        if (!rule) continue;

        if (rule.policyId) {
          await deletePolicy(rule.policyId);
        }

        let condition: string | undefined;
        if (tpl.hasThreshold && tpl.thresholdPattern && rule.threshold > 0) {
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
    },
    onSuccess: async () => {
      setStatusMsg({ text: 'Reglas guardadas', type: 'success' });
      await queryClient.invalidateQueries({ queryKey: queryKeys.review.policies });
    },
    onError: () => {
      setStatusMsg({ text: 'Error al guardar las reglas', type: 'error' });
    },
  });

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
    setStatusMsg(null);
    await saveMutation.mutateAsync(rules);
  };

  const ruleSearch = usePageSearch();
  const ruleTextFn = useCallback((tpl: RuleTemplate) => `${tpl.displayName} ${tpl.category} ${tpl.actionType}`, []);
  const filteredRules = useSearch(RULE_TEMPLATES, ruleTextFn, ruleSearch);
  const categories = [...new Set(filteredRules.map((tpl) => tpl.category))];

  if (policiesQuery.isLoading) {
    return (
      <div className="automation-rules-page">
        <div className="loading-wrap">Cargando reglas…</div>
      </div>
    );
  }

  return (
    <PageLayout
      className="automation-rules-page"
      title="Reglas de automatización"
      lead="Qué puede hacer la IA o los usuarios sin tu aprobación, según el tipo de acción."
    >
      <div className="rules-stack">
        {categories.map((cat) => (
          <div key={cat} className="rules-category">
            {filteredRules
              .filter((t) => t.category === cat)
              .map((tpl) => {
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
                            aria-label={`Umbral para ${tpl.displayName}`}
                            value={rule.threshold}
                            onChange={(e) => handleThresholdChange(tpl.actionType, Number(e.target.value))}
                          />
                          <span>{tpl.thresholdLabel}</span>
                        </div>
                      )}
                      <select
                        className={effectClass}
                        aria-label={`Acción para ${tpl.displayName}`}
                        value={rule.effect}
                        onChange={(e) => handleEffectChange(tpl.actionType, e.target.value as Effect)}
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
      </div>

      <div className="save-bar">
        <button type="button" className="btn-primary" onClick={handleSave} disabled={saveMutation.isPending}>
          {saveMutation.isPending ? 'Guardando…' : 'Guardar cambios'}
        </button>
      </div>

      {statusMsg && (
        <div
          className={`automation-status alert ${statusMsg.type === 'success' ? 'alert-success' : 'alert-error'}`}
          role={statusMsg.type === 'success' ? 'status' : 'alert'}
        >
          {statusMsg.text}
        </div>
      )}
    </PageLayout>
  );
}
