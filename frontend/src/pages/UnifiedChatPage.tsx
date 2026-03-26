import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { pymesAssistantChat, type PymesAssistantChatResponse } from '../lib/aiApi';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import {
  NOTIFICATION_CHAT_HANDOFF_KEY,
  buildHandoffUserMessage,
  type NotificationChatHandoff,
} from '../lib/notificationChatHandoff';
import './ChatDemoPage.css';

type ContactKind = 'human' | 'ai_pymes';

type ContactDef = {
  id: string;
  name: string;
  initials: string;
  color: string;
  kind: ContactKind;
  defaultPreview: string;
};

const AI_PYMES_ID = 'ai-pymes';

const CONTACT_DEFS: ContactDef[] = [
  {
    id: AI_PYMES_ID,
    name: 'Asistente Pymes',
    initials: 'AP',
    color: '#6366f1',
    kind: 'ai_pymes',
    defaultPreview: 'Ventas, compras internas y consultas del negocio…',
  },
  { id: '1', name: 'María García', initials: 'MG', color: '#3b82f6', kind: 'human', defaultPreview: 'Dale, hablamos mañana' },
  { id: '2', name: 'Juan Pérez', initials: 'JP', color: '#10b981', kind: 'human', defaultPreview: 'Perfecto, gracias!' },
  { id: '3', name: 'Ana López', initials: 'AL', color: '#8b5cf6', kind: 'human', defaultPreview: 'Te envío el presupuesto' },
  { id: '4', name: 'Carlos Ruiz', initials: 'CR', color: '#f59e0b', kind: 'human', defaultPreview: 'Listo el deploy' },
  { id: '5', name: 'Laura Díaz', initials: 'LD', color: '#ec4899', kind: 'human', defaultPreview: 'Quedó excelente!' },
];

const SEED_HUMAN_MESSAGES: Array<{
  id: string;
  contactId: string;
  text: string;
  fromMe: boolean;
  time: string;
}> = [
  { id: '1', contactId: '1', text: 'Hola! Cómo va el proyecto?', fromMe: false, time: '14:20' },
  { id: '2', contactId: '1', text: 'Bien, estamos cerrando el sprint.', fromMe: true, time: '14:22' },
  { id: '3', contactId: '1', text: 'Genial! Necesitás algo de mi lado?', fromMe: false, time: '14:23' },
  { id: '4', contactId: '1', text: 'Sí, la aprobación del diseño para avanzar.', fromMe: true, time: '14:25' },
  { id: '5', contactId: '1', text: 'Dale, hablamos mañana', fromMe: false, time: '14:30' },
  { id: '6', contactId: '2', text: 'Juan, te mandé el acceso al repo.', fromMe: true, time: '10:00' },
  { id: '7', contactId: '2', text: 'Perfecto, gracias!', fromMe: false, time: '10:05' },
  { id: '8', contactId: '3', text: 'Ana, tenés el presupuesto listo?', fromMe: true, time: '09:00' },
  { id: '9', contactId: '3', text: 'Te envío el presupuesto', fromMe: false, time: '09:15' },
];

type Msg = {
  id: string;
  contactId: string;
  text: string;
  fromMe: boolean;
  time: string;
  /** Sub-agente del orquestador (solo respuestas del Asistente Pymes). */
  routedLabel?: string;
};

let nextMsgId = 100;

function humanRoutedLabel(mode: string): string {
  if (mode === 'internal_procurement') return 'Compras internas';
  if (mode === 'internal_sales') return 'Ventas';
  return mode;
}

function applyPymesReply(reply: PymesAssistantChatResponse): Array<Pick<Msg, 'text' | 'fromMe' | 'routedLabel'>> {
  const label = humanRoutedLabel(reply.routed_mode);
  const out: Array<Pick<Msg, 'text' | 'fromMe' | 'routedLabel'>> = [
    { text: reply.reply, fromMe: false, routedLabel: label },
  ];
  if (reply.pending_confirmations?.length) {
    out.push({
      text: `Pendientes de confirmación: ${reply.pending_confirmations.join('; ')}`,
      fromMe: false,
      routedLabel: label,
    });
  }
  return out;
}

export function UnifiedChatPage() {
  const [searchParams] = useSearchParams();
  const [active, setActive] = useState(AI_PYMES_ID);
  const [msgs, setMsgs] = useState<Msg[]>(() =>
    SEED_HUMAN_MESSAGES.map((m) => ({
      id: m.id,
      contactId: m.contactId,
      text: m.text,
      fromMe: m.fromMe,
      time: m.time,
    })),
  );
  const [conversationIds, setConversationIds] = useState<Record<string, string | undefined>>({});
  const [input, setInput] = useState('');
  const [search, setSearch] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const endRef = useRef<HTMLDivElement>(null);
  /** Evita doble envío en React StrictMode (doble montaje del efecto). */
  const notificationHandoffInFlightRef = useRef(false);

  const activeDef = useMemo(() => CONTACT_DEFS.find((c) => c.id === active)!, [active]);
  const thread = useMemo(() => msgs.filter((m) => m.contactId === active), [msgs, active]);

  const contactsView = useMemo(() => {
    return CONTACT_DEFS.map((c) => {
      const last = msgs.filter((m) => m.contactId === c.id).at(-1);
      return {
        ...c,
        lastMsg: last?.text ?? c.defaultPreview,
      };
    });
  }, [msgs]);

  useEffect(() => {
    const agent = searchParams.get('agent');
    const legacy = searchParams.get('legacy');
    if (agent === 'ai-sales' || agent === 'ai-procurement' || legacy === 'commercial') {
      setActive(AI_PYMES_ID);
      return;
    }
    if (agent && CONTACT_DEFS.some((c) => c.id === agent)) {
      setActive(agent);
    }
  }, [searchParams]);

  // Aviso in-app → Asistente Pymes: primer turno automático con contexto (handoff vía sessionStorage).
  useEffect(() => {
    if (typeof sessionStorage === 'undefined') {
      return;
    }
    if (notificationHandoffInFlightRef.current) {
      return;
    }
    const raw = sessionStorage.getItem(NOTIFICATION_CHAT_HANDOFF_KEY);
    if (!raw) {
      return;
    }
    let handoff: NotificationChatHandoff;
    try {
      handoff = JSON.parse(raw) as NotificationChatHandoff;
    } catch {
      sessionStorage.removeItem(NOTIFICATION_CHAT_HANDOFF_KEY);
      return;
    }
    notificationHandoffInFlightRef.current = true;
    sessionStorage.removeItem(NOTIFICATION_CHAT_HANDOFF_KEY);

    const text = buildHandoffUserMessage(handoff);
    setActive(AI_PYMES_ID);

    const time = new Date().toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' });
    const userMsg: Msg = {
      id: String(++nextMsgId),
      contactId: AI_PYMES_ID,
      text,
      fromMe: true,
      time,
    };
    setMsgs((p) => [...p, userMsg]);

    setBusy(true);
    setError('');
    const run = async () => {
      try {
        const reply = await pymesAssistantChat({
          message: text,
          conversation_id: null,
          confirmed_actions: [],
        });
        setConversationIds((prev) => ({ ...prev, [AI_PYMES_ID]: reply.conversation_id }));
        const additions = applyPymesReply(reply).map(
          (row): Msg => ({
            id: String(++nextMsgId),
            contactId: AI_PYMES_ID,
            text: row.text,
            fromMe: row.fromMe,
            time: new Date().toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' }),
            routedLabel: row.routedLabel,
          }),
        );
        setMsgs((p) => [...p, ...additions]);
      } catch (err) {
        setError(
          formatFetchErrorForUser(
            err,
            'No se pudo contactar al asistente. Revisá VITE_AI_API_URL y el servicio AI.',
          ),
        );
      } finally {
        setBusy(false);
      }
    };
    void run();
  }, []);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [thread.length, active]);

  const clearAiThread = useCallback(() => {
    setMsgs((prev) => prev.filter((m) => m.contactId !== active));
    setConversationIds((prev) => {
      const next = { ...prev };
      delete next[active];
      return next;
    });
    setError('');
  }, [active]);

  const send = useCallback(async () => {
    const trimmed = input.trim();
    if (!trimmed || busy) return;

    const time = new Date().toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' });
    const userMsg: Msg = {
      id: String(++nextMsgId),
      contactId: active,
      text: trimmed,
      fromMe: true,
      time,
    };
    setMsgs((p) => [...p, userMsg]);
    setInput('');

    if (activeDef.kind === 'human') {
      return;
    }

    setBusy(true);
    setError('');
    const conv = conversationIds[active];
    const payload = {
      message: trimmed,
      conversation_id: conv ?? null,
      confirmed_actions: [] as string[],
    };
    try {
      const reply = await pymesAssistantChat(payload);
      setConversationIds((prev) => ({ ...prev, [active]: reply.conversation_id }));
      const additions = applyPymesReply(reply).map(
        (row): Msg => ({
          id: String(++nextMsgId),
          contactId: active,
          text: row.text,
          fromMe: row.fromMe,
          time: new Date().toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' }),
          routedLabel: row.routedLabel,
        }),
      );
      setMsgs((p) => [...p, ...additions]);
    } catch (err) {
      setError(
        formatFetchErrorForUser(
          err,
          'No se pudo contactar al asistente. Revisá VITE_AI_API_URL y el servicio AI.',
        ),
      );
    } finally {
      setBusy(false);
    }
  }, [active, activeDef.kind, busy, conversationIds, input]);

  const filteredContacts = contactsView.filter(
    (c) => !search || c.name.toLowerCase().includes(search.toLowerCase()),
  );

  return (
    <div className="cht">
      <div className="page-header">
        <h1>Chat</h1>
        <p style={{ color: 'var(--color-text-secondary)', margin: 0, fontSize: '0.88rem' }}>
          Personas y <strong>Asistente Pymes</strong> (orquestador: ventas y compras internas). Más agentes se sumarán
          aquí.
        </p>
      </div>
      <div className="cht__layout">
        <div className="cht__contacts">
          <div className="cht__contacts-header">
            <input
              className="cht__contacts-search"
              type="search"
              placeholder="Buscar…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <div className="cht__contacts-list">
            {filteredContacts.map((c) => (
              <button
                key={c.id}
                type="button"
                className={`cht__contact ${active === c.id ? 'cht__contact--active' : ''}`}
                onClick={() => {
                  setActive(c.id);
                  setError('');
                }}
              >
                <div className="cht__contact-avatar" style={{ background: c.color }}>
                  {c.initials}
                </div>
                <div className="cht__contact-info">
                  <div className="cht__contact-name">{c.name}</div>
                  <div className="cht__contact-preview">{c.lastMsg}</div>
                </div>
              </button>
            ))}
          </div>
        </div>
        <div className="cht__main">
          <div
            className="cht__header"
            style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}
          >
            <span style={{ flex: 1 }}>{activeDef.name}</span>
            {activeDef.kind !== 'human' ? (
              <button type="button" className="btn-secondary btn-sm" onClick={() => void clearAiThread()}>
                Nueva conversación
              </button>
            ) : null}
          </div>
          {error ? <p className="form-error" style={{ margin: '0.5rem 1rem 0' }}>{error}</p> : null}
          <div className="cht__messages">
            {thread.length === 0 && activeDef.kind !== 'human' ? (
              <p className="text-secondary" style={{ padding: '0 1rem' }}>
                Un solo asistente para consultas de ventas y de compras internas. Escribí tu consulta y el sistema enruta
                al sub-agente adecuado.
              </p>
            ) : null}
            {thread.map((m) => (
              <div key={m.id} className={`cht__msg ${m.fromMe ? 'cht__msg--me' : 'cht__msg--them'}`}>
                {m.routedLabel ? (
                  <div className="cht__msg-meta" style={{ fontSize: '0.7rem', opacity: 0.75, marginBottom: '0.25rem' }}>
                    {m.routedLabel}
                  </div>
                ) : null}
                {m.text}
                <div className="cht__msg-time">{m.time}</div>
              </div>
            ))}
            <div ref={endRef} />
          </div>
          <div className="cht__input-bar">
            <input
              placeholder={
                activeDef.kind === 'human'
                  ? 'Escribí un mensaje…'
                  : 'Ej.: resumí ventas del mes o el estado de solicitudes de compra…'
              }
              value={input}
              disabled={busy}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  void send();
                }
              }}
            />
            <button
              type="button"
              className="btn-primary btn-sm"
              disabled={busy || !input.trim()}
              onClick={() => void send()}
            >
              {busy ? 'Enviando…' : 'Enviar'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

export default UnifiedChatPage;
