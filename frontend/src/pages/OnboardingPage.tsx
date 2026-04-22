import { useClerk, useOrganization, useSession } from '@clerk/react';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { clerkEnabled } from '../lib/auth';
import { createSchedulingBranch, listSchedulingBranches, updateTenantSettings } from '../lib/api';
import { formatClerkAPIUserMessage } from '../lib/clerkErrors';
import { useI18n } from '../lib/i18n';
import { queryKeys } from '../lib/queryKeys';
import {
  saveTenantProfile,
  syncTenantProfileFromSettings,
  type PaymentMethod,
  type SellsType,
  type SubVerticalType,
  type TeamSize,
  type TenantProfile,
  type VerticalType,
} from '../lib/tenantProfile';

type VerticalGroup = 'commercial' | 'professionals' | 'workshops' | 'beauty' | 'restaurants';
type OnboardingSubVerticalOption = {
  value: string;
  vertical: VerticalType;
  labelKey: string;
  descKey: string;
};

const VERTICAL_GROUP_KEYS: { value: VerticalGroup; labelKey: string; descKey: string }[] = [
  { value: 'commercial', labelKey: 'onboarding.vertical.commercial', descKey: 'onboarding.vertical.commercialDesc' },
  {
    value: 'professionals',
    labelKey: 'onboarding.vertical.professionals',
    descKey: 'onboarding.vertical.professionalsDesc',
  },
  { value: 'workshops', labelKey: 'onboarding.vertical.workshops', descKey: 'onboarding.vertical.workshopsDesc' },
  { value: 'beauty', labelKey: 'onboarding.vertical.beauty', descKey: 'onboarding.vertical.beautyDesc' },
  {
    value: 'restaurants',
    labelKey: 'onboarding.vertical.restaurants',
    descKey: 'onboarding.vertical.restaurantsDesc',
  },
];

const SUB_VERTICAL_KEYS: Partial<Record<VerticalGroup, OnboardingSubVerticalOption[]>> = {
  professionals: [
    {
      value: 'teachers',
      vertical: 'professionals',
      labelKey: 'onboarding.vertical.teachers',
      descKey: 'onboarding.vertical.teachersDesc',
    },
    {
      value: 'consulting',
      vertical: 'professionals',
      labelKey: 'onboarding.vertical.consulting',
      descKey: 'onboarding.vertical.consultingDesc',
    },
  ],
  workshops: [
    {
      value: 'auto_repair',
      vertical: 'workshops',
      labelKey: 'onboarding.vertical.autoRepair',
      descKey: 'onboarding.vertical.autoRepairDesc',
    },
    {
      value: 'bike_shop',
      vertical: 'workshops',
      labelKey: 'onboarding.vertical.bikeShop',
      descKey: 'onboarding.vertical.bikeShopDesc',
    },
  ],
  beauty: [
    {
      value: 'salon',
      vertical: 'beauty',
      labelKey: 'onboarding.vertical.salon',
      descKey: 'onboarding.vertical.salonDesc',
    },
    {
      value: 'barbershop',
      vertical: 'beauty',
      labelKey: 'onboarding.vertical.barbershop',
      descKey: 'onboarding.vertical.barbershopDesc',
    },
    {
      value: 'aesthetics',
      vertical: 'beauty',
      labelKey: 'onboarding.vertical.aesthetics',
      descKey: 'onboarding.vertical.aestheticsDesc',
    },
  ],
  restaurants: [
    {
      value: 'restaurant',
      vertical: 'restaurants',
      labelKey: 'onboarding.vertical.restaurant',
      descKey: 'onboarding.vertical.restaurantDesc',
    },
    {
      value: 'bar',
      vertical: 'restaurants',
      labelKey: 'onboarding.vertical.bar',
      descKey: 'onboarding.vertical.barDesc',
    },
    {
      value: 'cafe',
      vertical: 'restaurants',
      labelKey: 'onboarding.vertical.cafe',
      descKey: 'onboarding.vertical.cafeDesc',
    },
  ],
};

const GROUP_TO_VERTICAL: Partial<Record<VerticalGroup, VerticalType>> = {
  commercial: 'none',
  professionals: 'professionals',
  workshops: 'workshops',
  beauty: 'beauty',
  restaurants: 'restaurants',
};

const ALL_VERTICAL_LABEL_KEYS: Record<VerticalType, string> = {
  none: 'onboarding.vertical.commercial',
  professionals: 'onboarding.vertical.professionals',
  workshops: 'onboarding.vertical.workshops',
  beauty: 'onboarding.vertical.beauty',
  restaurants: 'onboarding.vertical.restaurants',
  medical: 'onboarding.vertical.medical',
};

type Step = 1 | 2 | 3 | 4;

function resolveDefaultBranchTimezone(): string {
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone?.trim();
  return timezone || 'UTC';
}

async function ensureDefaultBranchExists(): Promise<void> {
  const current = await listSchedulingBranches();
  if ((current.items ?? []).length > 0) {
    return;
  }
  const payload = {
    code: 'principal',
    name: 'Principal',
    timezone: resolveDefaultBranchTimezone(),
    active: true,
  };
  try {
    await createSchedulingBranch(payload);
  } catch (error) {
    const refreshed = await listSchedulingBranches();
    if ((refreshed.items ?? []).length > 0) {
      return;
    }
    throw error;
  }
}

const TEAM_KEYS: { value: TeamSize; labelKey: string; descKey: string }[] = [
  { value: 'solo', labelKey: 'onboarding.team.solo', descKey: 'onboarding.team.soloDesc' },
  { value: 'small', labelKey: 'onboarding.team.small', descKey: 'onboarding.team.smallDesc' },
  { value: 'medium', labelKey: 'onboarding.team.medium', descKey: 'onboarding.team.mediumDesc' },
  { value: 'large', labelKey: 'onboarding.team.large', descKey: 'onboarding.team.largeDesc' },
];

const SELLS_KEYS: { value: SellsType; labelKey: string; descKey: string }[] = [
  { value: 'products', labelKey: 'onboarding.sells.products', descKey: 'onboarding.sells.productsDesc' },
  { value: 'services', labelKey: 'onboarding.sells.services', descKey: 'onboarding.sells.servicesDesc' },
  { value: 'both', labelKey: 'onboarding.sells.both', descKey: 'onboarding.sells.bothDesc' },
  { value: 'unsure', labelKey: 'onboarding.sells.unsure', descKey: 'onboarding.sells.unsureDesc' },
];

const CLIENT_LABEL_KEYS = ['clientes', 'pacientes', 'alumnos', 'usuarios'] as const;

const CURRENCY_KEYS = ['ARS', 'USD', 'EUR', 'BRL', 'MXN', 'CLP', 'COP'] as const;

const PAYMENT_KEYS: { value: PaymentMethod; labelKey: string }[] = [
  { value: 'cash', labelKey: 'onboarding.payment.cash' },
  { value: 'transfer', labelKey: 'onboarding.payment.transfer' },
  { value: 'card', labelKey: 'onboarding.payment.card' },
  { value: 'mixed', labelKey: 'onboarding.payment.mixed' },
];

type ClerkOnboardingBridges = {
  loaded: boolean;
  createOrganization: (params: { name: string }) => Promise<{ id: string }>;
  setActive: (params: { organization: string }) => Promise<void>;
  organization: { id: string } | null;
  orgLoaded: boolean;
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
  const queryClient = useQueryClient();
  const { t } = useI18n();
  const [step, setStep] = useState<Step>(1);

  const [businessName, setBusinessName] = useState('');
  const [teamSize, setTeamSize] = useState<TeamSize | ''>('');
  const [sells, setSells] = useState<SellsType | ''>('');
  const [clientLabel, setClientLabel] = useState<string>('clientes');
  const [customClientLabel, setCustomClientLabel] = useState('');
  const [usesScheduling, setUsesScheduling] = useState<boolean | null>(null);
  const [usesBilling, setUsesBilling] = useState<boolean | null>(null);
  const [currency, setCurrency] = useState('ARS');
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod | ''>('');
  const [verticalGroup, setVerticalGroup] = useState<VerticalGroup | ''>('');
  const [subVertical, setSubVertical] = useState('');

  const [finishing, setFinishing] = useState(false);
  const [finishError, setFinishError] = useState('');

  const resolvedVertical: VerticalType | '' = verticalGroup
    ? SUB_VERTICAL_KEYS[verticalGroup]
      ? (SUB_VERTICAL_KEYS[verticalGroup]!.find((opt) => opt.value === subVertical)?.vertical ?? '')
      : (GROUP_TO_VERTICAL[verticalGroup] ?? '')
    : '';

  const needsSubVertical = verticalGroup !== '' && !!SUB_VERTICAL_KEYS[verticalGroup];
  const resolvedSubVerticalLabelKey = verticalGroup && subVertical
    ? SUB_VERTICAL_KEYS[verticalGroup]?.find((opt) => opt.value === subVertical)?.labelKey ?? null
    : null;

  const canNext: Record<Step, boolean> = {
    1:
      businessName.trim().length >= 2 &&
      teamSize !== '' &&
      verticalGroup !== '' &&
      (!needsSubVertical || subVertical !== ''),
    2:
      sells !== '' &&
      (clientLabel !== '' || customClientLabel.trim() !== '') &&
      usesScheduling !== null &&
      usesBilling !== null,
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
      ...(subVertical ? { subVertical: subVertical as SubVerticalType } : {}),
      completedAt: new Date().toISOString(),
    };

    setFinishError('');
    setFinishing(true);

    if (clerkBridges) {
      if (!clerkBridges.loaded || !clerkBridges.orgLoaded) {
        setFinishError(t('onboarding.clerk.sessionNotReady'));
        setFinishing(false);
        return;
      }
      try {
        const name = profile.businessName.trim();
        if (!clerkBridges.organization) {
          const created = await clerkBridges.createOrganization({ name });
          await clerkBridges.setActive({ organization: created.id });
          await clerkBridges.afterSetActiveOrg?.();
        }
      } catch (err) {
        setFinishError(formatClerkAPIUserMessage(err, t('onboarding.clerk.organizationFailed')));
        setFinishing(false);
        return;
      }
    }

    try {
      await ensureDefaultBranchExists();
      const updated = await updateTenantSettings({
        business_name: profile.businessName,
        team_size: profile.teamSize,
        sells: profile.sells,
        client_label: profile.clientLabel,
        scheduling_enabled: profile.usesScheduling,
        uses_billing: profile.usesBilling,
        currency: profile.currency,
        payment_method: profile.paymentMethod,
        vertical: profile.vertical,
        onboarding_completed_at: profile.completedAt,
      });
      queryClient.setQueryData(queryKeys.tenant.settings, updated);
      const syncedProfile = syncTenantProfileFromSettings(updated);
      saveTenantProfile({
        ...(syncedProfile ?? profile),
        ...(profile.subVertical ? { subVertical: profile.subVertical } : {}),
      });
      navigate('/', { replace: true });
    } catch (err) {
      setFinishError(
        err instanceof Error ? err.message : t('profile.error.unreachable'),
      );
    } finally {
      setFinishing(false);
    }
  }

  const resolvedClientLabel =
    clientLabel === '__custom'
      ? customClientLabel.trim() || t('onboarding.clientLabel.clientes')
      : t(`onboarding.clientLabel.${clientLabel}`);

  return (
    <div className="onboarding-layout">
      <div className="onboarding-container">
        <div className="onboarding-header">
          <h1>{t('onboarding.header.title')}</h1>
          <p>{t('onboarding.header.subtitle')}</p>
          <div className="onboarding-progress">
            {[1, 2, 3, 4].map((s) => (
              <span key={s} className={`onboarding-dot${s === step ? ' active' : ''}${s < step ? ' done' : ''}`} />
            ))}
          </div>
        </div>

        <div className="onboarding-body">
          {step === 1 && (
            <div className="onboarding-step">
              <h2>{t('onboarding.step1.title')}</h2>

              <div className="onboarding-field">
                <label htmlFor="onboarding-business-name">{t('onboarding.step1.businessName')}</label>
                <input
                  id="onboarding-business-name"
                  type="text"
                  placeholder={t('onboarding.step1.businessNamePlaceholder')}
                  value={businessName}
                  onChange={(e) => setBusinessName(e.target.value)}
                  autoFocus
                />
              </div>

              <div className="onboarding-field">
                <label>{t('onboarding.step1.teamSize')}</label>
                <div className="onboarding-options">
                  {TEAM_KEYS.map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      className={`onboarding-option${teamSize === opt.value ? ' selected' : ''}`}
                      onClick={() => setTeamSize(opt.value)}
                    >
                      <strong>{t(opt.labelKey)}</strong>
                      <small>{t(opt.descKey)}</small>
                    </button>
                  ))}
                </div>
              </div>

              <div className="onboarding-field">
                <label>{t('onboarding.step1.verticalGroup')}</label>
                <div className="onboarding-options onboarding-options-vertical">
                  {VERTICAL_GROUP_KEYS.map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      className={`onboarding-option${verticalGroup === opt.value ? ' selected' : ''}`}
                      onClick={() => {
                        setVerticalGroup(opt.value);
                        setSubVertical('');
                      }}
                    >
                      <strong>{t(opt.labelKey)}</strong>
                      <small>{t(opt.descKey)}</small>
                    </button>
                  ))}
                </div>
              </div>

              {needsSubVertical && (
                <div className="onboarding-field">
                  <label>{t('onboarding.step1.subVertical')}</label>
                  <div className="onboarding-options onboarding-options-vertical">
                    {SUB_VERTICAL_KEYS[verticalGroup as VerticalGroup]!.map((opt) => (
                      <button
                        key={opt.value}
                        type="button"
                        className={`onboarding-option${subVertical === opt.value ? ' selected' : ''}`}
                        onClick={() => setSubVertical(opt.value)}
                      >
                        <strong>{t(opt.labelKey)}</strong>
                        <small>{t(opt.descKey)}</small>
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {step === 2 && (
            <div className="onboarding-step">
              <h2>{t('onboarding.step2.title')}</h2>

              <div className="onboarding-field">
                <label>{t('onboarding.step2.sells')}</label>
                <div className="onboarding-options">
                  {SELLS_KEYS.map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      className={`onboarding-option${sells === opt.value ? ' selected' : ''}`}
                      onClick={() => setSells(opt.value)}
                    >
                      <strong>{t(opt.labelKey)}</strong>
                      <small>{t(opt.descKey)}</small>
                    </button>
                  ))}
                </div>
              </div>

              <div className="onboarding-field">
                <label>{t('onboarding.step2.clientLabel')}</label>
                <div className="onboarding-chips">
                  {CLIENT_LABEL_KEYS.map((lbl) => (
                    <button
                      key={lbl}
                      type="button"
                      className={`onboarding-chip${clientLabel === lbl ? ' selected' : ''}`}
                      onClick={() => {
                        setClientLabel(lbl);
                        setCustomClientLabel('');
                      }}
                    >
                      {t(`onboarding.clientLabel.${lbl}`)}
                    </button>
                  ))}
                  <button
                    type="button"
                    className={`onboarding-chip${clientLabel === '__custom' ? ' selected' : ''}`}
                    onClick={() => setClientLabel('__custom')}
                  >
                    {t('onboarding.step2.clientLabelCustom')}
                  </button>
                </div>
                {clientLabel === '__custom' && (
                  <input
                    id="onboarding-custom-client"
                    type="text"
                    placeholder={t('onboarding.step2.clientLabelCustomPlaceholder')}
                    aria-label={t('onboarding.step2.clientLabelCustomAria')}
                    value={customClientLabel}
                    onChange={(e) => setCustomClientLabel(e.target.value)}
                    autoFocus
                  />
                )}
              </div>

              <div className="onboarding-field">
                <label>{t('onboarding.step2.scheduling', { clientLabel: resolvedClientLabel })}</label>
                <div className="onboarding-chips">
                  <button
                    type="button"
                    className={`onboarding-chip${usesScheduling === true ? ' selected' : ''}`}
                    onClick={() => setUsesScheduling(true)}
                  >
                    {t('onboarding.step2.schedulingYes')}
                  </button>
                  <button
                    type="button"
                    className={`onboarding-chip${usesScheduling === false ? ' selected' : ''}`}
                    onClick={() => setUsesScheduling(false)}
                  >
                    {t('onboarding.step2.schedulingNo')}
                  </button>
                </div>
              </div>

              <div className="onboarding-field">
                <label>{t('onboarding.step2.billing')}</label>
                <div className="onboarding-chips">
                  <button
                    type="button"
                    className={`onboarding-chip${usesBilling === true ? ' selected' : ''}`}
                    onClick={() => setUsesBilling(true)}
                  >
                    {t('onboarding.step2.billingYes')}
                  </button>
                  <button
                    type="button"
                    className={`onboarding-chip${usesBilling === false ? ' selected' : ''}`}
                    onClick={() => setUsesBilling(false)}
                  >
                    {t('onboarding.step2.billingNo')}
                  </button>
                </div>
              </div>
            </div>
          )}

          {step === 3 && (
            <div className="onboarding-step">
              <h2>{t('onboarding.step3.title')}</h2>

              <div className="onboarding-field">
                <label htmlFor="onboarding-currency">{t('onboarding.step3.currency')}</label>
                <select id="onboarding-currency" value={currency} onChange={(e) => setCurrency(e.target.value)}>
                  {CURRENCY_KEYS.map((code) => (
                    <option key={code} value={code}>
                      {t(`onboarding.currency.${code}`)}
                    </option>
                  ))}
                </select>
              </div>

              <div className="onboarding-field">
                <label>{t('onboarding.step3.paymentMethod')}</label>
                <div className="onboarding-options onboarding-options-row">
                  {PAYMENT_KEYS.map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      className={`onboarding-option compact${paymentMethod === opt.value ? ' selected' : ''}`}
                      onClick={() => setPaymentMethod(opt.value)}
                    >
                      <strong>{t(opt.labelKey)}</strong>
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}

          {step === 4 && (
            <div className="onboarding-step">
              <h2>{t('onboarding.step4.title')}</h2>
              <p className="onboarding-summary-intro">{t('onboarding.step4.intro')}</p>

              <div className="onboarding-summary">
                <div className="onboarding-summary-row">
                  <span>{t('onboarding.step4.business')}</span>
                  <strong>{businessName}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>{t('onboarding.step4.team')}</span>
                  <strong>{TEAM_KEYS.find((o) => o.value === teamSize)?.labelKey ? t(TEAM_KEYS.find((o) => o.value === teamSize)!.labelKey) : teamSize}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>{t('onboarding.step4.verticalType')}</span>
                  <strong>
                    {resolvedSubVerticalLabelKey
                      ? t(resolvedSubVerticalLabelKey)
                      : resolvedVertical
                        ? t(ALL_VERTICAL_LABEL_KEYS[resolvedVertical])
                        : '-'}
                  </strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>{t('onboarding.step4.sells')}</span>
                  <strong>{SELLS_KEYS.find((o) => o.value === sells)?.labelKey ? t(SELLS_KEYS.find((o) => o.value === sells)!.labelKey) : sells}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>{t('onboarding.step4.clientLabel')}</span>
                  <strong>{resolvedClientLabel}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>{t('onboarding.step4.scheduling')}</span>
                  <strong>{usesScheduling ? t('onboarding.step4.yes') : t('onboarding.step4.no')}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>{t('onboarding.step4.billing')}</span>
                  <strong>{usesBilling ? t('onboarding.step4.yes') : t('onboarding.step4.no')}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>{t('onboarding.step4.currency')}</span>
                  <strong>{currency}</strong>
                </div>
                <div className="onboarding-summary-row">
                  <span>{t('onboarding.step4.paymentMethod')}</span>
                  <strong>{PAYMENT_KEYS.find((o) => o.value === paymentMethod)?.labelKey ? t(PAYMENT_KEYS.find((o) => o.value === paymentMethod)!.labelKey) : paymentMethod}</strong>
                </div>
              </div>
              {finishError && <p className="alert alert-error onboarding-finish-error">{finishError}</p>}
            </div>
          )}
        </div>

        <div className="onboarding-footer">
          {step > 1 ? (
            <button type="button" className="onboarding-btn-back" onClick={back}>
              {t('onboarding.nav.back')}
            </button>
          ) : (
            <span />
          )}
          {step < 4 ? (
            <button type="button" className="onboarding-btn-next" disabled={!canNext[step]} onClick={next}>
              {t('onboarding.nav.next')}
            </button>
          ) : (
            <button
              type="button"
              className="onboarding-btn-next onboarding-btn-finish"
              disabled={!canFinishStep4}
              onClick={() => void finish()}
            >
              {finishing ? t('common.status.saving') : t('onboarding.nav.start')}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
