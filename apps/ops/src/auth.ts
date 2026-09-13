/**
 * Staff authentication via server-side sessions with HttpOnly cookies.
 *
 * How it works:
 * - Login: POST credentials → server creates DB session → sets HttpOnly cookie.
 * - Subsequent requests: browser sends cookie automatically → server validates.
 * - Page refresh: cookie persists → GET /auth/me restores identity. No logout.
 * - CSRF: server sets a readable `bs_csrf` cookie; we send it as X-CSRF-Token header.
 * - Logout: POST → server revokes session → clears cookies.
 */

export const API_BASE = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080/api/v1';

export interface StaffClaims {
  staffId: string;
  orgId: string;
  role: string;
}

/** Read the CSRF token from the readable cookie (not HttpOnly). */
function getCSRFToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)bs_csrf=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : '';
}

/**
 * Check for an existing session on page load.
 * The HttpOnly cookie is sent automatically by the browser.
 * Returns the staff identity if a valid session exists, or null.
 */
export async function checkExistingSession(): Promise<StaffClaims | null> {
  try {
    const res = await fetch(`${API_BASE}/staff/auth/me`, {
      credentials: 'include',
      cache: 'no-store',
    });
    if (!res.ok) return null;
    const data = await res.json();
    return {
      staffId: data.staff_id,
      orgId: data.organization_id,
      role: data.role,
    };
  } catch {
    return null;
  }
}

/**
 * Login with username/password.
 * The server sets an HttpOnly session cookie in the response.
 * We don't store any token — the browser handles cookie persistence.
 */
export async function login(username: string, password: string): Promise<StaffClaims> {
  const res = await fetch(`${API_BASE}/staff/auth/login`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'Login failed');
  return {
    staffId: data.staff_id,
    orgId: data.organization_id,
    role: data.role,
  };
}

/** Logout — server revokes the session and clears cookies. */
export async function logout(): Promise<void> {
  try {
    await fetch(`${API_BASE}/staff/auth/logout`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': getCSRFToken() },
    });
  } catch {
    // Best-effort: cookie expiry is the fallback if this fails.
  }
}

/**
 * Authenticated fetch wrapper.
 * - Cookies are sent automatically (credentials: 'include').
 * - CSRF token is added as a header for state-changing methods.
 * - No Authorization header needed.
 */
export async function staffFetch(
  input: RequestInfo | URL,
  init: RequestInit = {},
): Promise<Response> {
  const headers = new Headers(init.headers);

  // Add CSRF token for state-changing requests.
  const method = (init.method ?? 'GET').toUpperCase();
  if (method !== 'GET' && method !== 'HEAD') {
    headers.set('X-CSRF-Token', getCSRFToken());
  }

  // Remove any leftover Authorization headers from old code paths.
  headers.delete('Authorization');

  return fetch(input, {
    ...init,
    headers,
    credentials: 'include',
    cache: 'no-store',
  });
}
