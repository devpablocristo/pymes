import { useEffect, useMemo, useState } from 'react';
import type { CrudFeatureFlags, CrudPageConfig, CrudViewModeId } from '../../components/CrudPage';
import {
  CRUD_UI_PREFERENCES_FEATURE_KEYS,
  createCrudUiPreferencesApi,
  type CrudUiResourceOverride,
} from '@devpablocristo/modules-crud-ui';

export type CrudUiPreferencesResource = { resourceId: string; label: string };
export type CrudUiPreferenceFeatureKey = readonly [keyof CrudFeatureFlags, string];

export type CrudUiPreferencesPanelCopy = {
  title?: string;
  hint?: string;
  defaultViewLabel?: string;
};

export type CrudUiPreferencesPanelProps = {
  storageKey: string;
  resources: readonly CrudUiPreferencesResource[];
  changeEventName?: string;
  loadPageConfig: (resourceId: string) => Promise<Pick<CrudPageConfig<{ id: string }>, 'viewModes'> | null>;
  copy?: CrudUiPreferencesPanelCopy;
  hideResourceCardHeader?: boolean;
  hideDefaultViewSelector?: boolean;
  featureKeys?: readonly CrudUiPreferenceFeatureKey[];
  classes?: {
    section?: string;
    hint?: string;
    stack?: string;
    grid?: string;
    checkboxRow?: string;
  };
};

const CANONICAL_VIEW_MODES: Array<{ id: CrudViewModeId; label: string }> = [
  { id: 'list', label: 'Lista' },
  { id: 'gallery', label: 'Galería' },
  { id: 'kanban', label: 'Tablero' },
];

export function CrudUiPreferencesPanel({
  storageKey,
  resources,
  changeEventName,
  loadPageConfig,
  copy = {},
  hideResourceCardHeader = false,
  hideDefaultViewSelector = false,
  featureKeys = CRUD_UI_PREFERENCES_FEATURE_KEYS,
  classes = {},
}: CrudUiPreferencesPanelProps) {
  const api = useMemo(
    () =>
      createCrudUiPreferencesApi({
        storageKey,
        knownResourceIds: resources.map((r) => r.resourceId),
        changeEventName,
      }),
    [changeEventName, resources, storageKey],
  );

  const [state, setState] = useState<Record<string, CrudUiResourceOverride>>(() => api.readState());
  const [configs, setConfigs] = useState<Record<string, { label: string; viewModes: Array<{ id: CrudViewModeId; label: string }> }>>({});

  useEffect(() => {
    let cancelled = false;
    void Promise.all(
      resources.map(async (resource) => {
        const config = await loadPageConfig(resource.resourceId);
        return {
          resourceId: resource.resourceId,
          label: resource.label,
          viewModes: CANONICAL_VIEW_MODES.map((mode) => ({
            id: mode.id,
            label: config?.viewModes?.find((entry) => entry.id === mode.id)?.label ?? mode.label,
          })),
        };
      }),
    ).then((rows) => {
      if (cancelled) return;
      setConfigs(Object.fromEntries(rows.map((row) => [row.resourceId, { label: row.label, viewModes: row.viewModes }])));
    });
    return () => {
      cancelled = true;
    };
  }, [loadPageConfig, resources]);

  useEffect(() => {
    const onExternal = () => setState(api.readState());
    window.addEventListener(api.changeEventName, onExternal);
    return () => window.removeEventListener(api.changeEventName, onExternal);
  }, [api]);

  function updateResource(resourceId: string, next: CrudUiResourceOverride | undefined) {
    const merged: Record<string, CrudUiResourceOverride> = { ...state };
    if (next === undefined) delete merged[resourceId];
    else merged[resourceId] = next;
    setState(merged);
    api.writeState(merged);
  }

  return (
    <section className={classes.section}>
      {copy.title ? <h3>{copy.title}</h3> : null}
      {copy.hint ? <p className={classes.hint}>{copy.hint}</p> : null}
      <div className={classes.stack}>
        {resources.map((resource) => {
          const config = configs[resource.resourceId];
          const override = state[resource.resourceId] ?? {};
          const enabled = new Set(override.enabledViewModeIds ?? config?.viewModes.map((mode) => mode.id) ?? []);
          const defaultId = override.defaultViewModeId ?? config?.viewModes[0]?.id;
          const featureFlags = override.featureFlags ?? {};
          const rowExtra = classes.checkboxRow ? ` ${classes.checkboxRow}` : '';

          return (
            <div key={resource.resourceId} className="card">
              {!hideResourceCardHeader ? (
                <div className="card-header">
                  <h4>{config?.label ?? resource.label}</h4>
                </div>
              ) : null}
              <div className={classes.grid}>
                <div className="crud-ui-pref-grid">
                  {(config?.viewModes ?? []).map((mode) => (
                    <label key={mode.id} className={`crud-ui-pref-row${rowExtra}`}>
                      <span className="crud-ui-pref-row__label">{mode.label}</span>
                      <span className="crud-ui-pref-switch">
                        <input
                          type="checkbox"
                          role="switch"
                          className="crud-ui-pref-switch__input"
                          checked={enabled.has(mode.id)}
                          onChange={(e) => {
                            const nextEnabled = new Set(enabled);
                            if (e.target.checked) nextEnabled.add(mode.id);
                            else nextEnabled.delete(mode.id);
                            const enabledViewModeIds = Array.from(nextEnabled) as CrudViewModeId[];
                            updateResource(resource.resourceId, {
                              ...override,
                              enabledViewModeIds,
                              defaultViewModeId:
                                override.defaultViewModeId && enabledViewModeIds.includes(override.defaultViewModeId)
                                  ? override.defaultViewModeId
                                  : enabledViewModeIds[0],
                            });
                          }}
                        />
                        <span className="crud-ui-pref-switch__slider" aria-hidden />
                      </span>
                    </label>
                  ))}
                  {featureKeys.map(([flag, label]) => (
                    <label key={flag} className={`crud-ui-pref-row${rowExtra}`}>
                      <span className="crud-ui-pref-row__label">{label}</span>
                      <span className="crud-ui-pref-switch">
                        <input
                          type="checkbox"
                          role="switch"
                          className="crud-ui-pref-switch__input"
                          checked={featureFlags[flag] !== false}
                          onChange={(e) =>
                            updateResource(resource.resourceId, {
                              ...override,
                              featureFlags: {
                                ...featureFlags,
                                [flag]: e.target.checked,
                              },
                            })
                          }
                        />
                        <span className="crud-ui-pref-switch__slider" aria-hidden />
                      </span>
                    </label>
                  ))}
                </div>
              </div>
              {!hideDefaultViewSelector ? (
                <div className={classes.grid}>
                  <div className="form-group">
                    {copy.defaultViewLabel ? <label>{copy.defaultViewLabel}</label> : null}
                    <select
                      value={defaultId ?? ''}
                      onChange={(e) =>
                        updateResource(resource.resourceId, {
                          ...override,
                          defaultViewModeId: e.target.value as CrudViewModeId,
                        })
                      }
                    >
                      {(config?.viewModes ?? [])
                        .filter((mode) => enabled.has(mode.id))
                        .map((mode) => (
                          <option key={mode.id} value={mode.id}>
                            {mode.label}
                          </option>
                        ))}
                    </select>
                  </div>
                </div>
              ) : null}
            </div>
          );
        })}
      </div>
    </section>
  );
}
