import { useClerk, useOrganization, useSession } from '@clerk/react';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { clerkEnabled } from '../lib/auth';
import { formatClerkAPIUserMessage } from '../lib/clerkErrors';
import { useI18n } from '../lib/i18n';
import {
  saveTenantProfile,
  type PaymentMethod,
  type SellsType,
  type TeamSize,
  type TenantProfile,
  type VerticalType,
} from '../lib/tenantProfile';

// Verticales principales (primer selector)
type VerticalGroup = 'commercial' | 'professionals' | 'workshops' | 'beauty' | 'restaurants';

const VERTICAL_GROUP_OPTIONS: { value: VerticalGroup; label: string; desc: string }[] = [
  { value: 'commercial', label: 'Solo comercial', desc: 'Ventas, stock y cobros' },
  { value: 'professionals', label: 'Profesionales / Docentes', desc: 'Sesiones, alumnos y fichas' },
  { value: 'workshops', label: 'Talleres', desc: 'Vehículos, bicicletas, reparaciones' },
  { value: 'beauty', label: 'Belleza / Salón', desc: 'Equipo, servicios y agenda' },
  { value: 'restaurants', label: 'Bares / Restaurantes', desc: 'Salón, mesas y sesiones' },
];

// Sub-verticales por grupo (solo los que tienen más de una opción)
const SUB_VERTICAL_OPTIONS: Partial<Record<VerticalGroup, { value: VerticalType; label: string; desc: string }[]>> = {
  workshops: [
    { value: 'workshops', label: 'Taller mecánico', desc: 'Vehículos, órdenes y servicios' },
    { value: 'bike_shop', label: 'Bicicletería', desc: 'Bicicletas, reparaciones y servicios' },
  ],
};

// Mapeo directo grupo → vertical para los que no tienen sub-verticales
const GROUP_TO_VERTICAL: Partial<Record<VerticalGroup, VerticalType>> = {
  commercial: 'none',
  professionals: 'professionals',
  beauty: 'beauty',
  restaurants: 'restaurants',
};

// Para el resumen final
const ALL_VERTICAL_LABELS: Record<VerticalType, string> = {
  none: 'Solo comercial',
  professionals: 'Profesionales / Docentes',
  workshops: 'Taller mecánico',
  bike_shop: 'Bicicletería',
  beauty: 'Belleza / Salón',
  restaurants: 'Bares / Restaurantes',
};

type Step = 1 | 2 | 3 | 4;

const TEAM_OPTIONS: { value: TeamSize; label: string; desc: string }[] = [
  { value: 'solo', label: 'Solo yo', desc: 'Trabajo por mi cuenta' },
  { value: 'small', label: '2 a 5', desc: 'Equipo chico' },
  { value: 'medium', label: '6 a 20', desc: 'Equipo mediano' },
  { value: 'large', label: 'Más de 20', desc: 'Empresa' },
];

const SELLS_OPTIONS: { value: SellsType; label: string; desc: string }[] = [
  { value: 'products', label: 'Productos', desc: 'Vendo cosas físicas, tengo stock' },
  { value: 'services', label: 'Servicios', desc: 'Cobro por hora, sesión o proyecto' },
  { value: 'both', label: 'Ambos', desc: 'Productos y servicios' },
  { value: 'unsure', label: 'Todavía no sé', desc: 'Estoy explorando' },
];

const CLIENT_LABELS = ['clientes', 'pacientes', 'alumnos', 'usuarios'];

const CURRENCY_OPTIONS = [
  { value: 'ARS', label: 'Peso argentino (ARS)' },
  { value: 'USD', label: 'Dólar (USD)' },
  { value: 'EUR', label: 'Euro (EUR)' },
  { value: 'BRL', label: 'Real (BRL)' },
  { value: 'MXN', label: 'Peso mexicano (MXN)' },
  { value: 'CLP', label: 'Peso chileno (CLP)' },
  { value: 'COP', label: 'Peso colombiano (COP)' },
];

const PAYMENT_OPTIONS: { value: PaymentMethod; label: string }[] = [
  { value: 'cash', label: 'Efectivo' },
  { value: 'transfer', label: 'Transferencia' },
  { value: 'card', label: 'Tarjeta' },
  { value: 'mixed', label: 'Mixto (varios)' },
];

/** Recursos Clerk mínimos para crear la org al terminar el onboarding (si aún no hay una activa). */
type ClerkOnboardingBridges = {
  loaded: boolean;
  createOrganization: (params: { name: string }) => Promise<{ id: string }>;
  setActive: (params: { organization: string }) => Promise<void>;
  /** Si ya hay org (p. ej. invitación), no la creamos ni renombramos aquí. */
  organization: { id: string } | null;
  orgLoaded: boolean;
  /** Tras crear org y setActive, renueva la sesión para que el JWT traiga el claim de organización. */
  afterSetActiveOrg?: () => Promise<void>;
};

function OnboardingPageClerkBridge() {
  const clerk = useClerk();
  const { session } = useSession();
  const { organization, isLoaded: orgLoaded } = useOrganization();

  const bridges: ClerkOnboardingBridges = {
    loaded: clerk.loaded,
    createOrganization: (params) => clerk.createOrganization(params),
    setActive: (params) => clerk.setActive(params),
    organization: organization ? { id: organization.id } : null,
    orgLoaded,
    afterSetActiveOrg: session
      ? async () => {
          await session.reload();
        }
      : undefined,
  };

  return <OnboardingPageInner clerkBridges={bridges} />;
}

export function OnboardingPage() {
  if (!clerkEnabled) {
    return <OnboardingPageInner clerkBridges={null} />;
  }
  return <OnboardingPageClerkBridge />;
}

function OnboardingPageInner({ clerkBridges }: { clerkBridges: ClerkOnboardingBridges | null }) {
  const navigate = useNavigate();
  const { t } = useI18n();
  const [step, setStep] = useState<Step>(1);

  const [businessName, setBusinessName] = useState('');
  const [teamSize, setTeamSize] = useState<TeamSize | ''>('');
  const [sells, setSells] = useState<SellsType | ''>('');
  const [clientLabel, setClientLabel] = useState('clientes');
  const [customClientLabel, setCustomClientLabel] = useState('');
  const [usesScheduling, setUsesScheduling] = useState<boolean | null>(null);
  const [usesBilling, setUsesBilling] = useState<boolean | null>(null);
  const [currency, setCurrency] = useState('ARS');
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod | ''>('');
  const [verticalGroup, setVerticalGroup] = useState<VerticalGroup | ''>('');
  const [vertical, setVertical] = useState<VerticalType | ''>('');

  const [finishing, setFinishing] = useState(false);
  const [finishError, setFinishError] = useState('');

  // Resolver vertical final: si el grupo tiene sub-verticales, usar la selección; si no, mapeo directo
  const resolvedVertical: VerticalType | '' = verticalGroup
    ? (SUB_VERTICAL_OPTIONS[verticalGroup] ? vertical : GROUP_TO_VERTICAL[verticalGroup] ?? '')
    : '';

  const needsSubVertical = verticalGroup !== '' && !!SUB_VERTICAL_OPTIONS[verticalGroup];

  const canNext: Record<Step, boolean> = {
    1: businessName.trim().length >= 2 && teamSize !== '' && verticalGroup !== '' && (!needsSubVertical || vertical !== ''),
    2: sells !== '' && (clientLabel !== '' || customClientLabel.trim() !== '') && usesScheduling !== null && usesBilling !== null,
    3: currency !== '' && paymentMethod !== '',
    4: true,
  };

  const clerkReady = !clerkBridges || (clerkBridges.loaded && clerkBridges.orgLoaded);
  const canFinishStep4 = canNext[4] && !finishing && clerkReady;

  function next() {
    if (step < 4) setStep((step + 1) as Step);
  }

  function back() {
    if (step > 1) setStep((step - 1) as Step);
  }

  async function finish() {
    const profile: TenantProfile = {
      businessName: businessName.trim(),
      teamSize: teamSize as TeamSize,
      sells: sells as SellsType,
      clientLabel: clientLabel === '__custom' ? customClientLabel.trim() : clientLabel,
      usesScheduling: usesScheduling === true,
      usesBilling: usesBilling === true,
      currency,
      paymentMethod: paymentMethod as PaymentMethod,
      vertical: resolvedVertical as VerticalType,
      completedAt: new Date().toISOString(),
    };

    setFinishError('');

    if (clerkBridges) {
      if (!clerkBridges.loaded || !clerkBridges.orgLoaded) {
        setFinishError(t('onboarding.clerk.sessionNotReady'));
        return;
      }
      setFinishing(true);
      try {
        const name = profile.businessName.trim();
        if (!clerkBridges.organization) {
          const created = await clerkBridges.createOrganization({ name });
          await clerkBridges.setActive({ organization: created.id });
          await clerkBridges.afterSetActiveOrg?.();
        }
      } catch (err) {
        setFinishError(
          formatClerkAPIUserMessage(err, t('onboarding.clerk.organizationFailed')),
        );
        return;
      } finally {
        setFinishing(false);
      }
    }

    saveTenantProfile(profile);
    navigate('/', { replace: true });
  }

  const resolvedClientLabel = clientLabel === '__custom' ? (customClientLabel.trim() || 'clientes') : clientLabel;

  return (
    <div className="onboarding-layout">
      <div className="onboarding-container">
        <div className="onboarding-header">
          <h1>Configurá tu espacio</h1>
          <p>Unas preguntas rápidas para armar tu panel a medida.</p>
          <div className="onboarding-progress">
            {[1, 2, 3, 4].map((s) => (
              <span key={s} className={`onboarding-dot${s === step ? ' active' : ''}${s < step ? ' done' : ''}`} />
            ))}
          </div>
        </div>

        <div className="onboarding-body">
          {step === 1 && (
            <div className="onboarding-step">
              <h2>Tu negocio</h2>

              <div className="onboarding-field">
                <label>¿Cómo se llama tu negocio o actividad?</label>
                <input
                  type="text"
                  placeholder="Ej: Clases de inglés, Estudio López, Mi emprendimiento..."
                  value={businessName}
                  onChange={(e) => setBusinessName(e.target.value)}
                  autoFocus
                />
              </div>

              <div className="onboarding-field">
                <label>¿Cuántas personas trabajan?</label>
                <div className="onboarding-options">
                  {TEAM_OPTIONS.map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      className={`onboarding-option${teamSize === opt.value ? ' selected' : ''}`}
                      onClick={() => setTeamSize(opt.value)}
                    >
                      <strong>{opt.label}</strong>
                      <small>{opt.desc}</small>
                    </button>
                  ))}
                </div>
              </div>

              <div className="onboarding-field">
                <label>¿Qué tipo de negocio es?</label>
                <div className="onboarding-options onboarding-options-vertical">
                  {VERTICAL_GROUP_OPTIONS.map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      className={`onboarding-option${verticalGroup === opt.value ? ' selected' : ''}`}
                      onClick={() => {
                        setVerticalGroup(opt.value);
                        setVertical('');
                      }}
                    >
                      <strong>{opt.label}</strong>
                      <small>{opt.desc}</small>
                    </button>
                  ))}
                </div>
              </div>

              {needsSubVertical && (
                <div className="onboarding-field">
                  <label>¿Qué tipo de taller?</label>
                  <div className="onboarding-options onboarding-options-vertical">
                    {SUB_VERTICAL_OPTIONS[verticalGroup as VerticalGroup]!.map((opt) => (
                      <button
                        key={opt.value}
                        type="button"
                        className={`onboarding-option${vertical === opt.value ? ' selected' : ''}`}
                        onClick={() => setVertical(opt.value)}
                      >
                        <strong>{opt.label}</strong>
                        <small>{opt.desc}</small>
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {step === 2 && (
            <div className="onboarding-step">
              <h2>Tu actividad</h2>

              <div className="onboarding-field">
                <label>¿Qué ofrecés?</label>
                <div className="onboarding-options">
                  {SELLS_OPTIONS.map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      className={`onboarding-option${sells === opt.value ? ' selected' : ''}`}
                      onClick={() => setSells(opt.value)}
                    >
                      <strong>{opt.label}</strong>
                      <small>{opt.desc}</small>
                    </button>
                  ))}
                </div>
              </div>

              <div className="onboarding-field">
                <label>¿Cómo les decís a las personas que te contratan?</label>
                <div className="onboarding-chips">
                  {CLIENT_LABELS.map((lbl) => (
                    <button
                      key={lbl}
                      type="button"
                      className={`onboarding-chip${clientLabel === lbl ? ' selected' : ''}`}
                      onClick={() => {
                        setClientLabel(lbl);
                        setCustomClientLabel('');
                      }}
                    >
                      {lbl}
                    </button>
                  ))}
                  <button
                    type="button"
                    className={`onboarding-chip${clientLabel === '__custom' ? ' selected' : ''}`}
                    onClick={() => setClientLabel('__custom')}
                  >
                    otro...
                  </button>
                </div>
                {clientLabel === '__custom' && (
                  <input
                    type="text"
                    placeholder="¿Cómo les decís?"
                    value={customClientLabel}
                    onChange={(e) => setCustomClientLabel(e.target.value)}
                    autoFocus
                  />
                )}
              </div>

              <div className="onboarding-field">
                <label>¿Agendás turnos o sesiones con tus {resolvedClientLabel}?</label>
                <div className="onboarding-chips">
                  <button
                    type="button"
                    className={`onboarding-chip${usesScheduling === true ? ' selected' : ''}`}
                    onClick={() => setUsesScheduling(true)}
                  >
                    Sí
                  </button>
                  <button
                    type="button"
                    className={`onboarding-chip${usesScheduling === false ? ' selected' : ''}`}
                    onClick={() => setUsesScheduling(false)}
                  >
                    No
                  </button>
                </div>
              </div>

              <div className="onboarding-field">
                <label>¿Querés llevar control de cobros y pagos?</label>
                <div className="onboarding-chips">
                  <button
                    type="button"
                    className={`onboarding-chip${usesBilling === true ? ' selected' : ''}`}
                    onClick={() => setUsesBilling(true)}
                  >
                    Sí, quiero saber quién me debe y cuánto cobré
                  </button>
                  <button
                    type="button"
                    className={`onboarding-chip${usesBilling === false ? ' selected' : ''}`}
                    onClick={() => setUsesBilling(false)}
                  >
                    No, por ahora no
                  </button>
                </div>
              </div>
            </div>
          )}

          {step === 3 && (
            <div className="onboarding-step">
              <h2>Moneda y cobro</h2>

              <div className="onboarding-field">
                <label>¿En qué moneda operás?</label>
                <select value={currency} onChange={(e) => setCurrency(e.target.value)}>
                  {CURRENCY_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </div>

              <div className="onboarding-field">
                <label>¿Cómo cobrás principalmente?</label>
                <div className="onboarding-options onboarding-options-row">
                  {PAYMENT_OPTIONS.map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      className={`onboarding-option compact${paymentMethod === opt.value ? ' selected' : ''}`}
                      onClick={() => setPaymentMethod(opt.value)}
                    >
                      <strong>{opt.label}</strong>
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}

          {step === 4 && (
            <div className="onboarding-step">
              <h2>Todo listo</h2>
              <p className="onboarding-summary-intro">
                Vamos a configurar tu panel con esta información. Podés cambiarlo cuando quieras.
              </p>

              <div className="onboarding-summary">
                <div className="onboarding-summary-row">
                  <span>Negocio</span>
                  <strong>{businessName}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>Equipo</span>
                  <strong>{TEAM_OPTIONS.find((o) => o.value === teamSize)?.label ?? teamSize}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>Tipo de negocio</span>
                  <strong>{resolvedVertical ? ALL_VERTICAL_LABELS[resolvedVertical] : '-'}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>Ofrecés</span>
                  <strong>{SELLS_OPTIONS.find((o) => o.value === sells)?.label ?? sells}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>Les decís</span>
                  <strong>{resolvedClientLabel}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>Agenda turnos</span>
                  <strong>{usesScheduling ? 'Sí' : 'No'}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>Control de cobros</span>
                  <strong>{usesBilling ? 'Sí' : 'No'}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>Moneda</span>
                  <strong>{currency}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>Cobro</span>
                  <strong>{PAYMENT_OPTIONS.find((o) => o.value === paymentMethod)?.label ?? paymentMethod}</strong>
                </div>
              </div>
              {finishError && <p className="alert alert-error onboarding-finish-error">{finishError}</p>}
            </div>
          )}
        </div>

        <div className="onboarding-footer">
          {step > 1 ? (
            <button type="button" className="onboarding-btn-back" onClick={back}>
              Atrás
            </button>
          ) : (
            <span />
          )}
          {step < 4 ? (
            <button
              type="button"
              className="onboarding-btn-next"
              disabled={!canNext[step]}
              onClick={next}
            >
              Siguiente
            </button>
          ) : (
            <button
              type="button"
              className="onboarding-btn-next onboarding-btn-finish"
              disabled={!canFinishStep4}
              onClick={() => void finish()}
            >
              {finishing ? t('common.status.saving') : 'Empezar'}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
