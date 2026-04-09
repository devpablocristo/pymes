import type { ReactNode } from "react";

export type ConversationInboxTone = "default" | "attention" | "success";

export type ConversationInboxItem = {
  id: string;
  contactName: ReactNode;
  contactDetail?: ReactNode;
  preview: ReactNode;
  assignee?: ReactNode;
  status?: ReactNode;
  timestamp?: ReactNode;
  badge?: ReactNode;
  actions?: ReactNode;
  unread?: boolean;
  tone?: ConversationInboxTone;
};

export type ConversationInboxProps = {
  items: ConversationInboxItem[];
  loading?: boolean;
  loadingMessage?: ReactNode;
  emptyMessage: ReactNode;
  error?: ReactNode;
  summary?: ReactNode;
  className?: string;
};

function toneClassName(tone: ConversationInboxTone | undefined): string {
  switch (tone) {
    case "attention":
      return "m-conversation-inbox__card--attention";
    case "success":
      return "m-conversation-inbox__card--success";
    default:
      return "m-conversation-inbox__card--default";
  }
}

export function ConversationInbox({
  items,
  loading = false,
  loadingMessage = "Cargando…",
  emptyMessage,
  error,
  summary,
  className = "",
}: ConversationInboxProps) {
  return (
    <section className={`m-conversation-inbox ${className}`.trim()}>
      {error ? <div className="m-conversation-inbox__error">{error}</div> : null}
      {summary ? <div className="m-conversation-inbox__summary">{summary}</div> : null}
      {loading ? (
        <div className="m-conversation-inbox__empty">{loadingMessage}</div>
      ) : items.length === 0 ? (
        <div className="m-conversation-inbox__empty">{emptyMessage}</div>
      ) : (
        <ul className="m-conversation-inbox__list">
          {items.map((item) => (
            <li
              key={item.id}
              className={`m-conversation-inbox__card ${toneClassName(item.tone)}${
                item.unread ? " m-conversation-inbox__card--unread" : ""
              }`}
            >
              <div className="m-conversation-inbox__header">
                <div className="m-conversation-inbox__titleWrap">
                  <div className="m-conversation-inbox__title">{item.contactName}</div>
                  {item.contactDetail ? (
                    <div className="m-conversation-inbox__detail">{item.contactDetail}</div>
                  ) : null}
                </div>
                <div className="m-conversation-inbox__headerMeta">
                  {item.status ? <div className="m-conversation-inbox__status">{item.status}</div> : null}
                  {item.badge ? <div className="m-conversation-inbox__badge">{item.badge}</div> : null}
                </div>
              </div>
              <div className="m-conversation-inbox__preview">{item.preview}</div>
              {item.assignee ? <div className="m-conversation-inbox__assignee">{item.assignee}</div> : null}
              {item.timestamp ? <div className="m-conversation-inbox__timestamp">{item.timestamp}</div> : null}
              {item.actions ? <div className="m-conversation-inbox__actions">{item.actions}</div> : null}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
