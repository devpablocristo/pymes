import type { RefObject } from 'react';
import type { PymesAssistantAction, PymesAssistantChatBlock } from '../types/aiChat';
import type { LanguageCode } from '../lib/i18n';
import type { Msg } from './UnifiedChatPage.model';
import { badgeClassName, buttonClassName, kpiTrendClassName } from './UnifiedChatPage.helpers';
import { AssistantMarkdown } from './UnifiedChatMarkdown';

type ChatBlocksProps = {
  messageId: string;
  blocks: PymesAssistantChatBlock[];
  busy: boolean;
  onAction: (action: PymesAssistantAction) => void;
  t: (key: string, variables?: Record<string, string | number>) => string;
};

function ChatBlocks({ messageId, blocks, busy, onAction, t }: ChatBlocksProps) {
  return (
    <div className="cht__blocks">
      {blocks.map((block, index) => {
        if (block.type === 'text') {
          return (
            <div key={`${messageId}-block-${index}`} className="cht__block-text">
              <AssistantMarkdown text={block.text} />
            </div>
          );
        }
        if (block.type === 'actions') {
          const actions = block.actions ?? [];
          return (
            <div key={`${messageId}-block-${index}`} className="cht__block-actions">
              {actions.map((action) => (
                <button
                  key={action.id}
                  type="button"
                  className={buttonClassName(action.style)}
                  disabled={busy}
                  onClick={() => onAction(action)}
                >
                  {action.label}
                </button>
              ))}
            </div>
          );
        }
        if (block.type === 'insight_card') {
          return (
            <section key={`${messageId}-block-${index}`} className="cht__insight-card">
              <div className="cht__insight-title">{block.title}</div>
              {block.scope ? <div className="cht__insight-scope">{block.scope}</div> : null}
              <p className="cht__insight-summary">{block.summary}</p>
              {block.highlights?.length ? (
                <div className="cht__insight-highlights">
                  {block.highlights.map((item) => (
                    <div key={`${item.label}-${item.value}`} className="cht__insight-highlight">
                      <span>{item.label}</span>
                      <strong>{item.value}</strong>
                    </div>
                  ))}
                </div>
              ) : null}
              {block.recommendations?.length ? (
                <div className="cht__insight-recommendations">
                  {block.recommendations.map((item) => (
                    <div key={item} className="cht__insight-recommendation">
                      {item}
                    </div>
                  ))}
                </div>
              ) : null}
            </section>
          );
        }
        if (block.type === 'kpi_group') {
          const items = block.items ?? [];
          return (
            <section key={`${messageId}-block-${index}`} className="cht__kpi-group">
              {block.title ? <div className="cht__kpi-group-title">{block.title}</div> : null}
              <div className="cht__kpi-grid">
                {items.map((item) => (
                  <div key={`${item.label}-${item.value}`} className="cht__kpi-item">
                    <div className="cht__kpi-item-label">{item.label}</div>
                    <div className="cht__kpi-item-value">{item.value}</div>
                    {item.context ? <div className={kpiTrendClassName(item.trend)}>{item.context}</div> : null}
                  </div>
                ))}
              </div>
            </section>
          );
        }
        if (block.type === 'table') {
          const columns = block.columns ?? [];
          const rows = block.rows ?? [];
          return (
            <section key={`${messageId}-block-${index}`} className="cht__table-block">
              <div className="cht__table-title">{block.title}</div>
              {rows.length > 0 ? (
                <div className="cht__table-wrap">
                  <table className="cht__table">
                    <thead>
                      <tr>
                        {columns.map((column) => (
                          <th key={column}>{column}</th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {rows.map((row, rowIndex) => (
                        <tr key={`${block.title}-row-${rowIndex}`}>
                          {row.map((cell, cellIndex) => (
                            <td key={`${block.title}-row-${rowIndex}-cell-${cellIndex}`}>{cell}</td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="cht__table-empty">{block.empty_state ?? t('ai.chat.table.empty')}</div>
              )}
            </section>
          );
        }
        return null;
      })}
    </div>
  );
}

type ChatThreadProps = {
  messages: Msg[];
  busy: boolean;
  loadingHistory: boolean;
  endRef: RefObject<HTMLDivElement>;
  onAction: (action: PymesAssistantAction) => void;
  t: (key: string, variables?: Record<string, string | number>) => string;
};

export function ChatThread({ messages, busy, loadingHistory, endRef, onAction, t }: ChatThreadProps) {
  return (
    <div
      className="cht__messages"
      role="log"
      aria-live="polite"
      aria-relevant="additions text"
      aria-busy={busy || loadingHistory}
      aria-label={t('ai.chat.messagesAria')}
    >
      {loadingHistory && <div className="spinner cht__history-spinner" role="status" aria-label="Cargando historial" />}
      {messages.map((m) => (
        <div key={m.id} className={`cht__msg ${m.fromMe ? 'cht__msg--me' : 'cht__msg--them'}`}>
          {m.badgeLabels && m.badgeLabels.length > 0 ? (
            <div className="cht__msg-badges">
              {m.badgeLabels.map((badge, index) => (
                <span key={`${m.id}-${badge}`} className={badgeClassName(m.badgeTones?.[index] ?? 'neutral')}>
                  {badge}
                </span>
              ))}
            </div>
          ) : null}
          {m.blocks && m.blocks.length > 0 ? (
            <ChatBlocks messageId={m.id} blocks={m.blocks} busy={busy} onAction={onAction} t={t} />
          ) : m.fromMe ? (
            m.text
          ) : (
            <AssistantMarkdown text={m.text} />
          )}
          <div className="cht__msg-time">{m.time}</div>
        </div>
      ))}
      <div ref={endRef} />
    </div>
  );
}
