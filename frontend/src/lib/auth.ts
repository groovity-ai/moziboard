export function getSessionTokenFromCookie(): string {
  if (typeof document === 'undefined') return '';
  const match = document.cookie
    .split('; ')
    .find((row) => row.startsWith('session_id='));
  if (!match) return '';
  return decodeURIComponent(match.split('=').slice(1).join('='));
}

export function buildAuthHeaders(init?: HeadersInit): Headers {
  const headers = new Headers(init || {});
  const token = getSessionTokenFromCookie();
  if (token && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  return headers;
}
