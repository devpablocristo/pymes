import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { PageSearchProvider } from '../components/PageSearch';
import { LanguageProvider } from '../lib/i18n';
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
    routed_agent: 'ventas',
    routed_mode: 'ventas',
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

function getConversationButton(title: string): HTMLButtonElement {
  const label = screen.getByText(title);
  const button = label.closest('button');
  if (!(button instanceof HTMLButtonElement)) {
    throw new Error(`No se encontró el botón de conversación para ${title}`);
  }
  return button;
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

  it('mantiene limpia la nueva conversación sin rehidratar el último historial', async () => {
    renderUnifiedChat();

    expect(screen.getByRole('textbox', { name: /Ej\.: resumí ventas del mes o preguntá libre/i })).toBeInTheDocument();
    expect(getMessagesPane()).toHaveAttribute('role', 'log');

    await waitFor(() => {
      expect(aiMocks.listConversations).toHaveBeenCalledWith(30);
      expect(aiMocks.getConversation).toHaveBeenCalledWith('conv-1');
    });
    expect(await within(getMessagesPane()).findByText('Historial A')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Nueva conversación' }));

    await waitFor(() => {
      expect(within(getMessagesPane()).queryByText('Historial A')).not.toBeInTheDocument();
    });
    expect(getConversationButton('Conversación A')).toBeInTheDocument();
  });

  it('reemplaza el hilo AI al cambiar a otra conversación guardada', async () => {
    renderUnifiedChat();

    expect(await within(getMessagesPane()).findByText('Historial A')).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText(/Ej\.: resumí ventas del mes/i), {
      target: { value: 'Consulta nueva' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Enviar' }));

    expect(await within(getMessagesPane()).findByText('Consulta nueva')).toBeInTheDocument();
    expect(await within(getMessagesPane()).findByText('Respuesta del asistente')).toBeInTheDocument();

    fireEvent.click(getConversationButton('Conversación B'));

    expect(await within(getMessagesPane()).findByText('Historial B')).toBeInTheDocument();
    expect(within(getMessagesPane()).queryByText('Consulta nueva')).not.toBeInTheDocument();
    expect(within(getMessagesPane()).queryByText('Respuesta del asistente')).not.toBeInTheDocument();
    expect(within(getMessagesPane()).queryByText('Historial A')).not.toBeInTheDocument();
  });
});
