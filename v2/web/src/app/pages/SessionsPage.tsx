import { useEffect, useMemo, useState } from "react";
import type { components } from "../../api/schema.generated";
import { createIdempotencyKey } from "../../api/idempotency";
import { useProductApi } from "../../api/ProductApiContext";
import { useProductAuth } from "../../auth/AuthContext";
import { useI18n } from "../providers/I18nProvider";
import { SectionHeader, SectionSearch } from "../shell/SectionChrome";

type SessionList = components["schemas"]["SessionList"];
type DeviceSession = components["schemas"]["DeviceSession"];

export function SessionsPage() {
  const api = useProductApi();
  const auth = useProductAuth();
  const { language, t } = useI18n();
  const [sessions, setSessions] = useState<DeviceSession[]>();
  const [error, setError] = useState<string>();
  const [revision, setRevision] = useState(0);
  const [revoking, setRevoking] = useState<string>();
  const [search, setSearch] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setError(undefined);
    api
      .request<SessionList>("/api/v1/sessions?limit=100", {
        signal: controller.signal,
        skipJSONContentType: true,
      })
      .then((response) =>
        setSessions(response.items.filter((session) => session.status === "active")),
      )
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        setError(cause instanceof Error ? cause.message : t("sessions.loadError"));
      });
    return () => controller.abort();
  }, [api, revision, t]);

  const formatter = useMemo(
    () =>
      new Intl.DateTimeFormat(language === "es" ? "es-AR" : "en-US", {
        dateStyle: "medium",
        timeStyle: "short",
      }),
    [language],
  );
  const visibleSessions = useMemo(() => {
    const query = search.trim().toLocaleLowerCase(language === "es" ? "es" : "en");
    if (!query) return sessions;
    return sessions?.filter((session) =>
      [
        session.id,
        session.status,
        session.current ? t("sessions.current") : t("sessions.other"),
        formatter.format(new Date(session.last_active_at)),
        formatter.format(new Date(session.expires_at)),
      ].some((value) => value.toLocaleLowerCase(language === "es" ? "es" : "en").includes(query)),
    );
  }, [formatter, language, search, sessions, t]);

  async function revoke(session: DeviceSession) {
    setRevoking(session.id);
    setError(undefined);
    try {
      await api.request<void>(`/api/v1/sessions/${encodeURIComponent(session.id)}`, {
        method: "DELETE",
        headers: {
          "Idempotency-Key": createIdempotencyKey("session"),
        },
      });
      if (session.current) {
        await auth.signOut();
        return;
      }
      setSessions((current) => current?.filter((item) => item.id !== session.id));
      setRevision((value) => value + 1);
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : t("sessions.revokeError"));
    } finally {
      setRevoking(undefined);
    }
  }

  return (
    <div className="settings-page">
      <SectionHeader title={t("sessions.title")} subtitle={t("settings.eyebrow")} />
      <div className="settings-canvas">
        <div className="settings-heading">
          <div>
            <h2>{t("sessions.heading")}</h2>
            <p>{t("sessions.description")}</p>
          </div>
          <div className="settings-heading__actions">
            <SectionSearch
              label={`${t("nav.search")} ${t("sessions.title")}`}
              placeholder={`${t("nav.search")}…`}
              value={search}
              onChange={setSearch}
            />
            <span className="settings-count">{visibleSessions?.length ?? "—"}</span>
          </div>
        </div>

        {error ? (
          <div className="inline-state inline-state--error" role="alert">
            <strong>{t("state.errorTitle")}</strong>
            <span>{error}</span>
            <button type="button" onClick={() => setRevision((value) => value + 1)}>
              {t("state.retry")}
            </button>
          </div>
        ) : null}

        {!sessions && !error ? (
          <div className="settings-list" aria-label={t("state.loading")}>
            {[0, 1].map((item) => (
              <div className="settings-row settings-row--skeleton" key={item} aria-hidden="true" />
            ))}
          </div>
        ) : null}

        {visibleSessions?.length === 0 ? (
          <div className="inline-state">
            <strong>{search ? t("shell.noResults") : t("sessions.emptyTitle")}</strong>
            <span>{search ? t("nav.search") : t("sessions.emptyBody")}</span>
          </div>
        ) : null}

        {visibleSessions && visibleSessions.length > 0 ? (
          <div className="settings-list">
            {visibleSessions.map((session) => (
              <article className="settings-row" key={session.id}>
                <span className="settings-row__icon" aria-hidden="true">
                  {session.current ? "●" : "○"}
                </span>
                <div className="settings-row__body">
                  <div className="settings-row__title">
                    <strong>
                      {session.current ? t("sessions.current") : t("sessions.other")}
                    </strong>
                    <span className={`status-pill status-pill--${session.status}`}>
                      {session.status}
                    </span>
                  </div>
                  <span>
                    {t("sessions.lastActive")}: {formatter.format(new Date(session.last_active_at))}
                  </span>
                  <small>
                    {t("sessions.expires")}: {formatter.format(new Date(session.expires_at))}
                  </small>
                </div>
                <button
                  type="button"
                  className="settings-row__action"
                  disabled={Boolean(revoking)}
                  aria-busy={revoking === session.id}
                  onClick={() => void revoke(session)}
                >
                  {revoking === session.id ? t("sessions.revoking") : t("sessions.revoke")}
                </button>
              </article>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}
