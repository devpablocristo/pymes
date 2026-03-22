import { FormEvent, useState } from 'react';
import {
  commercialChatProcurement,
  commercialChatSales,
  type CommercialChatResponse,
} from '../lib/aiApi';
import { formatFetchErrorForUser } from '../lib/formatFetchError';

type Tab = 'sales' | 'procurement';

type ChatLine = { role: 'user' | 'assistant'; text: string };

export function CommercialAssistantPage() {
  const [tab, setTab] = useState<Tab>('sales');
  const [conversationSales, setConversationSales] = useState<string | undefined>();
  const [conversationProcurement, setConversationProcurement] = useState<string | undefined>();
  const [linesSales, setLinesSales] = useState<ChatLine[]>([]);
  const [linesProcurement, setLinesProcurement] = useState<ChatLine[]>([]);
  const [input, setInput] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const lines = tab === 'sales' ? linesSales : linesProcurement;
  const setLines = tab === 'sales' ? setLinesSales : setLinesProcurement;
  const conversationId = tab === 'sales' ? conversationSales : conversationProcurement;
  const setConversationId = tab === 'sales' ? setConversationSales : setConversationProcurement;

  function applyAssistantReply(reply: CommercialChatResponse): void {
    setConversationId(reply.conversation_id);
    setLines((prev) => {
      const next: ChatLine[] = [...prev, { role: 'assistant', text: reply.reply }];
      if (reply.pending_confirmations?.length) {
        next.push({
          role: 'assistant',
          text: `Pendientes de confirmación: ${reply.pending_confirmations.join('; ')}`,
        });
      }
      return next;
    });
  }

  async function onSubmit(e: FormEvent): Promise<void> {
    e.preventDefault();
    const trimmed = input.trim();
    if (!trimmed || busy) return;
    setBusy(true);
    setError('');
    setInput('');
    setLines((prev) => [...prev, { role: 'user', text: trimmed }]);
    try {
      const payload = {
        message: trimmed,
        conversation_id: conversationId ?? null,
        confirmed_actions: [] as string[],
      };
      const reply =
        tab === 'sales'
          ? await commercialChatSales(payload)
          : await commercialChatProcurement(payload);
      applyAssistantReply(reply);
    } catch (err) {
      setError(formatFetchErrorForUser(err, 'No se pudo contactar al asistente. Revisá VITE_AI_API_URL y el servicio AI.'));
    } finally {
      setBusy(false);
    }
  }

  function clearThread(): void {
    setConversationId(undefined);
    setLines([]);
    setError('');
  }

  return (
    <>
      <div className="page-header">
        <h1>Asistente comercial (AI)</h1>
        <p>Consultas de ventas o compras internas usando el mismo inicio de sesión que el core.</p>
      </div>

      <div className="card">
        <div className="actions-row u-mb-sm">
          <button
            type="button"
            className={tab === 'sales' ? 'btn-primary' : 'btn-secondary'}
            onClick={() => setTab('sales')}
          >
            Ventas
          </button>
          <button
            type="button"
            className={tab === 'procurement' ? 'btn-primary' : 'btn-secondary'}
            onClick={() => setTab('procurement')}
          >
            Compras internas
          </button>
          <button type="button" className="btn-secondary" onClick={clearThread}>
            Nueva conversación
          </button>
        </div>

        {error ? <p className="form-error">{error}</p> : null}

        <div className="admin-activity-wrap u-mb-md commercial-chat-log">
          {lines.length === 0 ? (
            <p className="text-secondary">Escribí un mensaje para comenzar.</p>
          ) : (
            lines.map((line, i) => (
              <div
                key={`${line.role}-${i}`}
                className="u-mb-sm"
                style={{
                  textAlign: line.role === 'user' ? 'right' : 'left',
                }}
              >
                <span
                  className={`badge ${line.role === 'user' ? 'badge-neutral' : 'badge-success'}`}
                  style={{ display: 'inline-block', maxWidth: '100%', whiteSpace: 'pre-wrap', textAlign: 'left' }}
                >
                  {line.text}
                </span>
              </div>
            ))
          )}
        </div>

        <form onSubmit={(e) => void onSubmit(e)}>
          <div className="form-group">
            <label htmlFor="commercial-chat-input">Mensaje</label>
            <textarea
              id="commercial-chat-input"
              className="admin-textarea"
              rows={3}
              value={input}
              disabled={busy}
              onChange={(e) => setInput(e.target.value)}
              placeholder="Ej.: resumí las ventas del mes o el estado de solicitudes de compra…"
            />
          </div>
          <button type="submit" className="btn-primary" disabled={busy || !input.trim()}>
            {busy ? 'Enviando…' : 'Enviar'}
          </button>
        </form>
      </div>
    </>
  );
}
