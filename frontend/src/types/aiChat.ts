import type { components, paths } from '../generated/pymes-ai.openapi';

type Schemas = components['schemas'];

export type CommercialChatRequest =
  paths['/v1/chat']['post']['requestBody']['content']['application/json'];

export type PymesAssistantChatResponse =
  paths['/v1/chat']['post']['responses'][200]['content']['application/json'];

export type PymesAssistantChatBaseResponse = Pick<
  PymesAssistantChatResponse,
  'chat_id' | 'reply' | 'tokens_used' | 'tool_calls' | 'pending_confirmations'
>;

export type PymesAssistantAction = Schemas['ChatAction'];
export type PymesAssistantChatTextBlock = Schemas['ChatTextBlock'];
export type PymesAssistantChatActionsBlock = Schemas['ChatActionsBlock'];
export type PymesAssistantChatInsightCardBlock = Schemas['ChatInsightCardBlock'];
export type PymesAssistantChatKpiGroupBlock = Schemas['ChatKpiGroupBlock'];
export type PymesAssistantChatTableBlock = Schemas['ChatTableBlock'];
export type PymesAssistantChatBlock = NonNullable<PymesAssistantChatResponse['blocks']>[number];
export type PymesRoutedAgent = PymesAssistantChatResponse['routed_agent'];
export type PymesRoutingSource = PymesAssistantChatResponse['routing_source'];
export type PymesChatOutputKind = PymesAssistantChatResponse['output_kind'];

export type InsightNotificationItem = Schemas['NotificationItem'];
export type InsightNotificationScope = Schemas['NotificationChatContext']['scope'];
export type InsightNotificationsResponse =
  paths['/v1/notifications']['post']['responses'][200]['content']['application/json'];
export type PymesInsightServiceKind = InsightNotificationsResponse['service_kind'];
export type PymesInsightOutputKind = InsightNotificationsResponse['output_kind'];
