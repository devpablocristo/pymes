import { describe, expect, it } from 'vitest';
import { extractInviteTokenFromSearchParams } from '../lib/inviteTokens';

function clerkTicketWithPayload(payload: Record<string, unknown>): string {
  const encodedPayload = globalThis.btoa(JSON.stringify(payload))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
  return `header.${encodedPayload}.signature`;
}

describe('InviteAcceptPage token extraction', () => {
  it('uses the direct local invite token when present', () => {
    const params = new URLSearchParams('token=local-token');

    expect(extractInviteTokenFromSearchParams(params, 'http://localhost:5180')).toBe('local-token');
  });

  it('extracts the local invite token from Clerk ticket redirect payloads', () => {
    const ticket = clerkTicketWithPayload({
      rurl: 'http://localhost:5180/invite/accept?token=from-clerk-ticket',
    });
    const params = new URLSearchParams(`__clerk_status=sign_up&__clerk_ticket=${encodeURIComponent(ticket)}`);

    expect(extractInviteTokenFromSearchParams(params, 'http://localhost:5180')).toBe('from-clerk-ticket');
  });

  it('returns an empty token for malformed Clerk tickets', () => {
    const params = new URLSearchParams('__clerk_ticket=not-a-jwt');

    expect(extractInviteTokenFromSearchParams(params, 'http://localhost:5180')).toBe('');
  });
});
