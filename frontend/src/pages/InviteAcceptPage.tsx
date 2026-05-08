import { useAuth, useClerk, useSession, useSignIn, useSignUp } from '@clerk/react';
import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { acceptTenantInvite, previewTenantInvite, type TenantInvitationPreview } from '../lib/api';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { extractInviteTokenFromSearchParams } from '../lib/inviteTokens';
import { queryKeys } from '../lib/queryKeys';

type InviteStage =
  | 'loading'
  | 'clerk-processing'
  | 'signin-required'
  | 'needs-signin-password'
  | 'needs-signup-fields'
  | 'needs-captcha'
  | 'accepting'
  | 'done'
  | 'error';

type ClerkResult = { error?: unknown | null };

type SignupFormState = {
  firstName: string;
  lastName: string;
  password: string;
  legalAccepted: boolean;
};

export function InviteAcceptPage() {
  const [params] = useSearchParams();
  const token = extractInviteTokenFromSearchParams(params);
  const clerkTicket = params.get('__clerk_ticket')?.trim() ?? '';
  const clerkStatus = params.get('__clerk_status')?.trim() ?? '';
  const clerk = useClerk();
  const { isLoaded, isSignedIn } = useAuth();
  const { signIn } = useSignIn();
  const { signUp } = useSignUp();
  const { session } = useSession();
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [stage, setStage] = useState<InviteStage>('loading');
  const [error, setError] = useState('');
  const [preview, setPreview] = useState<TenantInvitationPreview | null>(null);
  const [signinPassword, setSigninPassword] = useState('');
  const [signupForm, setSignupForm] = useState<SignupFormState>({
    firstName: '',
    lastName: '',
    password: '',
    legalAccepted: true,
  });
  const clerkTicketAttempted = useRef('');
  const acceptAttempted = useRef(false);
  const currentRedirect = `${location.pathname}${location.search}`;

  useEffect(() => {
    let alive = true;
    async function run(): Promise<void> {
      if (!token) {
        setStage('error');
        setError('La invitación no tiene token.');
        return;
      }
      try {
        setStage('loading');
        setError('');
        const response = await previewTenantInvite(token);
        if (!alive) return;
        setPreview(response.invite);
        if (!isLoaded) {
          return;
        }
        if (isSignedIn) {
          setStage('accepting');
          return;
        }
        setStage(clerkTicket ? 'clerk-processing' : 'signin-required');
      } catch (err) {
        if (!alive) return;
        setStage('error');
        setError(formatFetchErrorForUser(err, 'No se pudo validar la invitación.'));
      }
    }
    void run();
    return () => {
      alive = false;
    };
  }, [clerkTicket, isLoaded, isSignedIn, token]);

  useEffect(() => {
    if (!isLoaded || isSignedIn || !preview) {
      return;
    }
    if (!clerkTicket) {
      setStage('signin-required');
      return;
    }
    if (clerkTicketAttempted.current === clerkTicket) {
      return;
    }
    clerkTicketAttempted.current = clerkTicket;
    void processClerkTicket();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clerkTicket, isLoaded, isSignedIn, preview]);

  const activateClerkOrganization = useCallback(async (clerkOrgID: string | undefined): Promise<void> => {
    const organization = clerkOrgID?.trim();
    if (!organization) {
      return;
    }
    try {
      await withClerkTimeout<unknown>(clerk.setActive({ organization }), 'set_active_organization');
      if (session) {
        await withClerkTimeout<unknown>(session.reload(), 'reload_session');
      }
    } catch (err) {
      if (isCaptchaError(err)) {
        throw err;
      }
      console.warn('Clerk organization activation did not finish before navigation', err);
    }
  }, [clerk, session]);

  useEffect(() => {
    async function run(): Promise<void> {
      if (!isLoaded || !isSignedIn || !token || acceptAttempted.current) {
        return;
      }
      acceptAttempted.current = true;
      try {
        setStage('accepting');
        setError('');
        const accepted = await acceptTenantInvite(token);
        await activateClerkOrganization(accepted.clerk_org_id);
        void queryClient.invalidateQueries({ queryKey: queryKeys.session.current });
        void queryClient.invalidateQueries({ queryKey: queryKeys.me.current });
        void queryClient.invalidateQueries({ queryKey: queryKeys.tenant.settings });
        setStage('done');
        const tenantSlug = accepted.tenant_slug?.trim() || preview?.tenant_slug?.trim();
        navigate(tenantSlug ? `/${tenantSlug}/dashboard` : '/', { replace: true });
      } catch (err) {
        acceptAttempted.current = false;
        setStage('error');
        setError(formatFetchErrorForUser(err, 'No se pudo aceptar la invitación.'));
      }
    }
    void run();
  }, [activateClerkOrganization, isLoaded, isSignedIn, navigate, preview?.tenant_slug, queryClient, token]);

  async function processClerkTicket(): Promise<void> {
    try {
      setStage('clerk-processing');
      setError('');
      if (clerkStatus === 'sign_in') {
        await runSignInTicket();
        return;
      }
      if (clerkStatus === 'sign_up') {
        await runSignUpTicket();
        return;
      }
      if (clerkStatus === 'complete') {
        setStage('accepting');
        return;
      }
      setStage('error');
      setError('El estado de la invitación de Clerk no es válido.');
    } catch (err) {
      if (isCaptchaError(err)) {
        setStage('needs-captcha');
        setError('Clerk pidió una verificación anti-bot. Resolvela en esta pantalla y reintentamos.');
        return;
      }
      setStage('error');
      setError(formatFetchErrorForUser(err, 'Clerk no pudo completar la invitación.'));
    }
  }

  async function runSignInTicket(): Promise<void> {
    assertNoClerkError(await withClerkTimeout(signIn.ticket({ ticket: clerkTicket }), 'sign_in_ticket'));
    await continueSignIn();
  }

  async function continueSignIn(): Promise<void> {
    if (signIn.status === 'complete') {
      assertNoClerkError(await withClerkTimeout(signIn.finalize(), 'sign_in_finalize'));
      setStage('accepting');
      return;
    }
    if (signIn.status === 'needs_first_factor') {
      setStage('needs-signin-password');
      return;
    }
    if (signIn.status === 'needs_identifier') {
      setStage('signin-required');
      setError('Clerk pidió iniciar sesión con el email invitado antes de aceptar.');
      return;
    }
    if (signIn.status === 'needs_second_factor') {
      setStage('error');
      setError('Clerk pidió segundo factor. MFA debe quedar deshabilitado en DEV para este flujo.');
      return;
    }
    if (signIn.isTransferable) {
      assertNoClerkError(await withClerkTimeout(signUp.create({ transfer: true }), 'sign_up_transfer'));
      await continueSignUp();
      return;
    }
    setStage('error');
    setError(`Clerk dejó el inicio de sesión en estado ${signIn.status}.`);
  }

  async function runSignUpTicket(): Promise<void> {
    assertNoClerkError(
      await withClerkTimeout(signUp.create({ strategy: 'ticket', ticket: clerkTicket }), 'sign_up_ticket'),
    );
    await continueSignUp();
  }

  async function continueSignUp(): Promise<void> {
    if (signUp.status === 'complete') {
      assertNoClerkError(await withClerkTimeout(signUp.finalize(), 'sign_up_finalize'));
      setStage('accepting');
      return;
    }
    if (signUp.isTransferable) {
      assertNoClerkError(await withClerkTimeout(signIn.create({ transfer: true }), 'sign_in_transfer'));
      await continueSignIn();
      return;
    }
    if (signUp.status === 'missing_requirements') {
      setSignupForm((current) => ({
        ...current,
        firstName: current.firstName || signUp.firstName || '',
        lastName: current.lastName || signUp.lastName || '',
        legalAccepted: current.legalAccepted || Boolean(signUp.legalAcceptedAt),
      }));
      setStage('needs-signup-fields');
      return;
    }
    setStage('error');
    setError(`Clerk dejó el registro en estado ${signUp.status}.`);
  }

  async function handleSigninPasswordSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    if (!signinPassword.trim()) {
      setError('Ingresá la contraseña de la cuenta.');
      return;
    }
    try {
      setStage('clerk-processing');
      setError('');
      assertNoClerkError(await withClerkTimeout(signIn.password({ password: signinPassword }), 'sign_in_password'));
      await continueSignIn();
    } catch (err) {
      if (isCaptchaError(err)) {
        setStage('needs-captcha');
        setError('Clerk pidió verificación anti-bot. Resolvela y reintentamos.');
        return;
      }
      setStage('needs-signin-password');
      setError(formatFetchErrorForUser(err, 'No se pudo iniciar sesión con esa contraseña.'));
    }
  }

  async function handleSignupFieldsSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const needsPassword = missingSignUpField(signUp.missingFields, 'password');
    if (needsPassword && !signupForm.password.trim()) {
      setError('Clerk pidió crear una contraseña para esta cuenta.');
      return;
    }
    if (missingSignUpField(signUp.missingFields, 'first_name') && !signupForm.firstName.trim()) {
      setError('Clerk pidió nombre para crear la cuenta.');
      return;
    }
    if (missingSignUpField(signUp.missingFields, 'last_name') && !signupForm.lastName.trim()) {
      setError('Clerk pidió apellido para crear la cuenta.');
      return;
    }
    try {
      setStage('clerk-processing');
      setError('');
      const common = {
        emailAddress: missingSignUpField(signUp.missingFields, 'email_address') ? (preview?.email ?? undefined) : undefined,
        firstName: signupForm.firstName.trim() || undefined,
        lastName: signupForm.lastName.trim() || undefined,
        legalAccepted: signupForm.legalAccepted,
      };
      if (needsPassword) {
        assertNoClerkError(
          await withClerkTimeout(
            signUp.password({
              ...common,
              emailAddress: preview?.email ?? undefined,
              password: signupForm.password,
            }),
            'sign_up_password',
          ),
        );
      } else {
        assertNoClerkError(await withClerkTimeout(signUp.update(common), 'sign_up_update'));
      }
      await continueSignUp();
    } catch (err) {
      if (isCaptchaError(err)) {
        setStage('needs-captcha');
        setError('Clerk pidió verificación anti-bot. Resolvela y reintentamos.');
        return;
      }
      setStage('needs-signup-fields');
      setError(formatFetchErrorForUser(err, 'No se pudo completar el registro de Clerk.'));
    }
  }

  function retryClerkTicket(): void {
    clerkTicketAttempted.current = '';
    setStage('clerk-processing');
    setError('');
    void processClerkTicket();
  }

  if (!isLoaded) {
    return (
      <InviteShell>
        <InviteTitle preview={preview} />
        <p>Preparando la sesión…</p>
      </InviteShell>
    );
  }

  return (
    <InviteShell>
      <InviteTitle preview={preview} />
      {stage === 'loading' ? <p>Validando invitación…</p> : null}
      {stage === 'clerk-processing' ? (
        <>
          <p>Aceptando invitación con Clerk…</p>
          <div id="clerk-captcha" />
        </>
      ) : null}
      {stage === 'signin-required' ? (
        <>
          <p>Para aceptar esta invitación tenés que iniciar sesión con {preview?.email ?? 'el email invitado'}.</p>
          <button
            type="button"
            className="btn-primary"
            onClick={() => navigate(`/login?redirect_url=${encodeURIComponent(currentRedirect)}`)}
          >
            Iniciar sesión
          </button>
        </>
      ) : null}
      {stage === 'needs-signin-password' ? (
        <form className="auth-form" onSubmit={(event) => void handleSigninPasswordSubmit(event)}>
          <p>Clerk reconoció la invitación y pidió validar la cuenta existente.</p>
          {error ? <p className="form-error">{error}</p> : null}
          <div className="form-group">
            <label htmlFor="invite-password">Contraseña</label>
            <input
              id="invite-password"
              type="password"
              value={signinPassword}
              onChange={(event) => setSigninPassword(event.target.value)}
              autoComplete="current-password"
              autoFocus
            />
          </div>
          <button type="submit" className="btn-primary">
            Continuar
          </button>
        </form>
      ) : null}
      {stage === 'needs-signup-fields' ? (
        <form className="auth-form" onSubmit={(event) => void handleSignupFieldsSubmit(event)}>
          <p>Clerk necesita completar estos datos para crear la cuenta.</p>
          {error ? <p className="form-error">{error}</p> : null}
          {missingSignUpField(signUp.missingFields, 'first_name') ? (
            <div className="form-group">
              <label htmlFor="invite-first-name">Nombre</label>
              <input
                id="invite-first-name"
                value={signupForm.firstName}
                onChange={(event) => setSignupForm((current) => ({ ...current, firstName: event.target.value }))}
                autoFocus
              />
            </div>
          ) : null}
          {missingSignUpField(signUp.missingFields, 'last_name') ? (
            <div className="form-group">
              <label htmlFor="invite-last-name">Apellido</label>
              <input
                id="invite-last-name"
                value={signupForm.lastName}
                onChange={(event) => setSignupForm((current) => ({ ...current, lastName: event.target.value }))}
              />
            </div>
          ) : null}
          {missingSignUpField(signUp.missingFields, 'password') ? (
            <div className="form-group">
              <label htmlFor="invite-signup-password">Crear contraseña</label>
              <input
                id="invite-signup-password"
                type="password"
                value={signupForm.password}
                onChange={(event) => setSignupForm((current) => ({ ...current, password: event.target.value }))}
                autoComplete="new-password"
              />
            </div>
          ) : null}
          {missingSignUpField(signUp.missingFields, 'legal_accepted') ? (
            <label className="checkbox-inline">
              <input
                type="checkbox"
                checked={signupForm.legalAccepted}
                onChange={(event) => setSignupForm((current) => ({ ...current, legalAccepted: event.target.checked }))}
              />
              Acepto los términos requeridos por Clerk.
            </label>
          ) : null}
          <div id="clerk-captcha" />
          <button type="submit" className="btn-primary">
            Continuar
          </button>
        </form>
      ) : null}
      {stage === 'needs-captcha' ? (
        <>
          {error ? <p className="form-error">{error}</p> : null}
          <div id="clerk-captcha" />
          <button type="button" className="btn-primary" onClick={retryClerkTicket}>
            Reintentar
          </button>
        </>
      ) : null}
      {stage === 'accepting' ? <p>Creando acceso al tenant…</p> : null}
      {stage === 'done' ? <p>Invitación aceptada.</p> : null}
      {stage === 'error' ? <p className="form-error">{error}</p> : null}
    </InviteShell>
  );
}

function InviteTitle({ preview }: { preview: TenantInvitationPreview | null }) {
  return (
    <>
      <h1>Invitación</h1>
      {preview ? (
        <p>
          {preview.email} fue invitado a {preview.tenant_name || preview.tenant_slug}.
        </p>
      ) : null}
    </>
  );
}

function InviteShell({ children }: { children: ReactNode }) {
  return (
    <main className="auth-page">
      <section className="card auth-card">{children}</section>
    </main>
  );
}

function assertNoClerkError(result: ClerkResult): void {
  if (result?.error) {
    throw result.error;
  }
}

function withClerkTimeout<T>(promise: Promise<T>, label: string): Promise<T> {
  const timeoutMs = 15000;
  return new Promise((resolve, reject) => {
    const timeoutId = window.setTimeout(() => {
      reject(new Error(`${label}: clerk_captcha_or_timeout`));
    }, timeoutMs);
    promise.then(
      (value) => {
        window.clearTimeout(timeoutId);
        resolve(value);
      },
      (err: unknown) => {
        window.clearTimeout(timeoutId);
        reject(err);
      },
    );
  });
}

function missingSignUpField(fields: readonly string[] | undefined, field: string): boolean {
  const normalized = field.trim();
  const camel = normalized.replace(/_([a-z])/g, (_, char: string) => char.toUpperCase());
  return Boolean(fields?.some((item) => item === normalized || item === camel));
}

function isCaptchaError(err: unknown): boolean {
  const text = formatFetchErrorForUser(err, String(err)).toLowerCase();
  return (
    text.includes('captcha') ||
    text.includes('bot') ||
    text.includes('challenge') ||
    text.includes('cloudflare') ||
    text.includes('verification') ||
    text.includes('turnstile') ||
    text.includes('clerk-captcha') ||
    text.includes('clerk_captcha_or_timeout') ||
    text.includes('timeout')
  );
}
