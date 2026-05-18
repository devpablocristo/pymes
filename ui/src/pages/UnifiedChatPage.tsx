import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearch } from '@devpablocristo/platform-search';
import { pymesAssistantChat, listConversations, getConversation } from '../lib/aiApi';
import { humanRoutedLabel } from '../lib/aiLabels';
import { useI18n } from '../lib/i18n';
import {
  NOTIFICATION_CHAT_HANDOFF_KEY,
  buildChatRequestHandoff,
  buildHandoffUserMessage,
  type NotificationChatHandoff,
} from '../lib/notificationChatHandoff';
import type { CommercialChatRequest, PymesAssistantAction } from '../types/aiChat';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { queryKeys } from '../lib/queryKeys';
import { AI_PYMES_ID, type ManualRouteHint, type Msg } from './UnifiedChatPage.model';
import {
  badgeToneForRoute,
  buildAssistantMessages,
  buildNotificationHandoffMetaLabel,
  buildRouteHintMetaLabel,
  formatAssistantHttpError,
  formatChatTime,
  formatIsoTime,
  hasPromptForQueryBlock,
  nextChatMsgId,
  normalizeManualRouteHint,
  resolveInputPrompt,
  resolvePreferredLanguage,
} from './UnifiedChatPage.helpers';
import { ChatComposer } from './UnifiedChatComposer';
import { ChatThread } from './UnifiedChatThread';
export { AssistantMarkdown } from './UnifiedChatMarkdown';
import './UnifiedChatPage.css';

export function UnifiedChatPage() {
  const { language, t } = useI18n();
  const [msgs, setMsgs] = useState<Msg[]>([]);
  const [chatIds, setChatIds] = useState<Record<string, string | undefined>>({});
  const [pendingConfirmationsByContact, setPendingConfirmationsByContact] = useState<Record<string, string[]>>({});
  const [pendingRouteHintsByContact, setPendingRouteHintsByContact] = useState<
    Record<string, ManualRouteHint | undefined>
  >({});
  const [input, setInput] = useState('');
  const pageSearch = usePageSearch();
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
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.ai.conversations.list(30) });
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
        blocks: m.role === 'assistant' ? (m.blocks ?? []) : undefined,
        fromMe: m.role === 'user',
        time: formatIsoTime(m.ts, language),
      }));
      if (restored.length > 0) {
        setMsgs((prev) => [...prev.filter((p) => p.contactId !== AI_PYMES_ID), ...restored]);
      }
    }
  }, [conversationDetailQuery.data, historyConversationId, language]);

  const thread = useMemo(() => msgs.filter((m) => m.contactId === AI_PYMES_ID), [msgs]);
  const threadSearchText = useCallback(
    (m: Msg) => [m.text, m.routedLabel, m.metaLabel, ...(m.badgeLabels ?? [])].filter(Boolean).join(' '),
    [],
  );
  const filteredThread = useSearch(thread, threadSearchText, pageSearch);
  const activePendingConfirmations = useMemo(
    () => pendingConfirmationsByContact[AI_PYMES_ID] ?? [],
    [pendingConfirmationsByContact],
  );
  const activePendingRouteHint = pendingRouteHintsByContact[AI_PYMES_ID];
  const inputPrompt = useMemo(
    () => resolveInputPrompt(activePendingRouteHint, language, t),
    [activePendingRouteHint, language, t],
  );

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
      id: nextChatMsgId(),
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
          handoff: buildChatRequestHandoff(handoff),
          route_hint: handoff.routedAgent === 'insight_chat' ? 'insight_chat' : undefined,
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
        setError(formatAssistantHttpError(err, t('ai.chat.error.unreachable')));
      }
    };
    void run();
  }, [chatMutation, language, t]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [thread.length]);

  const clearAiThread = useCallback(() => {
    setMsgs((prev) => prev.filter((m) => m.contactId !== AI_PYMES_ID));
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
    setHistoryConversationId(null);
    setError('');
  }, []);

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
      const inheritedRouteHint = pendingRouteHintsByContact[AI_PYMES_ID] ?? null;
      const apiRouteHint: CommercialChatRequest['route_hint'] = options?.routeHint ?? inheritedRouteHint;

      const time = formatChatTime(language);
      const userMsg: Msg = {
        id: nextChatMsgId(),
        contactId: AI_PYMES_ID,
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
      const chatId = chatIds[AI_PYMES_ID];
      try {
        const reply = await chatMutation.mutateAsync({
          message: trimmed,
          chat_id: chatId ?? null,
          confirmed_actions: options?.confirmedActions ?? [],
          route_hint: apiRouteHint,
          preferred_language: language,
        });
        setChatIds((prev) => ({ ...prev, [AI_PYMES_ID]: reply.chat_id }));
        setPendingConfirmationsByContact((prev) => ({
          ...prev,
          [AI_PYMES_ID]: reply.pending_confirmations ?? [],
        }));
        if (hasPromptForQueryBlock(reply.blocks)) {
          setPendingRouteHintsByContact((prev) => ({
            ...prev,
            [AI_PYMES_ID]: undefined,
          }));
        } else {
          const nextStickyRouteHint = normalizeManualRouteHint(reply.routed_agent) ?? apiRouteHint ?? undefined;
          if (nextStickyRouteHint) {
            setPendingRouteHintsByContact((prev) => ({
              ...prev,
              [AI_PYMES_ID]: nextStickyRouteHint,
            }));
          }
        }
        const additions = buildAssistantMessages(AI_PYMES_ID, reply, language, t);
        setMsgs((p) => [...p, ...additions]);
      } catch (err) {
        setError(formatAssistantHttpError(err, t('ai.chat.error.unreachable')));
      }
    },
    [busy, chatIds, chatMutation, language, pendingRouteHintsByContact, t],
  );

  const send = useCallback(async () => {
    const trimmed = input.trim();
    if (!trimmed || busy) return;

    await sendAssistantMessage(trimmed, { clearInput: true });
  }, [busy, input, sendAssistantMessage]);

  const confirmPendingActions = useCallback(async () => {
    if (activePendingConfirmations.length === 0 || busy) {
      return;
    }
    await sendAssistantMessage(t('ai.chat.action.confirmPending'), {
      confirmedActions: activePendingConfirmations,
      echoText: t('ai.chat.action.confirmEcho', { actions: activePendingConfirmations.join(', ') }),
      clearInput: false,
    });
  }, [activePendingConfirmations, busy, sendAssistantMessage, t]);

  const handleAssistantBlockAction = useCallback(
    async (action: PymesAssistantAction) => {
      if (busy) return;
      if (action.kind === 'open_url' && action.url) {
        window.location.assign(action.url);
        return;
      }
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
            [AI_PYMES_ID]: routeHint,
          }));
          setMsgs((prev) => [
            ...prev,
            {
              id: nextChatMsgId(),
              contactId: AI_PYMES_ID,
              text: t('ai.chat.action.categoryPrefix', { label: action.label }),
              fromMe: true,
              time: now,
            },
            {
              id: nextChatMsgId(),
              contactId: AI_PYMES_ID,
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
    [busy, language, sendAssistantMessage, t],
  );

  return (
    <PageLayout className="cht" title={t('ai.chat.pageTitle')} lead={t('ai.chat.pageLead')}>
      <div className="cht__layout">
        <div className="cht__main">
          <div className="cht__header cht__header-row">
            <button type="button" className="btn-secondary btn-sm" onClick={() => void clearAiThread()}>
              {t('ai.chat.newConversation')}
            </button>
          </div>
          {error ? (
            <p role="alert" className="form-error cht__form-error-chat">
              {error}
            </p>
          ) : null}
          <ChatThread
            messages={filteredThread}
            busy={busy}
            loadingHistory={loadingHistory}
            endRef={endRef}
            onAction={(action) => void handleAssistantBlockAction(action)}
            t={t}
          />
          <ChatComposer
            input={input}
            busy={busy}
            inputPrompt={inputPrompt}
            pendingConfirmations={activePendingConfirmations}
            onInputChange={setInput}
            onSend={() => void send()}
            onConfirmPending={() => void confirmPendingActions()}
            t={t}
          />
        </div>
      </div>
    </PageLayout>
  );
}
