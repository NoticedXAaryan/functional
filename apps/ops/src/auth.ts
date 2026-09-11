import Keycloak from 'keycloak-js';

export const API_BASE = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080/api/v1';

export interface StaffClaims {
  token: string;
  staffId: string;
  orgId: string;
  role: string;
}

export interface StaffAuth {
  mode: 'demo' | 'oidc';
  client?: Keycloak;
  staff: StaffClaims | null;
}

// The official SDK owns OIDC redirects, PKCE, token refresh and logout.
// The singleton also avoids initializing the adapter twice under React StrictMode.
let initialization: Promise<StaffAuth> | undefined;
let identityClient: Keycloak | undefined;
const redirectUri = window.location.origin + window.location.pathname;

export function initializeStaffAuth(): Promise<StaffAuth> {
  initialization ??= initialize();
  return initialization;
}

async function initialize(): Promise<StaffAuth> {
  // Remove credentials persisted by older builds. Tokens now live only in memory.
  sessionStorage.removeItem('staff_token');
  const response = await fetch(`${API_BASE}/staff/auth/config`, { cache: 'no-store' });
  if (!response.ok) throw new Error('Sign-in configuration is unavailable. Reload to try again.');
  const config = await response.json();
  if (config.mode === 'demo') return { mode: 'demo', staff: null };
  if (config.mode !== 'oidc') throw new Error('Organization sign-in is not configured.');

  const issuer = new URL(config.issuer);
  const realmPath = issuer.pathname.match(/^(.*)\/realms\/([^/]+)$/);
  if (!realmPath || !config.client_id) throw new Error('The Keycloak issuer configuration is invalid.');
  const client = new Keycloak({
    url: issuer.origin + realmPath[1],
    realm: decodeURIComponent(realmPath[2]),
    clientId: config.client_id,
  });
  identityClient = client;
  const authenticated = await client.init({
    onLoad: 'check-sso',
    pkceMethod: 'S256',
    flow: 'standard',
    checkLoginIframe: false,
    redirectUri,
  });
  if (!authenticated || !client.token) return { mode: 'oidc', client, staff: null };

  const meResponse = await fetch(`${API_BASE}/staff/auth/me`, {
    cache: 'no-store',
    headers: { Authorization: `Bearer ${client.token}` },
  });
  if (!meResponse.ok) {
    // A provider account alone never provisions staff permissions.
    return { mode: 'oidc', client, staff: null };
  }
  const me = await meResponse.json();
  return {
    mode: 'oidc', client,
    staff: { token: client.token, staffId: me.staff_id, orgId: me.organization_id, role: me.role },
  };
}

export async function signInWithOrganization(): Promise<void> {
  if (!identityClient) throw new Error('Organization sign-in is unavailable.');
  await identityClient.login({ redirectUri, prompt: 'login' });
}

export async function signOutOfOrganization(): Promise<void> {
  if (identityClient) await identityClient.logout({ redirectUri });
}

export async function staffFetch(input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers);
  if (identityClient && headers.has('Authorization')) {
    try {
      await identityClient.updateToken(30);
    } catch {
      identityClient.clearToken();
      throw new Error('Your session has ended. Sign out and sign in again.');
    }
    if (!identityClient.token) throw new Error('Your session has ended. Sign out and sign in again.');
    headers.set('Authorization', `Bearer ${identityClient.token}`);
  }
  return fetch(input, { ...init, headers, cache: 'no-store' });
}
