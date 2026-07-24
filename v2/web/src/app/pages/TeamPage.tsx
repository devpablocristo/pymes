import { useEffect, useState, type FormEvent } from "react";
import type { components } from "../../api/schema.generated";
import { createIdempotencyKey } from "../../api/idempotency";
import { useProductApi } from "../../api/ProductApiContext";
import { useI18n } from "../providers/I18nProvider";

type MemberList = components["schemas"]["MemberList"];
type InvitationList = components["schemas"]["InvitationList"];
type CurrentSession = components["schemas"]["CurrentSession"];

type TeamData = {
  session: CurrentSession;
  members: MemberList["items"];
  invitations?: InvitationList["items"];
};

export function TeamPage() {
  const api = useProductApi();
  const { t } = useI18n();
  const [data, setData] = useState<TeamData>();
  const [error, setError] = useState<string>();
  const [revision, setRevision] = useState(0);
  const [busy, setBusy] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<"admin" | "member">("member");

  useEffect(() => {
    const controller = new AbortController();
    setError(undefined);
    Promise.all([
      api.request<CurrentSession>("/api/v1/session", {
        signal: controller.signal,
        skipJSONContentType: true,
      }),
      api.request<MemberList>("/api/v1/team/members?limit=100", {
        signal: controller.signal,
        skipJSONContentType: true,
      }),
    ])
      .then(async ([session, members]) => {
        const invitations = session.permissions.includes("team:invitation:manage")
          ? (
              await api.request<InvitationList>("/api/v1/team/invitations?limit=100", {
                signal: controller.signal,
                skipJSONContentType: true,
              })
            ).items
          : undefined;
        setData({ session, members: members.items, invitations });
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        setError(cause instanceof Error ? cause.message : t("team.loadError"));
      });
    return () => controller.abort();
  }, [api, revision, t]);

  async function command(
    operation: string,
    path: string,
    method: "POST" | "PATCH" | "DELETE",
    body?: unknown,
  ): Promise<boolean> {
    setBusy(operation);
    setNotice(undefined);
    setError(undefined);
    try {
      await api.request(path, {
        method,
        headers: {
          "Idempotency-Key": createIdempotencyKey(operation),
        },
        ...(body === undefined ? {} : { body: JSON.stringify(body) }),
      });
      setNotice(t("team.commandQueued"));
      setRevision((value) => value + 1);
      return true;
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : t("team.commandError"));
      return false;
    } finally {
      setBusy(undefined);
    }
  }

  async function invite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const email = inviteEmail.trim().toLowerCase();
    if (!email) return;
    const succeeded = await command("invite", "/api/v1/team/invitations", "POST", {
      email,
      role: inviteRole,
    });
    if (succeeded) {
      setInviteEmail("");
      setInviteRole("member");
    }
  }

  return (
    <div className="settings-page">
      <header className="page-topbar">
        <div>
          <h1>{t("team.title")}</h1>
          <small>{t("settings.eyebrow")}</small>
        </div>
      </header>
      <div className="settings-canvas">
        <div className="settings-heading">
          <div>
            <h2>{t("team.heading")}</h2>
            <p>{t("team.description")}</p>
          </div>
          <span className="settings-count">{data?.members.length ?? "—"}</span>
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

        {notice ? (
          <div className="inline-state inline-state--success" role="status">
            <strong>{notice}</strong>
          </div>
        ) : null}

        {!data && !error ? (
          <div className="settings-list" aria-label={t("state.loading")}>
            {[0, 1, 2].map((item) => (
              <div className="settings-row settings-row--skeleton" key={item} aria-hidden="true" />
            ))}
          </div>
        ) : null}

        {data ? (
          <>
            {data.session.permissions.includes("team:invitation:create") ? (
              <form className="settings-inline-form" onSubmit={(event) => void invite(event)}>
                <div>
                  <strong>{t("team.inviteHeading")}</strong>
                  <span>{t("team.inviteDescription")}</span>
                </div>
                <label>
                  <span className="visually-hidden">{t("team.inviteEmail")}</span>
                  <input
                    required
                    type="email"
                    autoComplete="email"
                    placeholder={t("team.inviteEmail")}
                    value={inviteEmail}
                    disabled={Boolean(busy)}
                    onChange={(event) => setInviteEmail(event.target.value)}
                  />
                </label>
                {data.session.role === "owner" ? (
                  <label>
                    <span className="visually-hidden">{t("team.inviteRole")}</span>
                    <select
                      value={inviteRole}
                      disabled={Boolean(busy)}
                      onChange={(event) =>
                        setInviteRole(event.target.value as "admin" | "member")
                      }
                    >
                      <option value="member">{t("roles.member")}</option>
                      <option value="admin">{t("roles.admin")}</option>
                    </select>
                  </label>
                ) : null}
                <button type="submit" disabled={Boolean(busy) || !inviteEmail.trim()}>
                  {busy === "invite" ? t("team.inviting") : t("team.invite")}
                </button>
              </form>
            ) : null}

            <section className="settings-section" aria-labelledby="team-members-title">
              <div className="settings-section__title">
                <h3 id="team-members-title">{t("team.members")}</h3>
                <span>{data.members.length}</span>
              </div>
              <div className="settings-list">
                {data.members.map((member) => (
                  <article className="settings-row" key={member.id}>
                    <span className="settings-row__avatar" aria-hidden="true">
                      {member.user.avatar_url ? (
                        <img src={member.user.avatar_url} alt="" />
                      ) : (
                        member.user.display_name.slice(0, 1).toUpperCase()
                      )}
                    </span>
                    <div className="settings-row__body">
                      <div className="settings-row__title">
                        <strong>{member.user.display_name}</strong>
                        <span className={`status-pill status-pill--${member.sync_status}`}>
                          {member.sync_status}
                        </span>
                      </div>
                      <span>{member.user.email}</span>
                      <small>{t(`roles.${member.role}`)}</small>
                    </div>
                    {canUpdateMember(data.session, member) ||
                    canRemoveMember(data.session, member) ? (
                      <div className="settings-row__actions">
                        {canUpdateMember(data.session, member) ? (
                          <select
                            aria-label={`${t("team.changeRole")}: ${member.user.display_name}`}
                            value={member.role}
                            disabled={Boolean(busy)}
                            onChange={(event) =>
                              void command(
                                `member-role-${member.id}`,
                                `/api/v1/team/members/${member.id}`,
                                "PATCH",
                                { role: event.target.value },
                              )
                            }
                          >
                            <option value="member">{t("roles.member")}</option>
                            <option value="admin">{t("roles.admin")}</option>
                          </select>
                        ) : null}
                        {canRemoveMember(data.session, member) ? (
                          <button
                            type="button"
                            className="settings-row__danger"
                            disabled={Boolean(busy)}
                            onClick={() =>
                              void command(
                                `member-remove-${member.id}`,
                                `/api/v1/team/members/${member.id}`,
                                "DELETE",
                              )
                            }
                          >
                            {t("team.remove")}
                          </button>
                        ) : null}
                      </div>
                    ) : null}
                  </article>
                ))}
              </div>
            </section>

            {data.invitations ? (
              <section className="settings-section" aria-labelledby="team-invitations-title">
                <div className="settings-section__title">
                  <h3 id="team-invitations-title">{t("team.invitations")}</h3>
                  <span>{data.invitations.length}</span>
                </div>
                {data.invitations.length === 0 ? (
                  <div className="inline-state">
                    <strong>{t("team.noInvitations")}</strong>
                    <span>{t("team.noInvitationsBody")}</span>
                  </div>
                ) : (
                  <div className="settings-list">
                    {data.invitations.map((invitation) => (
                      <article className="settings-row" key={invitation.id}>
                        <span className="settings-row__icon" aria-hidden="true">✉</span>
                        <div className="settings-row__body">
                          <div className="settings-row__title">
                            <strong>{invitation.email}</strong>
                            <span className={`status-pill status-pill--${invitation.sync_status}`}>
                              {invitation.sync_status}
                            </span>
                          </div>
                          <span>{t(`roles.${invitation.role}`)}</span>
                          <small>{invitation.status}</small>
                        </div>
                        <div className="settings-row__actions">
                          <button
                            type="button"
                            disabled={Boolean(busy)}
                            onClick={() =>
                              void command(
                                `invite-resend-${invitation.id}`,
                                `/api/v1/team/invitations/${invitation.id}/resend`,
                                "POST",
                              )
                            }
                          >
                            {t("team.resend")}
                          </button>
                          <button
                            type="button"
                            className="settings-row__danger"
                            disabled={Boolean(busy)}
                            onClick={() =>
                              void command(
                                `invite-revoke-${invitation.id}`,
                                `/api/v1/team/invitations/${invitation.id}/revoke`,
                                "POST",
                              )
                            }
                          >
                            {t("team.revoke")}
                          </button>
                        </div>
                      </article>
                    ))}
                  </div>
                )}
              </section>
            ) : null}
          </>
        ) : null}
      </div>
    </div>
  );
}

function isManageableTarget(
  session: CurrentSession,
  member: MemberList["items"][number],
) {
  if (member.id === session.membership.id || member.role === "owner") return false;
  if (session.role === "owner") return true;
  return session.role === "admin" && member.role === "member";
}

function canUpdateMember(
  session: CurrentSession,
  member: MemberList["items"][number],
) {
  return (
    session.permissions.includes("team:member:update") &&
    isManageableTarget(session, member)
  );
}

function canRemoveMember(
  session: CurrentSession,
  member: MemberList["items"][number],
) {
  return (
    session.permissions.includes("team:member:remove") &&
    isManageableTarget(session, member)
  );
}
