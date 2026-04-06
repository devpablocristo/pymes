import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { useSearch } from '@devpablocristo/modules-search';
import {
  pymesAssistantChat,
  listConversations,
  getConversation,
  type ConversationSummary,
} from '../lib/aiApi';
import { humanInsightScopeLabel, humanRoutedLabel, humanRoutingSourceLabel } from '../lib/aiLabels';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { useI18n, type LanguageCode } from '../lib/i18n';
import {
  NOTIFICATION_CHAT_HANDOFF_KEY,
  buildHandoffUserMessage,
  type NotificationChatHandoff,
} from '../lib/notificationChatHandoff';
import type { CommercialChatRequest, PymesAssistantAction, PymesAssistantChatBlock, PymesAssistantChatResponse } from '../types/aiChat';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { queryKeys } from '../lib/queryKeys';
import './UnifiedChatPage.css';

type ContactKind = 'human' | 'ai_pymes';
type ManualRouteHint = Exclude<NonNullable<CommercialChatRequest['route_hint']>, 'general' | 'copilot'>;

type ContactDef = {
  id: string;
  name: string;
  initials: string;
  color: string;
  kind: ContactKind;
  defaultPreview: string;
};

const AI_PYMES_ID = 'ai-pymes';
const HUMAN_CONTACT_DEFS: ContactDef[] = [
  {
    id: '1',
    name: 'María García',
    initials: 'MG',
    color: 'var(--color-primary)',
    kind: 'human',
    defaultPreview: 'Dale, hablamos mañana',
  },
  {
    id: '2',
    name: 'Juan Pérez',
    initials: 'JP',
    color: 'var(--color-success)',
    kind: 'human',
    defaultPreview: 'Perfecto, gracias!',
  },
  {
    id: '3',
    name: 'Ana López',
    initials: 'AL',
    color: 'var(--color-purple)',
    kind: 'human',
    defaultPreview: 'Te envío el presupuesto',
  },
  {
    id: '4',
    name: 'Carlos Ruiz',
    initials: 'CR',
    color: 'var(--color-warning)',
    kind: 'human',
    defaultPreview: 'Listo el deploy',
  },
  {
    id: '5',
    name: 'Laura Díaz',
    initials: 'LD',
    color: 'var(--color-accent-pink)',
    kind: 'human',
    defaultPreview: 'Quedó excelente!',
  },
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
  blocks?: PymesAssistantChatBlock[];
  routedLabel?: string;
  metaLabel?: string;
  badgeLabels?: string[];
  badgeTones?: MsgBadgeTone[];
};

type MsgBadgeTone = 'ventas' | 'cobros' | 'compras' | 'clientes' | 'productos' | 'general' | 'neutral';
type AssistantReplyRow = Pick<Msg, 'text' | 'fromMe' | 'routedLabel' | 'blocks' | 'metaLabel' | 'badgeLabels' | 'badgeTones'>;

let nextMsgId = 100;

function normalizeManualRouteHint(value: string | null | undefined): ManualRouteHint | undefined {
  if (
    value === 'clientes' ||
    value === 'productos' ||
    value === 'ventas' ||
    value === 'cobros' ||
    value === 'compras'
  ) {
    return value;
  }
  return undefined;
}

function hasPromptForQueryBlock(blocks: PymesAssistantChatBlock[] | undefined): boolean {
  return Boolean(
    blocks?.some(
      (block) =>
        block.type === 'actions' &&
        (block.actions ?? []).some((action) => action.selection_behavior === 'prompt_for_query'),
    ),
  );
}

function buttonClassName(style?: PymesAssistantAction['style']): string {
  if (style === 'primary') return 'btn-primary btn-sm';
  if (style === 'ghost') return 'cht__block-action cht__block-action--ghost';
  return 'btn-secondary btn-sm';
}

function kpiTrendClassName(trend?: 'up' | 'down' | 'flat' | 'unknown' | null): string {
  if (trend === 'up') return 'cht__kpi-item-trend cht__kpi-item-trend--up';
  if (trend === 'down') return 'cht__kpi-item-trend cht__kpi-item-trend--down';
  if (trend === 'flat') return 'cht__kpi-item-trend cht__kpi-item-trend--flat';
  return 'cht__kpi-item-trend';
}

function badgeToneForRoute(mode: string | null | undefined): MsgBadgeTone {
  if (mode === 'ventas' || mode === 'internal_sales') return 'ventas';
  if (mode === 'cobros') return 'cobros';
  if (mode === 'compras' || mode === 'internal_procurement') return 'compras';
  if (mode === 'clientes') return 'clientes';
  if (mode === 'productos') return 'productos';
  if (mode === 'general' || mode === 'copilot') return 'general';
  return 'neutral';
}

function badgeClassName(tone: MsgBadgeTone): string {
  return `cht__msg-badge cht__msg-badge--${tone}`;
}

function localeForLanguage(language: LanguageCode): string {
  return language === 'en' ? 'en-US' : 'es-AR';
}

function formatChatTime(language: LanguageCode): string {
  return new Date().toLocaleTimeString(localeForLanguage(language), { hour: '2-digit', minute: '2-digit' });
}

function formatIsoTime(iso: string | null | undefined, language: LanguageCode): string {
  if (!iso) return '';
  try {
    return new Date(iso).toLocaleTimeString(localeForLanguage(language), { hour: '2-digit', minute: '2-digit' });
  } catch {
    return '';
  }
}

function resolvePreferredLanguage(contentLanguage: string | undefined, fallbackLanguage: LanguageCode): LanguageCode {
  if (contentLanguage === 'en' || contentLanguage === 'es') {
    return contentLanguage;
  }
  return fallbackLanguage;
}

function humanBadgeCategoryLabel(mode: string, language: LanguageCode): string {
  if (mode === 'clientes') return humanRoutedLabel('clientes', language);
  if (mode === 'productos') return humanRoutedLabel('productos', language);
  if (mode === 'ventas') return humanRoutedLabel('ventas', language);
  if (mode === 'cobros') return humanRoutedLabel('cobros', language);
  if (mode === 'compras') return humanRoutedLabel('compras', language);
  return humanRoutedLabel('general', language);
}

function buildAssistantMetaLabel(
  reply: PymesAssistantChatResponse,
  language: LanguageCode,
  t: (key: string, variables?: Record<string, string | number>) => string,
): string {
  const parts = [
    `${t('ai.chat.meta.request')} ${reply.request_id}`,
    reply.output_kind,
    humanRoutedLabel(reply.routed_agent || reply.routed_mode, language),
    humanRoutingSourceLabel(reply.routing_source, language),
  ];
  return parts.join(' · ');
}

function buildNotificationHandoffMetaLabel(
  handoff: NotificationChatHandoff,
  language: LanguageCode,
  t: (key: string, variables?: Record<string, string | number>) => string,
): string | null {
  const parts = [`${t('ai.chat.meta.notification')} ${handoff.notificationId}`];
  if (handoff.routedAgent) {
    parts.push(`${t('ai.chat.meta.agent')} ${humanRoutedLabel(handoff.routedAgent, language)}`);
  }
  if (handoff.scope) {
    parts.push(`${t('ai.chat.meta.context')} ${humanInsightScopeLabel(handoff.scope, language)}`);
  }
  return parts.length > 0 ? parts.join(' · ') : null;
}

function buildRouteHintMetaLabel(
  routeHint: CommercialChatRequest['route_hint'],
  language: LanguageCode,
  t: (key: string, variables?: Record<string, string | number>) => string,
): string | undefined {
  if (!routeHint) return undefined;
  return `${t('ai.chat.meta.manualRoute')} · ${humanRoutedLabel(routeHint, language)}`;
}

function buildAssistantBadgeLabels(reply: PymesAssistantChatResponse, language: LanguageCode): string[] {
  return [humanBadgeCategoryLabel(reply.routed_agent || reply.routed_mode, language)];
}

function buildAssistantBadgeTones(reply: PymesAssistantChatResponse): MsgBadgeTone[] {
  return [badgeToneForRoute(reply.routed_agent || reply.routed_mode)];
}

function applyPymesReply(
  reply: PymesAssistantChatResponse,
  language: LanguageCode,
  t: (key: string, variables?: Record<string, string | number>) => string,
): AssistantReplyRow[] {
  const agentLabel = humanRoutedLabel(reply.routed_agent || reply.routed_mode, language);
  const sourceLabel = humanRoutingSourceLabel(reply.routing_source, language);
  const routedLabel = agentLabel === sourceLabel ? agentLabel : `${agentLabel} · ${sourceLabel}`;
  return [
    {
      text: reply.reply,
      fromMe: false,
      routedLabel,
      metaLabel: buildAssistantMetaLabel(reply, language, t),
      badgeLabels: buildAssistantBadgeLabels(reply, language),
      badgeTones: buildAssistantBadgeTones(reply),
      blocks: reply.blocks ?? [],
    },
  ];
}

function materializeAssistantReplyRows(contactId: string, rows: AssistantReplyRow[], language: LanguageCode): Msg[] {
  const time = formatChatTime(language);
  return rows.map((row) => ({
    id: String(++nextMsgId),
    contactId,
    text: row.text,
    blocks: row.blocks,
    fromMe: row.fromMe,
    time,
    routedLabel: row.routedLabel,
    metaLabel: row.metaLabel,
    badgeLabels: row.badgeLabels,
    badgeTones: row.badgeTones,
  }));
}

function buildAssistantMessages(
  contactId: string,
  reply: PymesAssistantChatResponse,
  language: LanguageCode,
  t: (key: string, variables?: Record<string, string | number>) => string,
): Msg[] {
  return materializeAssistantReplyRows(contactId, applyPymesReply(reply, language, t), language);
}

function resolveInputPrompt(
  kind: ContactKind,
  routeHint: ManualRouteHint | undefined,
  language: LanguageCode,
  t: (key: string, variables?: Record<string, string | number>) => string,
): string {
  if (kind === 'human') {
    return t('ai.chat.input.humanPlaceholder');
  }
  if (routeHint) {
    return t('ai.chat.input.routePlaceholder', {
      label: humanRoutedLabel(routeHint, language),
    });
  }
  return t('ai.chat.input.defaultPlaceholder');
}

export function UnifiedChatPage() {
  const { language, t } = useI18n();
  const [searchParams] = useSearchParams();
  const contactDefs = useMemo<ContactDef[]>(
    () => [
      {
        id: AI_PYMES_ID,
        name: 'Asistente Pymes',
        initials: 'AP',
        color: 'var(--color-accent-indigo)',
        kind: 'ai_pymes',
        defaultPreview: t('ai.chat.input.defaultPlaceholder'),
      },
      ...HUMAN_CONTACT_DEFS,
    ],
    [t],
  );
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
  const [chatIds, setChatIds] = useState<Record<string, string | undefined>>({});
  const [pendingConfirmationsByContact, setPendingConfirmationsByContact] = useState<Record<string, string[]>>({});
  const [pendingRouteHintsByContact, setPendingRouteHintsByContact] = useState<
    Record<string, ManualRouteHint | undefined>
  >({});
  const [input, setInput] = useState('');
  const search = usePageSearch();
  const [error, setError] = useState('');
  const endRef = useRef<HTMLDivElement>(null);
  const notificationHandoffInFlightRef = useRef(false);
  const initialConversationHydratedRef = useRef(false);
  const skipInitialAiHydrationRef = useRef(
    typeof sessionStorage !== 'undefined' && Boolean(sessionStorage.getItem(NOTIFICATION_CHAT_HANDOFF_KEY)),
  );
  const [historyConversationId, setHistoryConversationId] = useState<string | null>(null);
  const queryClient = useQueryClient();

  // ── Persistencia: conversaciones guardadas ──
  const conversationsQuery = useQuery({
    queryKey: queryKeys.ai.conversations.list(30),
    queryFn: () => listConversations(30),
  });
  const conversationDetailQuery = useQuery({
    queryKey: queryKeys.ai.conversations.detail(historyConversationId ?? ''),
    queryFn: () => getConversation(historyConversationId ?? ''),
    enabled: historyConversationId !== null,
  });
  const chatMutation = useMutation({
    mutationFn: pymesAssistantChat,
    onSuccess: async (reply) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.ai.conversations.list(30) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.ai.conversations.detail(reply.chat_id) }),
      ]);
    },
  });
  const savedConversations = useMemo(() => conversationsQuery.data?.items ?? [], [conversationsQuery.data?.items]);
  const loadingHistory = conversationDetailQuery.isFetching;
  const busy = chatMutation.isPending;

  useEffect(() => {
    if (initialConversationHydratedRef.current) return;
    if (skipInitialAiHydrationRef.current) {
      initialConversationHydratedRef.current = true;
      return;
    }
    if (savedConversations.length > 0 && !chatIds[AI_PYMES_ID]) {
      const latest = savedConversations[0];
      initialConversationHydratedRef.current = true;
      setChatIds((prev) => ({ ...prev, [AI_PYMES_ID]: latest.id }));
      setHistoryConversationId(latest.id);
      return;
    }
    if (conversationsQuery.isSuccess) {
      initialConversationHydratedRef.current = true;
    }
  }, [chatIds, conversationsQuery.isSuccess, savedConversations]);

  useEffect(() => {
    if (conversationDetailQuery.data) {
      const detail = conversationDetailQuery.data;
      const restored: Msg[] = detail.messages.map((m, i) => ({
        id: `restored-${detail.id}-${i}`,
        contactId: AI_PYMES_ID,
        text: m.content,
        fromMe: m.role === 'user',
        time: formatIsoTime(m.ts, language),
      }));
      if (restored.length > 0) {
        setMsgs((prev) => [...prev.filter((p) => p.contactId !== AI_PYMES_ID), ...restored]);
      }
    }
  }, [conversationDetailQuery.data, historyConversationId, language]);

  function selectSavedConversation(conv: ConversationSummary) {
    setActive(AI_PYMES_ID);
    setChatIds((prev) => ({ ...prev, [AI_PYMES_ID]: conv.id }));
    setHistoryConversationId(conv.id);
    setError('');
  }

  const activeDef = useMemo(() => contactDefs.find((c) => c.id === active)!, [active, contactDefs]);
  const thread = useMemo(() => msgs.filter((m) => m.contactId === active), [msgs, active]);
  const activePendingConfirmations = useMemo(
    () => pendingConfirmationsByContact[active] ?? [],
    [pendingConfirmationsByContact, active],
  );
  const activePendingRouteHint = pendingRouteHintsByContact[active];
  const inputPrompt = useMemo(
    () => resolveInputPrompt(activeDef.kind, activePendingRouteHint, language, t),
    [activeDef.kind, activePendingRouteHint, language, t],
  );

  const contactsView = useMemo(() => {
    return contactDefs.map((c) => {
      const last = msgs.filter((m) => m.contactId === c.id).at(-1);
      return {
        ...c,
        lastMsg: last?.text ?? c.defaultPreview,
      };
    });
  }, [contactDefs, msgs]);

  useEffect(() => {
    const agent = searchParams.get('agent');
    const legacy = searchParams.get('legacy');
    if (agent === 'ai-sales' || agent === 'ai-procurement' || legacy === 'commercial') {
      setActive(AI_PYMES_ID);
      return;
    }
    if (agent && contactDefs.some((c) => c.id === agent)) {
      setActive(agent);
    }
  }, [contactDefs, searchParams]);

  // Aviso in-app → Asistente Pymes: primer turno automático con contexto
  useEffect(() => {
    if (typeof sessionStorage === 'undefined') return;
    if (notificationHandoffInFlightRef.current) return;
    const raw = sessionStorage.getItem(NOTIFICATION_CHAT_HANDOFF_KEY);
    if (!raw) return;
    let handoff: NotificationChatHandoff;
    try {
      handoff = JSON.parse(raw) as NotificationChatHandoff;
    } catch {
      sessionStorage.removeItem(NOTIFICATION_CHAT_HANDOFF_KEY);
      return;
    }
    notificationHandoffInFlightRef.current = true;
    skipInitialAiHydrationRef.current = true;
    initialConversationHydratedRef.current = true;
    sessionStorage.removeItem(NOTIFICATION_CHAT_HANDOFF_KEY);

    const text = buildHandoffUserMessage(handoff);
    setActive(AI_PYMES_ID);
    setHistoryConversationId(null);
    setChatIds((prev) => {
      const next = { ...prev };
      delete next[AI_PYMES_ID];
      return next;
    });
    setPendingConfirmationsByContact((prev) => {
      const next = { ...prev };
      delete next[AI_PYMES_ID];
      return next;
    });
    setPendingRouteHintsByContact((prev) => {
      const next = { ...prev };
      delete next[AI_PYMES_ID];
      return next;
    });

    const time = formatChatTime(language);
    const userMsg: Msg = {
      id: String(++nextMsgId),
      contactId: AI_PYMES_ID,
      text,
      fromMe: true,
      time,
      metaLabel: buildNotificationHandoffMetaLabel(handoff, language, t) ?? undefined,
    };
    setMsgs((prev) => [...prev.filter((msg) => msg.contactId !== AI_PYMES_ID), userMsg]);

    setError('');
    const run = async () => {
      try {
        const reply = await chatMutation.mutateAsync({
          message: text,
          chat_id: null,
          confirmed_actions: [],
          route_hint: handoff.routedAgent === 'copilot' ? 'copilot' : undefined,
          preferred_language: resolvePreferredLanguage(handoff.contentLanguage, language),
        });
        setChatIds((prev) => ({ ...prev, [AI_PYMES_ID]: reply.chat_id }));
        setPendingConfirmationsByContact((prev) => ({
          ...prev,
          [AI_PYMES_ID]: reply.pending_confirmations ?? [],
        }));
        const additions = buildAssistantMessages(AI_PYMES_ID, reply, language, t);
        setMsgs((p) => [...p, ...additions]);
      } catch (err) {
        setError(formatFetchErrorForUser(err, t('ai.chat.error.unreachable')));
      }
    };
    void run();
  }, [chatMutation, language, t]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [thread.length, active]);

  const clearAiThread = useCallback(() => {
    setMsgs((prev) => prev.filter((m) => m.contactId !== active));
    setChatIds((prev) => {
      const next = { ...prev };
      delete next[active];
      return next;
    });
    setPendingConfirmationsByContact((prev) => {
      const next = { ...prev };
      delete next[active];
      return next;
    });
    setPendingRouteHintsByContact((prev) => {
      const next = { ...prev };
      delete next[active];
      return next;
    });
    setHistoryConversationId(null);
    setError('');
  }, [active]);

  const sendAssistantMessage = useCallback(
    async (
      text: string,
      options?: {
        confirmedActions?: string[];
        echoText?: string;
        clearInput?: boolean;
        routeHint?: ManualRouteHint;
      },
    ) => {
      const trimmed = text.trim();
      if (!trimmed || busy) return;
      const inheritedRouteHint = pendingRouteHintsByContact[active] ?? null;
      const apiRouteHint: CommercialChatRequest['route_hint'] = options?.routeHint ?? inheritedRouteHint;

      const time = formatChatTime(language);
      const userMsg: Msg = {
        id: String(++nextMsgId),
        contactId: active,
        text: options?.echoText ?? trimmed,
        fromMe: true,
        time,
        metaLabel: buildRouteHintMetaLabel(apiRouteHint, language, t),
      };
      setMsgs((p) => [...p, userMsg]);
      if (options?.clearInput ?? true) {
        setInput('');
      }

      setError('');
      const chatId = chatIds[active];
      try {
        const reply = await chatMutation.mutateAsync({
          message: trimmed,
          chat_id: chatId ?? null,
          confirmed_actions: options?.confirmedActions ?? [],
          route_hint: apiRouteHint,
          preferred_language: language,
        });
        setChatIds((prev) => ({ ...prev, [active]: reply.chat_id }));
        setPendingConfirmationsByContact((prev) => ({
          ...prev,
          [active]: reply.pending_confirmations ?? [],
        }));
        if (hasPromptForQueryBlock(reply.blocks)) {
          setPendingRouteHintsByContact((prev) => ({
            ...prev,
            [active]: undefined,
          }));
        } else {
          const nextStickyRouteHint =
            normalizeManualRouteHint(reply.routed_agent || reply.routed_mode) ?? apiRouteHint ?? undefined;
          if (nextStickyRouteHint) {
            setPendingRouteHintsByContact((prev) => ({
              ...prev,
              [active]: nextStickyRouteHint,
            }));
          }
        }
        const additions = buildAssistantMessages(active, reply, language, t);
        setMsgs((p) => [...p, ...additions]);
      } catch (err) {
        setError(formatFetchErrorForUser(err, t('ai.chat.error.unreachable')));
      }
    },
    [active, busy, chatIds, chatMutation, language, pendingRouteHintsByContact, t],
  );

  const send = useCallback(async () => {
    const trimmed = input.trim();
    if (!trimmed || busy) return;

    if (activeDef.kind === 'human') {
      const time = formatChatTime(language);
      const userMsg: Msg = {
        id: String(++nextMsgId),
        contactId: active,
        text: trimmed,
        fromMe: true,
        time,
      };
      setMsgs((p) => [...p, userMsg]);
      setInput('');
      return;
    }
    await sendAssistantMessage(trimmed, { clearInput: true });
  }, [active, activeDef.kind, busy, input, language, sendAssistantMessage]);

  const confirmPendingActions = useCallback(async () => {
    if (activeDef.kind !== 'ai_pymes' || activePendingConfirmations.length === 0 || busy) {
      return;
    }
    await sendAssistantMessage(t('ai.chat.action.confirmPending'), {
      confirmedActions: activePendingConfirmations,
      echoText: t('ai.chat.action.confirmEcho', { actions: activePendingConfirmations.join(', ') }),
      clearInput: false,
    });
  }, [activeDef.kind, activePendingConfirmations, busy, sendAssistantMessage, t]);

  const handleAssistantBlockAction = useCallback(
    async (action: PymesAssistantAction) => {
      if (busy) return;
      if (action.kind === 'open_url' && action.url) {
        window.location.assign(action.url);
        return;
      }
      if (activeDef.kind !== 'ai_pymes') return;
      if (action.kind === 'confirm_action') {
        await sendAssistantMessage(action.message ?? t('ai.chat.action.confirmPending'), {
          confirmedActions: action.confirmed_actions ?? [],
          echoText: action.label,
          clearInput: false,
        });
        return;
      }
      if (action.kind === 'send_message') {
        if (action.selection_behavior === 'prompt_for_query' && action.route_hint) {
          const routeHint = action.route_hint as ManualRouteHint;
          const routeLabel = humanRoutedLabel(routeHint, language);
          const now = formatChatTime(language);
          setPendingRouteHintsByContact((prev) => ({
            ...prev,
            [active]: routeHint,
          }));
          setMsgs((prev) => [
            ...prev,
            {
              id: String(++nextMsgId),
              contactId: active,
              text: t('ai.chat.action.categoryPrefix', { label: action.label }),
              fromMe: true,
              time: now,
            },
            {
              id: String(++nextMsgId),
              contactId: active,
              text: t('ai.chat.action.askAboutRoute', { label: routeLabel }),
              fromMe: false,
              time: now,
              badgeLabels: [routeLabel],
              badgeTones: [badgeToneForRoute(routeHint)],
            },
          ]);
          return;
        }
        await sendAssistantMessage(action.message ?? action.label, {
          echoText: action.route_hint ? t('ai.chat.action.categoryPrefix', { label: action.label }) : undefined,
          clearInput: false,
          routeHint: action.route_hint as ManualRouteHint | undefined,
        });
      }
    },
    [active, activeDef.kind, busy, language, sendAssistantMessage, t],
  );

  const contactTextFn = useCallback((c: (typeof contactsView)[number]) => c.name, []);
  const filteredContacts = useSearch(contactsView, contactTextFn, search);

  return (
    <PageLayout className="cht" title={t('ai.chat.pageTitle')} lead={t('ai.chat.pageLead')}>
      <div className="cht__layout">
        <div className="cht__contacts">
          <nav className="cht__contacts-list" aria-label="Contactos y conversaciones">
            {filteredContacts.map((c) => (
              <button
                key={c.id}
                type="button"
                className={`cht__contact ${active === c.id ? 'cht__contact--active' : ''}`}
                aria-pressed={active === c.id}
                aria-label={`${c.name}. ${c.lastMsg}`}
                onClick={() => {
                  setActive(c.id);
                  setError('');
                }}
              >
                <div className="cht__contact-avatar" style={{ '--cht-avatar-bg': c.color } as CSSProperties}>
                  {c.initials}
                </div>
                <div className="cht__contact-info">
                  <div className="cht__contact-name">{c.name}</div>
                  <div className="cht__contact-preview">{c.lastMsg}</div>
                </div>
              </button>
            ))}
            {/* Conversaciones previas guardadas */}
            {savedConversations.length > 0 && (
              <>
                <div className="cht__conversations-divider">
                  {t('ai.chat.previousConversations') || 'Conversaciones anteriores'}
                </div>
                {savedConversations.map((conv) => (
                  <button
                    key={conv.id}
                    type="button"
                    className={`cht__contact ${chatIds[AI_PYMES_ID] === conv.id ? 'cht__contact--active' : ''}`}
                    aria-pressed={chatIds[AI_PYMES_ID] === conv.id}
                    aria-label={`${conv.title || 'Sin título'}. ${conv.message_count} mensajes`}
                    onClick={() => selectSavedConversation(conv)}
                  >
                    <div className="cht__contact-avatar cht__contact-avatar--saved">AP</div>
                    <div className="cht__contact-info">
                      <div className="cht__contact-name">{conv.title || 'Sin título'}</div>
                      <div className="cht__contact-preview">{conv.message_count} mensajes</div>
                    </div>
                  </button>
                ))}
              </>
            )}
          </nav>
        </div>
        <div className="cht__main">
          <div className="cht__header cht__header-row">
            <div className="cht__header-title">{activeDef.name}</div>
            {activeDef.kind !== 'human' ? (
              <button type="button" className="btn-secondary btn-sm" onClick={() => void clearAiThread()}>
                {t('ai.chat.newConversation')}
              </button>
            ) : null}
          </div>
          {error ? (
            <p role="alert" className="form-error cht__form-error-chat">
              {error}
            </p>
          ) : null}
          <div
            className="cht__messages"
            role="log"
            aria-live="polite"
            aria-relevant="additions text"
            aria-busy={busy || loadingHistory}
            aria-label={`Mensajes con ${activeDef.name}`}
          >
            {loadingHistory && (
              <div className="spinner cht__history-spinner" role="status" aria-label="Cargando historial" />
            )}
            {thread.map((m) => (
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
                  <div className="cht__blocks">
                    {m.blocks.map((block, index) => {
                      if (block.type === 'text') {
                        return (
                          <div key={`${m.id}-block-${index}`} className="cht__block-text">
                            {block.text}
                          </div>
                        );
                      }
                      if (block.type === 'actions') {
                        const actions = block.actions ?? [];
                        return (
                          <div key={`${m.id}-block-${index}`} className="cht__block-actions">
                            {actions.map((action) => (
                              <button
                                key={action.id}
                                type="button"
                                className={buttonClassName(action.style)}
                                disabled={busy}
                                onClick={() => void handleAssistantBlockAction(action)}
                              >
                                {action.label}
                              </button>
                            ))}
                          </div>
                        );
                      }
                      if (block.type === 'insight_card') {
                        return (
                          <section key={`${m.id}-block-${index}`} className="cht__insight-card">
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
                          <section key={`${m.id}-block-${index}`} className="cht__kpi-group">
                            {block.title ? <div className="cht__kpi-group-title">{block.title}</div> : null}
                            <div className="cht__kpi-grid">
                              {items.map((item) => (
                                <div key={`${item.label}-${item.value}`} className="cht__kpi-item">
                                  <div className="cht__kpi-item-label">{item.label}</div>
                                  <div className="cht__kpi-item-value">{item.value}</div>
                                  {item.context ? (
                                    <div className={kpiTrendClassName(item.trend)}>{item.context}</div>
                                  ) : null}
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
                          <section key={`${m.id}-block-${index}`} className="cht__table-block">
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
                ) : (
                  m.text
                )}
                <div className="cht__msg-time">{m.time}</div>
              </div>
            ))}
            <div ref={endRef} />
          </div>
          {activeDef.kind === 'ai_pymes' && activePendingConfirmations.length > 0 ? (
            <div className="cht__pending-bar">
              <span>Pendientes: {activePendingConfirmations.join(', ')}</span>
              <button
                type="button"
                className="btn-secondary btn-sm"
                disabled={busy}
                onClick={() => void confirmPendingActions()}
              >
                Confirmar acciones
              </button>
            </div>
          ) : null}
          <div className="cht__input-bar">
            <input
              aria-label={inputPrompt}
              placeholder={inputPrompt}
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
              {busy ? t('ai.chat.sending') : t('ai.chat.send')}
            </button>
          </div>
        </div>
      </div>
    </PageLayout>
  );
}
