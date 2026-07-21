function decodeBase64Url(value: string): string {
  const normalized = value.replace(/-/g, '+').replace(/_/g, '/');
  const paddingLength = (4 - (normalized.length % 4)) % 4;
  return globalThis.atob(`${normalized}${'='.repeat(paddingLength)}`);
}

function inviteTokenFromUrl(value: string, origin: string): string {
  try {
    const url = new URL(value, origin);
    return url.searchParams.get('token')?.trim() ?? '';
  } catch {
    return '';
  }
}

export function extractInviteTokenFromSearchParams(params: URLSearchParams, origin = window.location.origin): string {
  const directToken = params.get('token')?.trim() ?? '';
  if (directToken) {
    return directToken;
  }

  const ticket = params.get('__clerk_ticket')?.trim() ?? '';
  const payloadSegment = ticket.split('.')[1] ?? '';
  if (!payloadSegment) {
    return '';
  }

  try {
    const payload = JSON.parse(decodeBase64Url(payloadSegment)) as Record<string, unknown>;
    const candidates = [payload.rurl, payload.redirect_url, payload.redirectUrl, payload.url];
    for (const candidate of candidates) {
      if (typeof candidate !== 'string') {
        continue;
      }
      const token = inviteTokenFromUrl(candidate, origin);
      if (token) {
        return token;
      }
    }
  } catch {
    return '';
  }

  return '';
}
