import type {
  CommercialChatRequest,
  PymesAssistantAction,
  PymesAssistantChatBlock,
  PymesAssistantChatResponse,
} from '../types/aiChat';
import { humanInsightScopeLabel, humanRoutedLabel, humanRoutingSourceLabel } from '../lib/aiLabels';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import type { LanguageCode } from '../lib/i18n';
import type { NotificationChatHandoff } from '../lib/notificationChatHandoff';
import type { AssistantReplyRow, ManualRouteHint, Msg, MsgBadgeTone } from './UnifiedChatPage.model';

let nextMsgId = 100;

export function nextChatMsgId(): string {
  return String(++nextMsgId);
}

export function normalizeManualRouteHint(value: string | null | undefined): ManualRouteHint | undefined {
  if (
    value === 'customers' ||
    value === 'products' ||
    value === 'services' ||
    value === 'sales' ||
    value === 'collections' ||
    value === 'purchases' ||
    value === 'employees'
  ) {
    return value;
  }
  return undefined;
}

export function hasPromptForQueryBlock(blocks: PymesAssistantChatBlock[] | undefined): boolean {
  return Boolean(
    blocks?.some(
      (block) =>
        block.type === 'actions' &&
        (block.actions ?? []).some((action) => action.selection_behavior === 'prompt_for_query'),
    ),
  );
}

export function buttonClassName(style?: PymesAssistantAction['style']): string {
  if (style === 'primary') return 'btn-primary btn-sm';
  if (style === 'ghost') return 'cht__block-action cht__block-action--ghost';
  return 'btn-secondary btn-sm';
}

export function kpiTrendClassName(trend?: 'up' | 'down' | 'flat' | 'unknown' | null): string {
  if (trend === 'up') return 'cht__kpi-item-trend cht__kpi-item-trend--up';
  if (trend === 'down') return 'cht__kpi-item-trend cht__kpi-item-trend--down';
  if (trend === 'flat') return 'cht__kpi-item-trend cht__kpi-item-trend--flat';
  return 'cht__kpi-item-trend';
}

export function badgeToneForRoute(mode: string | null | undefined): MsgBadgeTone {
  if (mode === 'sales' || mode === 'internal_sales') return 'sales';
  if (mode === 'collections') return 'collections';
  if (mode === 'purchases' || mode === 'internal_procurement') return 'purchases';
  if (mode === 'customers') return 'customers';
  if (mode === 'products' || mode === 'services') return 'products';
  if (mode === 'employees') return 'employees';
  if (mode === 'general' || mode === 'insight_chat') return 'general';
  return 'neutral';
}

export function badgeClassName(tone: MsgBadgeTone): string {
  return `cht__msg-badge cht__msg-badge--${tone}`;
}

export function localeForLanguage(language: LanguageCode): string {
  return language === 'en' ? 'en-US' : 'es-AR';
}

export function formatChatTime(language: LanguageCode): string {
  return new Date().toLocaleTimeString(localeForLanguage(language), { hour: '2-digit', minute: '2-digit' });
}

export function formatIsoTime(iso: string | null | undefined, language: LanguageCode): string {
  if (!iso) return '';
  try {
    return new Date(iso).toLocaleTimeString(localeForLanguage(language), { hour: '2-digit', minute: '2-digit' });
  } catch {
    return '';
  }
}

export function resolvePreferredLanguage(
  contentLanguage: string | undefined,
  defaultLanguage: LanguageCode,
): LanguageCode {
  if (contentLanguage === 'en' || contentLanguage === 'es') {
    return contentLanguage;
  }
  return defaultLanguage;
}

export function humanBadgeCategoryLabel(mode: string, language: LanguageCode): string {
  if (mode === 'customers') return humanRoutedLabel('customers', language);
  if (mode === 'products') return humanRoutedLabel('products', language);
  if (mode === 'sales') return humanRoutedLabel('sales', language);
  if (mode === 'collections') return humanRoutedLabel('collections', language);
  if (mode === 'purchases') return humanRoutedLabel('purchases', language);
  if (mode === 'employees') return humanRoutedLabel('employees', language);
  return humanRoutedLabel('general', language);
}

export function isDeterministicReply(reply: PymesAssistantChatResponse): boolean {
  return reply.answer_mode === 'facts_only' || Boolean(reply.deterministic?.used && !reply.llm?.used);
}

export function buildAssistantMetaLabel(
  reply: PymesAssistantChatResponse,
  language: LanguageCode,
  t: (key: string, variables?: Record<string, string | number>) => string,
): string {
  const parts = [
    `${t('ai.chat.meta.request')} ${reply.request_id}`,
    reply.output_kind,
    humanRoutedLabel(reply.routed_agent, language),
    humanRoutingSourceLabel(reply.routing_source, language),
  ];
  if (reply.analysis_scope) {
    parts.push(humanInsightScopeLabel(reply.analysis_scope, language));
  }
  return parts.join(' · ');
}

export function formatAssistantHttpError(err: unknown, defaultMessage: string): string {
  const body =
    typeof (err as { body?: unknown } | null)?.body === 'string' ? String((err as { body?: string }).body) : '';
  if (body) {
    try {
      const parsed = JSON.parse(body) as { error?: { code?: string; message?: string } };
      if (parsed.error?.code === 'gemini_unavailable' && parsed.error.message) {
        return parsed.error.message;
      }
      if (parsed.error?.message) {
        return parsed.error.message;
      }
    } catch {
      // formatFetchErrorForUser handles the original error message below.
    }
  }
  return formatFetchErrorForUser(err, defaultMessage);
}

export function buildNotificationHandoffMetaLabel(
  handoff: NotificationChatHandoff,
  language: LanguageCode,
  t: (key: string, variables?: Record<string, string | number>) => string,
): string | null {
  const parts = [`${t('ai.chat.meta.notification')} ${handoff.notificationId}`];
  const routedAgent = handoff.routedAgent;
  const showAgentLabel = Boolean(routedAgent) && !(routedAgent === 'insight_chat' && handoff.scope);
  if (showAgentLabel && routedAgent) {
    parts.push(`${t('ai.chat.meta.agent')} ${humanRoutedLabel(routedAgent, language)}`);
  }
  if (handoff.scope) {
    parts.push(`${t('ai.chat.meta.context')} ${humanInsightScopeLabel(handoff.scope, language)}`);
  }
  return parts.length > 0 ? parts.join(' · ') : null;
}

export function buildRouteHintMetaLabel(
  routeHint: CommercialChatRequest['route_hint'],
  language: LanguageCode,
  t: (key: string, variables?: Record<string, string | number>) => string,
): string | undefined {
  if (!routeHint) return undefined;
  return `${t('ai.chat.meta.manualRoute')} · ${humanRoutedLabel(routeHint, language)}`;
}

export function buildAssistantBadgeLabels(reply: PymesAssistantChatResponse, language: LanguageCode): string[] {
  const labels = [humanBadgeCategoryLabel(reply.routed_agent, language)];
  if (isDeterministicReply(reply)) {
    labels.push(language === 'en' ? 'Deterministic' : 'Determinista');
  }
  if (reply.llm?.used && reply.llm.model) {
    labels.push(`Gemini ${reply.llm.model}`);
  }
  const toolCount = reply.evidence?.tools?.length ?? reply.tool_calls?.length ?? 0;
  if (toolCount > 0) {
    labels.push(language === 'en' ? `${toolCount} evidence tools` : `${toolCount} herramientas`);
  }
  const period = reply.evidence?.period;
  if (period?.label) {
    labels.push(language === 'en' ? `Period ${period.label}` : `Período ${period.label}`);
  }
  return labels;
}

export function buildAssistantBadgeTones(reply: PymesAssistantChatResponse): MsgBadgeTone[] {
  const tones: MsgBadgeTone[] = [badgeToneForRoute(reply.routed_agent)];
  if (isDeterministicReply(reply)) {
    tones.push('deterministic');
  }
  if (reply.llm?.used && reply.llm.model) {
    tones.push('gemini');
  }
  const toolCount = reply.evidence?.tools?.length ?? reply.tool_calls?.length ?? 0;
  if (toolCount > 0) {
    tones.push('neutral');
  }
  if (reply.evidence?.period?.label) {
    tones.push('neutral');
  }
  return tones;
}

export function applyPymesReply(
  reply: PymesAssistantChatResponse,
  language: LanguageCode,
  t: (key: string, variables?: Record<string, string | number>) => string,
): AssistantReplyRow[] {
  const agentLabel = humanRoutedLabel(reply.routed_agent, language);
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

export function materializeAssistantReplyRows(
  contactId: string,
  rows: AssistantReplyRow[],
  language: LanguageCode,
): Msg[] {
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

export function buildAssistantMessages(
  contactId: string,
  reply: PymesAssistantChatResponse,
  language: LanguageCode,
  t: (key: string, variables?: Record<string, string | number>) => string,
): Msg[] {
  return materializeAssistantReplyRows(contactId, applyPymesReply(reply, language, t), language);
}

export function resolveInputPrompt(
  routeHint: ManualRouteHint | undefined,
  language: LanguageCode,
  t: (key: string, variables?: Record<string, string | number>) => string,
): string {
  if (routeHint) {
    return t('ai.chat.input.routePlaceholder', {
      label: humanRoutedLabel(routeHint, language),
    });
  }
  return t('ai.chat.input.defaultPlaceholder');
}
