// Tipos del chat de Pymes mapeados al contrato canónico de Companion.
//
// Origen autoritativo: `axis/companion/openapi.yaml`.
// Los tipos se generan en `../generated/companion.openapi.ts`. Acá los
// re-exportamos con los nombres heredados (CommercialChatRequest,
// PymesAssistantChatResponse, etc.) para no romper consumidores existentes
// (`UnifiedChatPage.tsx`, etc.).
//
// El contrato v0.1 de Companion solo emite bloques `text`. Las variantes
// históricas no-text (actions/insight_card/kpi_group/table) quedan como
// tipos opcionales/estructurales para que el código de render existente
// compile, pero hoy no se reciben del backend. Se irán implementando
// incrementalmente en el contrato canónico (sin breaking change: la unión
// `ChatBlock` admite nuevas variantes).
import type { components, paths } from '../generated/companion.openapi';

type Schemas = components['schemas'];

// ── Tipos canónicos directos ─────────────────────────────────────
// El request canónico de Companion. Lo extendemos con `chat_id` aceptando
// null (el FE legacy manda null cuando arranca conversación nueva) y con
// `preferred_language` que no está en el contrato pero el FE espera.
type CompanionChatRequest =
  paths['/v1/chat']['post']['requestBody']['content']['application/json'];

export interface CommercialChatRequest extends Omit<CompanionChatRequest, 'chat_id' | 'route_hint'> {
  chat_id?: string | null;
  route_hint?: string | null;
  preferred_language?: 'es' | 'en' | string;
}

// ChatHandoff de Companion + extras del FE legacy (notification_id, etc.)
type CanonChatHandoff = Schemas['ChatHandoff'];
export interface PymesChatHandoff extends CanonChatHandoff {
  notification_id?: string;
  source_id?: string;
  insight_scope?: string;
  period?: 'today' | 'week' | 'month' | string;
  compare?: boolean;
  top_limit?: number;
  context?: unknown;
}
export type PymesChatHandoffSource = PymesChatHandoff['source'];

// El wire response de Companion (`paths['/v1/chat']['post']...`) usa
// `ChatBlock` narrow (solo `text`). Acá lo extendemos para que `blocks`
// admita la unión legacy completa (text + actions + insight + kpi + table).
// Los campos opcionales del wire son `string | undefined`; los relajamos a
// `string | null | undefined` porque la UI heredada los usaba con `null`.
type CompanionChatResponse =
  paths['/v1/chat']['post']['responses'][200]['content']['application/json'];

export interface PymesAssistantChatResponse extends Omit<CompanionChatResponse, 'blocks' | 'pending_confirmations' | 'routed_agent' | 'routing_source' | 'output_kind'> {
  // Companion canon: estos tres podrían omitirse, pero el FE los usa como
  // strings de display (con `''` cuando no hay valor); en la práctica
  // Companion los manda siempre como strings, así que los exponemos como
  // required string para no obligar al FE a coerce en cada uso.
  routed_agent: string;
  routing_source: string;
  output_kind: string;
  blocks?: PymesAssistantChatBlock[];
  pending_confirmations?: PymesChatPendingConfirmation[];
  // Campos extras que pymes-ai exponía y el FE consume opcionalmente. No están
  // en el contrato canónico de Companion v0.1; viven acá como opcionales con
  // shape laxo hasta que el contrato los absorba o el FE los elimine.
  // LEGACY-CHAT-EXTRAS.
  llm?: Record<string, any> & { used?: boolean; provider?: unknown; model?: unknown };
  evidence?: Record<string, any>;
  answer_mode?: string;
  /** `deterministic` históricamente era un objeto `{used, summary, blocks}`
   *  en pymes-ai. El FE acceso a `.used` y similares; lo tipamos laxo. */
  deterministic?: { used?: boolean; summary?: string; blocks?: PymesAssistantChatBlock[]; [k: string]: any };
  request_id?: string;
  analysis_scope?: string;
}

export type PymesAssistantChatBaseResponse = Pick<
  PymesAssistantChatResponse,
  'chat_id' | 'reply' | 'tokens_used' | 'tool_calls' | 'pending_confirmations'
>;

// `PymesAssistantChatBlock` es la unión completa de variantes de bloque que
// el FE soporta renderizar. Companion v0.1 solo emite la variante `text`;
// las demás variantes (LEGACY-BLOCK-TYPES) están declaradas para que el
// código de render existente siga compilando hasta que el backend las
// emita o las eliminemos del FE.
export type PymesAssistantChatBlock =
  | PymesAssistantChatTextBlock
  | PymesAssistantChatActionsBlock
  | PymesAssistantChatInsightCardBlock
  | PymesAssistantChatKpiGroupBlock
  | PymesAssistantChatTableBlock;
export type PymesRoutedAgent = PymesAssistantChatResponse['routed_agent'];
export type PymesRoutingSource = PymesAssistantChatResponse['routing_source'];
export type PymesChatOutputKind = PymesAssistantChatResponse['output_kind'];

export type InsightNotificationItem = Schemas['NotificationItem'];
export type PymesChatPendingConfirmation = Schemas['ChatPendingConfirmation'];
export type InsightNotificationsResponse =
  paths['/v1/notifications']['post']['responses'][200]['content']['application/json'];
export type PymesInsightServiceKind = InsightNotificationsResponse['service_kind'];
export type PymesInsightOutputKind = InsightNotificationsResponse['output_kind'];

// ── Variantes de bloque y acción ─────────────────────────────────
//
// Companion v0.1 expone `ChatBlock` como objeto con `type` + `text?`. Las
// variantes históricas (actions, insight_card, kpi_group, table) no están
// presentes en el contrato actual. Las modelamos acá como structural types
// **opcionales** para mantener el código de render compilando sin tener
// que stub-ear cada componente UI. En cuanto Companion emita estas
// variantes, los tipos se mudarán al contrato canónico y este shim se
// elimina (búsquese `LEGACY-BLOCK-TYPES`).

/** Acción clickable embebida en un ChatActionsBlock. LEGACY-BLOCK-TYPES. */
export interface PymesAssistantAction {
  id: string;
  label: string;
  kind?: string;
  style?: 'primary' | 'secondary' | 'ghost' | string;
  url?: string;
  route_hint?: string;
  message?: string;
  selection_behavior?: 'single' | 'multi' | string;
  confirmed_actions?: string[];
  payload?: unknown;
  binding_hash?: string;
}

/** Bloque de texto plano (variante `text` del contrato canónico).
 *  En el wire `text` es opcional, pero para que el render por defecto compile
 *  lo tratamos como required en este shim (vacío "" si no hay contenido).
 */
export interface PymesAssistantChatTextBlock {
  type: 'text';
  text: string;
}

/** Bloque con acciones aprobables. LEGACY-BLOCK-TYPES. */
export interface PymesAssistantChatActionsBlock {
  type: 'actions';
  actions: PymesAssistantAction[];
  title?: string;
}

/** Insight cards (KPI + narrativa). LEGACY-BLOCK-TYPES. */
export interface PymesAssistantChatInsightCardBlock {
  type: 'insight_card';
  title?: string;
  scope?: string;
  summary?: string;
  highlights?: Array<{ label?: string | number; value?: string | number }>;
  recommendations?: string[];
  context?: unknown;
}

/** Grupo de KPIs. LEGACY-BLOCK-TYPES. */
export interface PymesAssistantChatKpiGroupBlock {
  type: 'kpi_group';
  title?: string;
  items?: Array<{
    key?: string;
    label?: string;
    value?: string | number;
    trend?: 'up' | 'down' | 'flat' | 'unknown' | null;
    context?: string;
  }>;
}

/** Tabla genérica. LEGACY-BLOCK-TYPES. */
export interface PymesAssistantChatTableBlock {
  type: 'table';
  title?: string;
  columns?: string[];
  rows?: Array<Array<string | number | null | undefined>>;
  empty_state?: string;
}

/** Scope textual de una notificación insight (compat con shape pymes-ai
 * legacy). En el contrato canónico está como `NotificationItem.scope: string`. */
export type InsightNotificationScope = string;
