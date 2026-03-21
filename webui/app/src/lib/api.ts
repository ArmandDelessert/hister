export interface AppConfig {
  wsUrl: string;
  searchUrl: string;
  openResultsOnNewTab: boolean;
  hotkeys: Record<string, string>;
  authMode: 'token' | 'user' | 'none';
  username?: string;
}

let _config: AppConfig | null = null;
let _csrf: string = '';

export function getCsrf(): string {
  return _csrf;
}

export function setCsrf(tok: string): void {
  _csrf = tok;
}

export function getAuthMode(): string {
  return _config?.authMode ?? 'none';
}

export function getUsername(): string {
  return _config?.username ?? '';
}

export function resetConfig(): void {
  _config = null;
}

export async function fetchConfig(): Promise<AppConfig> {
  if (_config) return _config;
  const headers: Record<string, string> = {};
  const token = localStorage.getItem('access-token');
  if (token) {
    headers['X-Access-Token'] = token;
  }
  const res = await fetch('api/config', { headers, credentials: 'include' });
  if (res.status === 403) {
    window.location.href = '/auth';
    throw new Error('Authentication required');
  }
  const tok = res.headers.get('X-CSRF-Token');
  if (tok) _csrf = tok;
  _config = await res.json();
  return _config!;
}

export async function login(username: string, password: string): Promise<{ username: string }> {
  const res = await fetch('api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    throw new Error('Invalid credentials');
  }
  _config = null;
  return res.json();
}

export async function logout(): Promise<void> {
  await apiFetch('/logout', { method: 'POST' });
  _config = null;
}

export async function apiFetch(url: string, options: RequestInit = {}): Promise<Response> {
  const headers: Record<string, string> = {
    ...(options.headers as Record<string, string>),
  };
  if (_csrf && options.method && options.method.toUpperCase() !== 'GET') {
    headers['X-CSRF-Token'] = _csrf;
  }
  const token = localStorage.getItem('access-token');
  if (token) {
    headers['X-Access-Token'] = token;
  }
  const res = await fetch('api' + url, { ...options, headers, credentials: 'include' });
  if (res.status === 403) {
    window.location.href = '/auth';
    throw new Error('Authentication required');
  }
  const newTok = res.headers.get('X-CSRF-Token');
  if (newTok) _csrf = newTok;
  return res;
}
