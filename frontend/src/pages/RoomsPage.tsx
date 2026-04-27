import { useEffect, useState, type FormEvent } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';
import {
  listRooms, listJoinedRooms, createRoom, joinRoom, leaveRoom,
  type Room,
} from '../api/client';
import { useToast } from '../contexts/ToastContext';
import type { SidebarContext } from '../components/Layout';

export default function RoomsPage() {
  const { refreshSidebar } = useOutletContext<SidebarContext>();
  const navigate = useNavigate();
  const { show: toast } = useToast();

  const [publicRooms, setPublicRooms] = useState<Room[]>([]);
  const [joinedIds, setJoinedIds] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState('');
  const [newRoomName, setNewRoomName] = useState('');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');

  const fetchData = async () => {
    setLoading(true);
    setFetchError('');
    try {
      const [all, joined] = await Promise.all([listRooms(), listJoinedRooms()]);
      setPublicRooms(all);
      setJoinedIds(new Set(joined.map(r => r.id)));
    } catch (err) {
      setFetchError(err instanceof Error ? err.message : 'Failed to load rooms');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchData(); }, []);

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    setCreateError('');
    const name = newRoomName.trim();
    if (!name) return;
    setCreating(true);
    try {
      const room = await createRoom(name);
      setNewRoomName('');
      await joinRoom(room.id);
      refreshSidebar();
      navigate(`/rooms/${room.id}`, { state: { roomName: room.name } });
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'Failed to create room');
    } finally {
      setCreating(false);
    }
  };

  const handleJoin = async (room: Room) => {
    try {
      await joinRoom(room.id);
      setJoinedIds(prev => new Set([...prev, room.id]));
      refreshSidebar();
      navigate(`/rooms/${room.id}`, { state: { roomName: room.name } });
    } catch (err) {
      toast(
        err instanceof Error ? err.message : 'Failed to join room',
        'error',
        'Try refreshing the page if the room no longer appears.',
      );
    }
  };

  const handleLeave = async (room: Room) => {
    try {
      await leaveRoom(room.id);
      setJoinedIds(prev => { const s = new Set(prev); s.delete(room.id); return s; });
      refreshSidebar();
    } catch (err) {
      toast(
        err instanceof Error ? err.message : 'Failed to leave room',
        'error',
        'Please try again, or refresh the page.',
      );
    }
  };

  return (
    <>
      <div className="page-header">
        <h2># Rooms</h2>
      </div>

      <div className="page-body">
        {/* Create room */}
        <div className="section-title">Create a room</div>
        <form className="inline-form" onSubmit={handleCreate}>
          <input
            className="form-input"
            type="text"
            placeholder="room-name"
            value={newRoomName}
            onChange={e => setNewRoomName(e.target.value)}
          />
          <button
            type="submit"
            className="btn btn-primary"
            disabled={creating || !newRoomName.trim()}
          >
            {creating ? 'Creating…' : 'Create'}
          </button>
        </form>
        {createError && <p className="error-msg" style={{ marginBottom: 16 }}>{createError}</p>}

        {/* All public rooms */}
        <div className="section-title">All rooms</div>
        {loading ? (
          <p className="loading">Loading rooms…</p>
        ) : fetchError ? (
          <div className="load-error" style={{ textAlign: 'left', padding: 0 }}>
            <p style={{ marginBottom: 12 }}>⚠ {fetchError}</p>
            <div style={{ display: 'flex', gap: 8 }}>
              <button className="btn btn-secondary" onClick={fetchData}>Try again</button>
              <button className="btn btn-ghost" onClick={() => window.location.reload()}>Refresh page</button>
            </div>
          </div>
        ) : publicRooms.length === 0 ? (
          <p className="empty-state">No rooms yet — create the first one!</p>
        ) : (
          <div className="rooms-grid">
            {publicRooms.map(room => {
              const joined = joinedIds.has(room.id);
              return (
                <div key={room.id} className="room-card">
                  <div className="room-card-name">
                    <span className="hash">#</span>
                    {room.name ?? 'Unnamed'}
                    {joined && <span className="badge badge-joined">joined</span>}
                  </div>
                  <div className="room-card-actions">
                    {joined ? (
                      <>
                        <button
                          className="btn btn-primary"
                          onClick={() => navigate(`/rooms/${room.id}`, { state: { roomName: room.name } })}
                        >
                          Open
                        </button>
                        <button
                          className="btn btn-secondary"
                          onClick={() => handleLeave(room)}
                        >
                          Leave
                        </button>
                      </>
                    ) : (
                      <button className="btn btn-primary" onClick={() => handleJoin(room)}>
                        Join
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </>
  );
}
