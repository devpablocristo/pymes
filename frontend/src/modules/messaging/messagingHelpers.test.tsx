import { describe, expect, it } from 'vitest';
import {
  buildMessagingCampaignsSummary,
  buildMessagingInboxSummary,
  formatMessagingConversationTimestamp,
} from './messagingHelpers';

describe('messagingHelpers', () => {
  it('builds inbox summary', () => {
    expect(
      buildMessagingInboxSummary(
        [
          { unread_count: 2, status: 'open', assigned_to: 'u1' },
          { unread_count: 0, status: 'resolved', assigned_to: '' },
        ] as never,
        2,
      ),
    ).toBe('2 visibles · 1 abiertas · 2 sin leer · 1 asignadas');
  });

  it('builds campaigns summary', () => {
    expect(
      buildMessagingCampaignsSummary(
        [
          { status: 'draft' },
          { status: 'completed' },
          { status: 'completed' },
        ] as never,
        3,
      ),
    ).toBe('3 visibles · 1 draft · 2 completadas');
  });

  it('formats conversation timestamp fallback', () => {
    expect(formatMessagingConversationTimestamp()).toBe('Sin actividad todavía');
  });
});
