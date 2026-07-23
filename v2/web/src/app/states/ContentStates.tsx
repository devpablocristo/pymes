import type { ReactNode } from "react";

type MessageStateProps = {
  title: string;
  body: string;
  action?: ReactNode;
};

export function LoadingState({ label }: { label: string }) {
  return (
    <div className="content-state content-state--loading" role="status" aria-live="polite">
      <span className="content-state__spinner" aria-hidden="true" />
      <span>{label}</span>
    </div>
  );
}

export function SkeletonState({ label }: { label: string }) {
  return (
    <div className="skeleton-state" role="status" aria-label={label}>
      <span className="skeleton-state__line skeleton-state__line--short" />
      <span className="skeleton-state__line" />
      <span className="skeleton-state__line" />
    </div>
  );
}

export function EmptyState({ title, body, action }: MessageStateProps) {
  return (
    <section className="content-state content-state--empty">
      <span className="content-state__mark" aria-hidden="true">
        <img src="/assets/iso.svg" alt="" />
      </span>
      <h2>{title}</h2>
      <p>{body}</p>
      {action}
    </section>
  );
}

export function RecoverableErrorState({
  title,
  body,
  retryLabel,
  onRetry,
}: MessageStateProps & { retryLabel: string; onRetry: () => void }) {
  return (
    <section className="content-state content-state--error" role="alert">
      <span className="content-state__error-mark" aria-hidden="true">
        !
      </span>
      <h2>{title}</h2>
      <p>{body}</p>
      <button type="button" className="button button--primary" onClick={onRetry}>
        {retryLabel}
      </button>
    </section>
  );
}

export function FatalErrorState({
  title,
  body,
  reloadLabel,
}: MessageStateProps & { reloadLabel: string }) {
  return (
    <main className="fatal-state" role="alert">
      <img src="/assets/iso.svg" alt="" aria-hidden="true" />
      <p className="fatal-state__code">ERR / START</p>
      <h1>{title}</h1>
      <p>{body}</p>
      <button type="button" className="button button--primary" onClick={() => window.location.reload()}>
        {reloadLabel}
      </button>
    </main>
  );
}
