import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  createInsightNotifications,
  createWatcher,
  getConversation,
  listConversations,
  listWatchers,
  pymesAssistantChat,
  updateWatcher,
} from './aiApi';

const apiMocks = vi.hoisted(() => ({
  apiRequest: vi.fn(),
}));

vi.mock('./api', () => ({
  apiRequest: (...args: unknown[]) => apiMocks.apiRequest(...args),
}));

describe('aiApi', () => {
  beforeEach(() => {
    apiMocks.apiRequest.mockReset();
    apiMocks.apiRequest.mockResolvedValue({});
  });

  it('routes assistant chat through Pymes backend', async () => {
    const payload = {
      chat_id: 'conversation-1',
      message: 'hola',
      route_hint: 'sales',
      preferred_language: 'es',
      confirmed_actions: ['a-1'],
    };

    await pymesAssistantChat(payload);

    expect(apiMocks.apiRequest).toHaveBeenCalledWith('/v1/ai/chat', {
      method: 'POST',
      body: payload,
    });
  });

  it('routes Companion reads through Pymes backend', async () => {
    await listConversations(30);
    await getConversation('conversation-1');
    await createInsightNotifications({ kind: 'insight', period: 'today' });

    expect(apiMocks.apiRequest).toHaveBeenNthCalledWith(1, '/v1/ai/chat/conversations?limit=30');
    expect(apiMocks.apiRequest).toHaveBeenNthCalledWith(2, '/v1/ai/chat/conversations/conversation-1');
    expect(apiMocks.apiRequest).toHaveBeenNthCalledWith(3, '/v1/ai/notifications', {
      method: 'POST',
      body: { kind: 'insight', period: 'today' },
    });
  });

  it('routes watcher management through Pymes backend', async () => {
    await listWatchers('org-1');
    await createWatcher({
      org_id: 'org-1',
      name: 'Ventas',
      watcher_type: 'sales',
      config: { threshold: 1 },
      enabled: true,
    });
    await updateWatcher('watcher-1', { threshold: 2 }, false);

    expect(apiMocks.apiRequest).toHaveBeenNthCalledWith(1, '/v1/ai/watchers?org_id=org-1');
    expect(apiMocks.apiRequest).toHaveBeenNthCalledWith(2, '/v1/ai/watchers', {
      method: 'POST',
      body: {
        org_id: 'org-1',
        name: 'Ventas',
        watcher_type: 'sales',
        config: { threshold: 1 },
        enabled: true,
      },
    });
    expect(apiMocks.apiRequest).toHaveBeenNthCalledWith(3, '/v1/ai/watchers/watcher-1', {
      method: 'PATCH',
      body: { config: { threshold: 2 }, enabled: false },
    });
  });
});
