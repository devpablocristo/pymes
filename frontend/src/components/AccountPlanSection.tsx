import { useEffect, useMemo, useState } from 'react';
import { createPortal, getBillingStatus } from '../lib/api';
import { formatBillingPageError } from '../lib/formatFetchError';
import { useI18n } from '../lib/i18n';
import type { BillingStatus, SessionResponse } from '../lib/types';

const settingsReturnPath = '/settings';

/**
 * Resumen de facturación del tenant en el perfil. Cambio de plan vía portal de Stripe, no en esta pantalla.
 */
export function AccountPlanSection({ session }: { session: SessionResponse }) {
  const { t, language } = useI18n();
  const [billing, setBilling] = useState<BillingStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [stripeUnavailable, setStripeUnavailable] = useState(false);
  const [banner, setBanner] = useState<{ variant: 'warning' | 'error'; text: string } | null>(null);
  const [portalError, setPortalError] = useState('');

  const isAdmin = session.auth.product_role === 'admin';

  const billingStatusLabels = useMemo(
    () => ({
      active: t('billing.status.active'),
      trialing: t('billing.status.trialing'),
      past_due: t('billing.status.past_due'),
      canceled: t('billing.status.canceled'),
      unpaid: t('billing.status.unpaid'),
    }),
    [t],
  );

  async function load(): Promise<void> {
    setLoading(true);
    try {
      const resp = await getBillingStatus();
      setBilling(resp);
      setBanner(null);
      setStripeUnavailable(false);
    } catch (err) {
      setBilling(null);
      const { kind, message } = formatBillingPageError(
        err,
        t('billing.error.unreachable'),
        t('billing.notice.stripeNotConfigured'),
      );
      setStripeUnavailable(kind === 'stripe_unconfigured');
      setBanner({ variant: kind === 'stripe_unconfigured' ? 'warning' : 'error', text: message });
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [session.auth.org_id]);

  const returnUrl = `${window.location.origin}${settingsReturnPath}`;

  async function openPortal(): Promise<void> {
    if (!isAdmin || stripeUnavailable) {
      return;
    }
    setPortalError('');
    try {
      const resp = await createPortal({ return_url: returnUrl });
      // Portal alojado en Stripe: misma pestaña + return_url al perfil (no se puede embeber en la SPA).
      window.location.assign(resp.portal_url);
    } catch (e) {
      const { kind, message } = formatBillingPageError(
        e,
        t('billing.error.unreachable'),
        t('billing.notice.stripeNotConfigured'),
      );
      setStripeUnavailable(kind === 'stripe_unconfigured');
      setPortalError(message);
    }
  }

  const statusLabel =
    billing?.status != null
      ? billingStatusLabels[billing.status as keyof typeof billingStatusLabels] ?? billing.status
      : '—';

  const planLabel =
    billing != null && billing.plan_code ? t(`billing.plan.${billing.plan_code}`) : '—';

  const periodEndLabel =
    billing?.current_period_end != null
      ? new Date(billing.current_period_end).toLocaleDateString(language === 'en' ? 'en-US' : 'es-AR')
      : '—';

  const statusBadgeClass =
    billing != null &&
    (billing.status === 'active' || billing.status === 'trialing')
      ? 'badge-success'
      : 'badge-warning';

  return (
    <div className="profile-account-plan-section">
      {banner &&
        (banner.variant === 'warning' ? (
          <div className="alert alert-warning profile-form-alert">{banner.text}</div>
        ) : (
          <div className="alert alert-error profile-form-alert">{banner.text}</div>
        ))}

      {loading && <p className="text-muted">{t('common.status.loading')}</p>}

      {!loading && (
        <>
          <table className="profile-session-table profile-billing-table">
            <tbody>
              <tr>
                <th scope="row">{t('profile.billing.plan')}</th>
                <td>
                  <span className="profile-session-value">{planLabel}</span>
                </td>
              </tr>
              <tr>
                <th scope="row">{t('profile.billing.status')}</th>
                <td>
                  <span className="profile-session-value">
                    {billing != null ? (
                      <span className={`badge ${statusBadgeClass}`}>{statusLabel}</span>
                    ) : (
                      '—'
                    )}
                  </span>
                </td>
              </tr>
              <tr>
                <th scope="row">{t('profile.billing.periodEnd')}</th>
                <td>
                  <span className="profile-session-value mono">{periodEndLabel}</span>
                </td>
              </tr>
            </tbody>
          </table>

          {!billing && !banner && <p className="text-muted profile-billing-load-error">{t('profile.billing.loadError')}</p>}

          {isAdmin && (
            <p className="profile-billing-actions profile-billing-actions--tight">
              <button
                type="button"
                className="btn-secondary"
                disabled={stripeUnavailable}
                onClick={() => void openPortal()}
              >
                {t('profile.billing.managePortal')}
              </button>
            </p>
          )}
          {portalError && <p className="alert alert-error profile-form-alert">{portalError}</p>}
        </>
      )}
    </div>
  );
}
