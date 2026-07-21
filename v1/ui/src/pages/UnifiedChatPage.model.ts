import type { CommercialChatRequest, PymesAssistantChatBlock } from '../types/aiChat';

export type ManualRouteHint = Exclude<NonNullable<CommercialChatRequest['route_hint']>, 'general' | 'insight_chat'>;

export const AI_PYMES_ID = 'ai-pymes';

export type Msg = {
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

export type MsgBadgeTone =
  | 'sales'
  | 'collections'
  | 'purchases'
  | 'customers'
  | 'products'
  | 'employees'
  | 'general'
  | 'deterministic'
  | 'gemini'
  | 'neutral';

export type AssistantReplyRow = Pick<
  Msg,
  'text' | 'fromMe' | 'routedLabel' | 'blocks' | 'metaLabel' | 'badgeLabels' | 'badgeTones'
>;
