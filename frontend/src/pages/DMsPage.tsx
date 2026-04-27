import { useEffect, useState } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';
import { listUsers, createOrGetDM, type UserProfile } from '../api/client';
import { useAuth } from '../contexts/AuthContext';
import { useToast } from '../contexts/ToastContext';
import type { SidebarContext } from '../components/Layout';

export default function DMsPage() {
  const { userId } = useAuth();
  const { refreshSidebar } = useOutletContext<SidebarContext>();
  const navigate = useNavigate();
  const { show: toast } = useToast();

  const [users, setUsers] = useState<UserProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState('');
  const [starting, setStarting] = useState<string | null>(null);

  const fetchUsers = () => {
    setLoading(true);
    setFetchError('');
    listUsers()
      .then(setUsers)
      .catch(err => {
        setFetchError(err instanceof Error ? err.message : 'Failed to load users');
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchUsers(); }, []);

  const handleStartDM = async (user: UserProfile) => {
    setStarting(user.id);
    try {
      const dm = await createOrGetDM(user.id);
      refreshSidebar();
      navigate(`/dms/${dm.room.id}`, { state: { otherUser: dm.other_user } });
    } catch (err) {
      toast(
        err instanceof Error ? err.message : 'Failed to open conversation',
        'error',
        'Please try again, or refresh the page if the issue continues.',
      );
    } finally {
      setStarting(null);
    }
  };

  const others = users.filter(u => u.id !== userId);

  return (
    <>
      <div className="page-header">
        <h2>Direct Messages</h2>
      </div>

      <div className="page-body">
        <div className="section-title">Users</div>
        <p style={{ color: 'var(--text-2)', fontSize: 13, marginBottom: 16 }}>
          Click a user to open or start a conversation.
        </p>

        {loading ? (
          <p className="loading">Loading users…</p>
        ) : fetchError ? (
          <div className="load-error" style={{ textAlign: 'left', padding: 0 }}>
            <p style={{ marginBottom: 12 }}>⚠ {fetchError}</p>
            <div style={{ display: 'flex', gap: 8 }}>
              <button className="btn btn-secondary" onClick={fetchUsers}>Try again</button>
              <button className="btn btn-ghost" onClick={() => window.location.reload()}>Refresh page</button>
            </div>
          </div>
        ) : others.length === 0 ? (
          <p className="empty-state">No other users registered yet.</p>
        ) : (
          <div className="users-list">
            {others.map(user => (
              <div key={user.id} className="user-row">
                <div className="u-avatar">
                  {user.username.charAt(0).toUpperCase()}
                </div>
                <span className="u-name">{user.username}</span>
                <button
                  className="btn btn-primary"
                  onClick={() => handleStartDM(user)}
                  disabled={starting === user.id}
                >
                  {starting === user.id ? '…' : 'Message'}
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </>
  );
}
