const AUTH_BASE = '/api/v1/auth';
const CHAT_BASE = '/api/v1';

function getToken(): string | null {
  return localStorage.getItem('token');
}

function authHeaders(): Record<string, string> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  return headers;
}

async function safeFetch(url: string, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(url, init);
  } catch {
    throw new Error('Could not connect to the server. Check your internet connection and try again.');
  }
}

function friendlyError(status: number, serverMsg: string): string {
  switch (status) {
    case 400: return serverMsg || 'Invalid input. Please check your details.';
    case 401: return 'Your session has expired. Please sign in again.';
    case 403: return "You don't have permission to do that.";
    case 404: return 'Not found — it may have been deleted.';
    case 409: return serverMsg || 'This already exists.';
    case 429: return 'Too many requests. Please wait a moment and try again.';
    case 500:
    case 502:
    case 503: return 'Server error — please try again in a moment.';
    default:  return serverMsg || `Something went wrong (${status}). Please try again.`;
  }
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    const serverMsg = (body as { error?: { message?: string } }).error?.message ?? '';
    throw new Error(friendlyError(res.status, serverMsg));
  }
  if (res.status === 204) return undefined as unknown as T;
  const body = await res.json();
  return (body as { data: T }).data;
}

// ── Auth ────────────────────────────────────────────────────────
export interface UserProfile {
  id: string;
  username: string;
  created_at?: string;
}

export async function register(username: string, password: string): Promise<UserProfile> {
  const res = await safeFetch(`${AUTH_BASE}/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (res.status === 409) throw new Error('This username is already taken. Please choose a different one.');
  return handleResponse<UserProfile>(res);
}

export async function login(username: string, password: string): Promise<{ token: string }> {
  const res = await safeFetch(`${AUTH_BASE}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (res.status === 401) throw new Error('Incorrect username or password. Please try again.');
  return handleResponse<{ token: string }>(res);
}

// ── Users ───────────────────────────────────────────────────────
export async function getMe(): Promise<UserProfile> {
  const res = await safeFetch(`${CHAT_BASE}/users/me`, { headers: authHeaders() });
  return handleResponse<UserProfile>(res);
}

export async function listUsers(): Promise<UserProfile[]> {
  const res = await safeFetch(`${CHAT_BASE}/users`, { headers: authHeaders() });
  return handleResponse<UserProfile[]>(res);
}

// ── Rooms ───────────────────────────────────────────────────────
export interface Room {
  id: string;
  name: string | null;
  is_dm: boolean;
  created_by: string;
  created_at: string;
}

export async function createRoom(name: string): Promise<Room> {
  const res = await safeFetch(`${CHAT_BASE}/rooms`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ name }),
  });
  return handleResponse<Room>(res);
}

export async function listRooms(): Promise<Room[]> {
  const res = await safeFetch(`${CHAT_BASE}/rooms`, { headers: authHeaders() });
  return handleResponse<Room[]>(res);
}

export async function listJoinedRooms(): Promise<Room[]> {
  const res = await safeFetch(`${CHAT_BASE}/rooms/me`, { headers: authHeaders() });
  return handleResponse<Room[]>(res);
}

export async function joinRoom(roomId: string): Promise<void> {
  const res = await safeFetch(`${CHAT_BASE}/rooms/${roomId}/join`, {
    method: 'POST',
    headers: authHeaders(),
  });
  return handleResponse<void>(res);
}

export async function leaveRoom(roomId: string): Promise<void> {
  const res = await safeFetch(`${CHAT_BASE}/rooms/${roomId}/leave`, {
    method: 'POST',
    headers: authHeaders(),
  });
  return handleResponse<void>(res);
}

// ── DMs ─────────────────────────────────────────────────────────
export interface DMRoom {
  room: Room;
  other_user: { id: string; username: string };
}

export async function createOrGetDM(userId: string): Promise<DMRoom> {
  const res = await safeFetch(`${CHAT_BASE}/dms`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ user_id: userId }),
  });
  return handleResponse<DMRoom>(res);
}

export async function listDMs(): Promise<DMRoom[]> {
  const res = await safeFetch(`${CHAT_BASE}/dms`, { headers: authHeaders() });
  return handleResponse<DMRoom[]>(res);
}

// ── Messages ────────────────────────────────────────────────────
export interface Message {
  id: string;
  room_id: string;
  sender_id: string;
  sender_username: string;
  content: string;
  created_at: string;
}

export async function listMessages(roomId: string, limit = 50, offset = 0): Promise<Message[]> {
  const res = await safeFetch(
    `${CHAT_BASE}/rooms/${roomId}/messages?limit=${limit}&offset=${offset}`,
    { headers: authHeaders() },
  );
  return handleResponse<Message[]>(res);
}
