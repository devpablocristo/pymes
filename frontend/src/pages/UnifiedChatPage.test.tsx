import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { PageSearchProvider } from '../components/PageSearch';
import { LanguageProvider } from '../lib/i18n';
import { NOTIFICATION_CHAT_HANDOFF_KEY } from '../lib/notificationChatHandoff';
import { UnifiedChatPage } from './UnifiedChatPage';
import type { ConversationDetail, ConversationSummary } from '../lib/aiApi';
import type { PymesAssistantChatResponse } from '../types/aiChat';

const aiMocks = vi.hoisted(() => ({
  listConversations: vi.fn<[number?], Promise<{ items: ConversationSummary[] }>>(),
  getConversation: vi.fn<[string], Promise<ConversationDetail>>(),
  pymesAssistantChat: vi.fn<[], Promise<PymesAssistantChatResponse>>(),
}));

vi.mock('../lib/aiApi', () => ({
  listConversations: (...args: [number?]) => aiMocks.listConversations(...args),
  getConversation: (...args: [string]) => aiMocks.getConversation(...args),
  pymesAssistantChat: (...args: []) => aiMocks.pymesAssistantChat(...args),
}));

function buildChatReply(overrides?: Partial<PymesAssistantChatResponse>): PymesAssistantChatResponse {
  return {
    chat_id: 'conv-3',
    reply: 'Respuesta del asistente',
    request_id: 'req-1',
    routed_agent: 'sales',
    routing_source: 'llm',
    output_kind: 'chat',
    tokens_used: 10,
    tool_calls: [],
    pending_confirmations: [],
    blocks: [],
    ...overrides,
  } as PymesAssistantChatResponse;
}

function renderUnifiedChat() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <LanguageProvider initialLanguage="es">
          <PageSearchProvider>
            <UnifiedChatPage />
          </PageSearchProvider>
        </LanguageProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { queryClient };
}

function getMessagesPane(): HTMLElement {
  const pane = document.querySelector('.cht__messages');
  if (!(pane instanceof HTMLElement)) {
    throw new Error('No se encontró el panel de mensajes');
  }
  return pane;
}


describe('UnifiedChatPage', () => {
  beforeEach(() => {
    aiMocks.listConversations.mockReset();
    aiMocks.getConversation.mockReset();
    aiMocks.pymesAssistantChat.mockReset();
    window.sessionStorage.clear();
    window.HTMLElement.prototype.scrollIntoView = vi.fn();

    aiMocks.listConversations.mockResolvedValue({
      items: [
        {
          id: 'conv-1',
          title: 'Conversación A',
          created_at: '2026-04-02T12:00:00Z',
          updated_at: '2026-04-02T12:00:00Z',
          message_count: 2,
        },
        {
          id: 'conv-2',
          title: 'Conversación B',
          created_at: '2026-04-01T12:00:00Z',
          updated_at: '2026-04-01T12:00:00Z',
          message_count: 1,
        },
      ],
    });
    aiMocks.getConversation.mockImplementation(async (conversationId: string) => ({
      id: conversationId,
      title: conversationId === 'conv-1' ? 'Conversación A' : 'Conversación B',
      created_at: '2026-04-02T12:00:00Z',
      updated_at: '2026-04-02T12:00:00Z',
      messages: [
        {
          role: 'assistant',
          content: conversationId === 'conv-1' ? 'Historial A' : 'Historial B',
          ts: '2026-04-02T12:00:00Z',
          tool_calls: [],
        },
      ],
    }));
    aiMocks.pymesAssistantChat.mockResolvedValue(buildChatReply());
  });

  it('hidrata el último historial del asistente automáticamente al abrir el chat', async () => {
    renderUnifiedChat();

    expect(screen.getByRole('textbox', { name: /Ej\.: resumí ventas del mes o preguntá libre/i })).toBeInTheDocument();
    expect(getMessagesPane()).toHaveAttribute('role', 'log');

    await waitFor(() => {
      expect(aiMocks.listConversations).toHaveBeenCalledWith(30);
      expect(aiMocks.getConversation).toHaveBeenCalledWith('conv-1');
    });
    expect(await within(getMessagesPane()).findByText('Historial A')).toBeInTheDocument();
  });

  it('limpia el hilo del asistente al iniciar una nueva conversación', async () => {
    renderUnifiedChat();

    expect(await within(getMessagesPane()).findByText('Historial A')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Nueva conversación' }));

    await waitFor(() => {
      expect(within(getMessagesPane()).queryByText('Historial A')).not.toBeInTheDocument();
    });
    // No queda ningún botón de conversación previa en la sidebar (vista estilo WhatsApp).
    expect(screen.queryByText('Conversación A')).not.toBeInTheDocument();
    expect(screen.queryByText('Conversación B')).not.toBeInTheDocument();
  });

  it('prioriza el handoff desde notificaciones sobre la rehidratación automática del último historial', async () => {
    window.sessionStorage.setItem(
      NOTIFICATION_CHAT_HANDOFF_KEY,
      JSON.stringify({
        notificationId: 'notif-1',
        title: 'Cobro pendiente',
        body: 'Hay un cobro para revisar.',
        chatContext: {
          suggested_user_message: 'Explicame este cobro pendiente',
        },
        routedAgent: 'sales',
        contentLanguage: 'es',
      }),
    );

    renderUnifiedChat();

    await waitFor(() => {
      expect(aiMocks.listConversations).toHaveBeenCalledWith(30);
      expect(aiMocks.pymesAssistantChat).toHaveBeenCalled();
    });

    const chatCalls = aiMocks.pymesAssistantChat.mock.calls as unknown[][];
    expect(chatCalls[0]?.[0]).toEqual(
      expect.objectContaining({
        message: 'Explicame este cobro pendiente',
        chat_id: null,
        preferred_language: 'es',
      }),
    );
    expect(aiMocks.getConversation).not.toHaveBeenCalled();
    expect(await within(getMessagesPane()).findByText('Explicame este cobro pendiente')).toBeInTheDocument();
    expect(await within(getMessagesPane()).findByText('Respuesta del asistente')).toBeInTheDocument();
    expect(within(getMessagesPane()).queryByText('Historial A')).not.toBeInTheDocument();
  });
});
