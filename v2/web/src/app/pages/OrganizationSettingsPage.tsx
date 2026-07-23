import { useEffect, useState, type FormEvent } from "react";
import type { components } from "../../api/schema.generated";
import { createIdempotencyKey } from "../../api/idempotency";
import { useProductApi } from "../../api/ProductApiContext";
import { useI18n } from "../providers/I18nProvider";

type CurrentSession = components["schemas"]["CurrentSession"];
type Organization = components["schemas"]["Organization"];

export function OrganizationSettingsPage() {
  const api = useProductApi();
  const { t } = useI18n();
  const [session, setSession] = useState<CurrentSession>();
  const [name, setName] = useState("");
  const [error, setError] = useState<string>();
  const [saved, setSaved] = useState(false);
  const [saving, setSaving] = useState(false);
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setError(undefined);
    api
      .request<CurrentSession>("/api/v1/session", {
        signal: controller.signal,
        skipJSONContentType: true,
      })
      .then((response) => {
        setSession(response);
        setName(response.organization.name);
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        setError(cause instanceof Error ? cause.message : t("organization.loadError"));
      });
    return () => controller.abort();
  }, [api, revision, t]);

  const canUpdate = session?.permissions.includes("organization:update") ?? false;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalizedName = name.trim();
    if (!session || !canUpdate || !normalizedName) return;

    setSaving(true);
    setSaved(false);
    setError(undefined);
    try {
      const organization = await api.request<Organization>("/api/v1/organization", {
        method: "PATCH",
        headers: {
          "Idempotency-Key": createIdempotencyKey("organization"),
        },
        body: JSON.stringify({ name: normalizedName }),
      });
      setSession((current) =>
        current
          ? {
              ...current,
              organization,
            }
          : current,
      );
      setName(organization.name);
      setSaved(true);
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : t("organization.saveError"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="settings-page">
      <header className="page-topbar">
        <div>
          <h1>{t("organization.title")}</h1>
          <small>{t("settings.eyebrow")}</small>
        </div>
      </header>
      <div className="settings-canvas">
        <div className="settings-heading">
          <div>
            <h2>{t("organization.heading")}</h2>
            <p>{t("organization.description")}</p>
          </div>
          {session ? (
            <span className={`status-pill status-pill--${session.organization.sync_status}`}>
              {session.organization.sync_status}
            </span>
          ) : null}
        </div>

        {error ? (
          <div className="inline-state inline-state--error" role="alert">
            <strong>{t("state.errorTitle")}</strong>
            <span>{error}</span>
            {!session ? (
              <button type="button" onClick={() => setRevision((value) => value + 1)}>
                {t("state.retry")}
              </button>
            ) : null}
          </div>
        ) : null}

        {!session && !error ? (
          <div className="settings-list" aria-label={t("state.loading")}>
            <div className="settings-row settings-row--skeleton" aria-hidden="true" />
            <div className="settings-row settings-row--skeleton" aria-hidden="true" />
          </div>
        ) : null}

        {session ? (
          <form className="settings-form" onSubmit={(event) => void submit(event)}>
            <label>
              <span>{t("organization.name")}</span>
              <input
                required
                maxLength={120}
                disabled={!canUpdate || saving}
                value={name}
                onChange={(event) => {
                  setName(event.target.value);
                  setSaved(false);
                }}
              />
            </label>
            <label>
              <span>{t("organization.slug")}</span>
              <input readOnly value={session.organization.slug} />
              <small>{t("organization.slugHelp")}</small>
            </label>
            <div className="settings-form__footer">
              {!canUpdate ? <span>{t("organization.readOnly")}</span> : null}
              {saved ? (
                <span className="settings-form__success" role="status">
                  {t("organization.saved")}
                </span>
              ) : null}
              {canUpdate ? (
                <button
                  type="submit"
                  className="button button--primary"
                  disabled={saving || !name.trim() || name.trim() === session.organization.name}
                >
                  {saving ? t("organization.saving") : t("organization.save")}
                </button>
              ) : null}
            </div>
          </form>
        ) : null}
      </div>
    </div>
  );
}
