import { http, HttpResponse } from 'msw';

// ── Persistent in-memory state (resets on page reload — that's fine for demo) ──

const allRooms = [
  { id: 'r1', name: 'general',    is_dm: false, created_by: 'u1',   created_at: '2026-04-20T08:00:00Z' },
  { id: 'r2', name: 'design',     is_dm: false, created_by: 'u-me', created_at: '2026-04-21T10:00:00Z' },
  { id: 'r3', name: 'random',     is_dm: false, created_by: 'u2',   created_at: '2026-04-22T12:00:00Z' },
  { id: 'r4', name: 'dev-corner', is_dm: false, created_by: 'u3',   created_at: '2026-04-23T09:00:00Z' },
];

const otherUsers = [
  { id: 'u1', username: 'alice',   created_at: '2026-01-01T00:00:00Z' },
  { id: 'u2', username: 'bob',     created_at: '2026-01-02T00:00:00Z' },
  { id: 'u3', username: 'charlie', created_at: '2026-01-03T00:00:00Z' },
];

const joinedRoomIds = new Set(['r1', 'r2']);

const dmList: { room: object; other_user: { id: string; username: string } }[] = [
  {
    room: { id: 'dm1', name: null, is_dm: true, created_by: 'u-me', created_at: '2026-04-25T00:00:00Z' },
    other_user: { id: 'u1', username: 'alice' },
  },
];

interface Msg {
  id: string; room_id: string; sender_id: string;
  sender_username: string; content: string; created_at: string;
}

const messageStore: Record<string, Msg[]> = {
  r1: [
    { id: 'msg1', room_id: 'r1', sender_id: 'u1',   sender_username: 'alice',   content: 'Hey everyone! 👋',              created_at: '2026-04-27T09:00:00Z' },
    { id: 'msg2', room_id: 'r1', sender_id: 'u2',   sender_username: 'bob',     content: "What's up!",                    created_at: '2026-04-27T09:01:00Z' },
    { id: 'msg3', room_id: 'r1', sender_id: 'u-me', sender_username: 'you',     content: 'Hello everyone!',               created_at: '2026-04-27T09:02:00Z' },
    { id: 'msg4', room_id: 'r1', sender_id: 'u1',   sender_username: 'alice',   content: 'Nice to have you here 😊',      created_at: '2026-04-27T09:03:00Z' },
    { id: 'msg5', room_id: 'r1', sender_id: 'u3',   sender_username: 'charlie', content: 'Welcome!',                      created_at: '2026-04-27T09:04:00Z' },
  ],
  r2: [
    { id: 'msg6', room_id: 'r2', sender_id: 'u-me', sender_username: 'you',     content: "Here's the updated mockup",     created_at: '2026-04-26T14:00:00Z' },
    { id: 'msg7', room_id: 'r2', sender_id: 'u3',   sender_username: 'charlie', content: 'Love the color palette! 🎨',    created_at: '2026-04-26T14:05:00Z' },
    { id: 'msg8', room_id: 'r2', sender_id: 'u-me', sender_username: 'you',     content: 'Thanks! Still tweaking the typography', created_at: '2026-04-26T14:07:00Z' },
  ],
  dm1: [
    { id: 'msg9',  room_id: 'dm1', sender_id: 'u1',   sender_username: 'alice', content: "Hey, how's the project going?",     created_at: '2026-04-27T10:00:00Z' },
    { id: 'msg10', room_id: 'dm1', sender_id: 'u-me', sender_username: 'you',   content: 'Going well! Almost done with the UI', created_at: '2026-04-27T10:02:00Z' },
    { id: 'msg11', room_id: 'dm1', sender_id: 'u1',   sender_username: 'alice', content: 'Looks amazing btw 🔥',               created_at: '2026-04-27T10:03:00Z' },
  ],
};

let counter = 1000;

function ok<T>(data: T): { data: T; error: null } {
  return { data, error: null };
}

function usernameFromRequest(request: Request): string {
  const auth = request.headers.get('Authorization') ?? '';
  const token = auth.replace('Bearer ', '');
  return token.startsWith('mock:') ? token.slice(5) : 'you';
}

export const handlers = [

  // ── Auth ────────────────────────────────────────────────────
  http.post('/api/v1/auth/register', async ({ request }) => {
    const { username } = await request.json() as { username: string; password: string };
    return HttpResponse.json(
      ok({ id: 'u-me', username, created_at: new Date().toISOString() }),
      { status: 201 },
    );
  }),

  http.post('/api/v1/auth/login', async ({ request }) => {
    const { username } = await request.json() as { username: string; password: string };
    return HttpResponse.json(ok({ token: `mock:${username}` }));
  }),

  // ── Users ────────────────────────────────────────────────────
  http.get('/api/v1/users/me', ({ request }) => {
    const username = usernameFromRequest(request);
    return HttpResponse.json(ok({ id: 'u-me', username, created_at: new Date().toISOString() }));
  }),

  http.get('/api/v1/users', () => {
    return HttpResponse.json(ok(otherUsers));
  }),

  // ── Rooms ────────────────────────────────────────────────────
  http.get('/api/v1/rooms/me', () => {
    return HttpResponse.json(ok(allRooms.filter(r => joinedRoomIds.has(r.id))));
  }),

  http.get('/api/v1/rooms', () => {
    return HttpResponse.json(ok(allRooms));
  }),

  http.post('/api/v1/rooms', async ({ request }) => {
    const { name } = await request.json() as { name: string };
    const room = {
      id: `r-${++counter}`,
      name,
      is_dm: false,
      created_by: 'u-me',
      created_at: new Date().toISOString(),
    };
    allRooms.push(room);
    messageStore[room.id] = [];
    return HttpResponse.json(ok(room), { status: 201 });
  }),

  http.post('/api/v1/rooms/:id/join', ({ params }) => {
    joinedRoomIds.add(params.id as string);
    return new HttpResponse(null, { status: 204 });
  }),

  http.post('/api/v1/rooms/:id/leave', ({ params }) => {
    joinedRoomIds.delete(params.id as string);
    return new HttpResponse(null, { status: 204 });
  }),

  // ── DMs ──────────────────────────────────────────────────────
  http.get('/api/v1/dms', () => {
    return HttpResponse.json(ok(dmList));
  }),

  http.post('/api/v1/dms', async ({ request }) => {
    const { user_id } = await request.json() as { user_id: string };
    const existing = dmList.find(d => d.other_user.id === user_id);
    if (existing) return HttpResponse.json(ok(existing));

    const user = otherUsers.find(u => u.id === user_id);
    if (!user) return HttpResponse.json(
      { data: null, error: { code: 404, message: 'User not found' } },
      { status: 404 },
    );

    const dmId = `dm-${++counter}`;
    const entry = {
      room: { id: dmId, name: null, is_dm: true, created_by: 'u-me', created_at: new Date().toISOString() },
      other_user: { id: user.id, username: user.username },
    };
    dmList.push(entry);
    messageStore[dmId] = [];
    return HttpResponse.json(ok(entry));
  }),

  // ── Messages ─────────────────────────────────────────────────
  http.get('/api/v1/rooms/:id/messages', ({ params }) => {
    const msgs = messageStore[params.id as string] ?? [];
    return HttpResponse.json(ok([...msgs].reverse())); // newest-first like the real API
  }),
];
