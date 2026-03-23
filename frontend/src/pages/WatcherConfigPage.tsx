import { useCallback, useEffect, useState } from 'react';
import { listWatchers, updateWatcher, type WatcherResponse } from '../lib/reviewApi';
import './WatcherConfigPage.css';

interface WatcherTemplate {
  watcherType: string;
  name: string;
  description: string;
  thresholdKey: string;
  thresholdLabel: string;
  thresholdUnit: string;
  defaultThreshold: number;
  hasThreshold: boolean;
}

const WATCHER_TEMPLATES: WatcherTemplate[] = [
  {
    watcherType: 'stale_work_orders',
    name: 'Avisar clientes con OT demorada',
    description: 'Notifica al cliente cuando su orden de trabajo lleva mucho tiempo sin avanzar',
    thresholdKey: 'threshold_days',
    thresholdLabel: 'Mas de',
    thresholdUnit: 'dias sin avanzar',
    defaultThreshold: 3,
    hasThreshold: true,
  },
  {
    watcherType: 'unconfirmed_appointments',
    name: 'Recordar turnos no confirmados',
    description: 'Envia recordatorio el dia anterior al turno si no fue confirmado',
    thresholdKey: 'hours_before_appointment',
    thresholdLabel: '',
    thresholdUnit: '',
    defaultThreshold: 24,
    hasThreshold: false,
  },
  {
    watcherType: 'low_stock',
    name: 'Alertar stock bajo',
    description: 'Alerta cuando un producto tiene pocas unidades disponibles',
    thresholdKey: 'threshold_units',
    thresholdLabel: 'Menos de',
    thresholdUnit: 'unidades',
    defaultThreshold: 5,
    hasThreshold: true,
  },
  {
    watcherType: 'inactive_customers',
    name: 'Contactar clientes inactivos',
    description: 'Envia mensaje a clientes que no visitan hace mucho tiempo',
    thresholdKey: 'threshold_months',
    thresholdLabel: 'Sin visita hace mas de',
    thresholdUnit: 'meses',
    defaultThreshold: 6,
    hasThreshold: true,
  },
  {
    watcherType: 'revenue_drop',
    name: 'Alerta de caida de facturacion',
    description: 'Notifica si la facturacion cae significativamente respecto al mes anterior',
    thresholdKey: 'threshold_percent',
    thresholdLabel: 'Mas de',
    thresholdUnit: '% de caida',
    defaultThreshold: 20,
    hasThreshold: true,
  },
];

interface WatcherState {
  enabled: boolean;
  threshold: number;
  watcherId?: string;
  lastRunAt?: string | null;
  lastResult?: { found: number; proposed: number; executed: number } | null;
}

export default function WatcherConfigPage() {
  const [watchers, setWatchers] = useState<Record<string, WatcherState>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [statusMsg, setStatusMsg] = useState<{ text: string; type: 'success' | 'error' } | null>(null);

  const loadWatchers = useCallback(async () => {
    try {
      const resp = await listWatchers();
      const items = resp.watchers || [];
      const state: Record<string, WatcherState> = {};

      for (const tpl of WATCHER_TEMPLATES) {
        const match = items.find((w: WatcherResponse) => w.watcher_type === tpl.watcherType);
        if (match) {
          const config = (match.config || {}) as Record<string, number>;
          state[tpl.watcherType] = {
            enabled: match.enabled,
            threshold: config[tpl.thresholdKey] ?? tpl.defaultThreshold,
            watcherId: match.id,
            lastRunAt: match.last_run_at,
            lastResult: match.last_result,
          };
        } else {
          state[tpl.watcherType] = {
            enabled: false,
            threshold: tpl.defaultThreshold,
          };
        }
      }
      setWatchers(state);
    } catch {
      const state: Record<string, WatcherState> = {};
      for (const tpl of WATCHER_TEMPLATES) {
        state[tpl.watcherType] = { enabled: false, threshold: tpl.defaultThreshold };
      }
      setWatchers(state);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadWatchers();
  }, [loadWatchers]);

  const handleToggle = (watcherType: string) => {
    setWatchers((prev) => ({
      ...prev,
      [watcherType]: { ...prev[watcherType], enabled: !prev[watcherType].enabled },
    }));
    setStatusMsg(null);
  };

  const handleThresholdChange = (watcherType: string, value: number) => {
    setWatchers((prev) => ({
      ...prev,
      [watcherType]: { ...prev[watcherType], threshold: value },
    }));
    setStatusMsg(null);
  };

  const handleSave = async () => {
    setSaving(true);
    setStatusMsg(null);
    try {
      for (const tpl of WATCHER_TEMPLATES) {
        const state = watchers[tpl.watcherType];
        if (!state || !state.watcherId) continue;
        await updateWatcher(
          state.watcherId,
          { [tpl.thresholdKey]: state.threshold },
          state.enabled,
        );
      }
      setStatusMsg({ text: 'Configuracion guardada', type: 'success' });
    } catch {
      setStatusMsg({ text: 'Error al guardar', type: 'error' });
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <div className="watcher-config-page"><div className="loading">Cargando configuracion...</div></div>;
  }

  return (
    <div className="watcher-config-page">
      <h1>Asistente proactivo</h1>
      <p className="subtitle">Configura alertas y acciones automaticas</p>

      {WATCHER_TEMPLATES.map((tpl) => {
        const state = watchers[tpl.watcherType];
        if (!state) return null;
        return (
          <div
            key={tpl.watcherType}
            className={`watcher-card ${!state.enabled ? 'disabled' : ''}`}
          >
            <input
              type="checkbox"
              className="watcher-toggle"
              checked={state.enabled}
              onChange={() => handleToggle(tpl.watcherType)}
            />
            <div className="watcher-content">
              <div className="watcher-name">{tpl.name}</div>
              <div className="watcher-desc">{tpl.description}</div>
              {tpl.hasThreshold && (
                <div className="watcher-threshold">
                  <span>{tpl.thresholdLabel}</span>
                  <input
                    type="number"
                    min={1}
                    value={state.threshold}
                    onChange={(e) =>
                      handleThresholdChange(tpl.watcherType, Number(e.target.value))
                    }
                    disabled={!state.enabled}
                  />
                  <span>{tpl.thresholdUnit}</span>
                </div>
              )}
              {state.lastRunAt && (
                <div className="watcher-last-run">
                  Ultimo chequeo: {new Date(state.lastRunAt).toLocaleString('es-AR')}
                  {state.lastResult && (
                    <span>
                      {' '}— Encontrados: {state.lastResult.found}, Ejecutados: {state.lastResult.executed}
                    </span>
                  )}
                </div>
              )}
            </div>
          </div>
        );
      })}

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
