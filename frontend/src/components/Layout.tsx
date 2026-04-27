import { useEffect, useState, useCallback } from 'react';
import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { listJoinedRooms, listDMs, type Room, type DMRoom } from '../api/client';

export interface SidebarContext {
  refreshSidebar: () => void;
}

export default function Layout() {
  const { username, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  const [rooms, setRooms] = useState<Room[]>([]);
  const [dms, setDMs] = useState<DMRoom[]>([]);

  const refreshSidebar = useCallback(() => {
    listJoinedRooms()
      .then(r => setRooms(r.filter(rm => !rm.is_dm)))
      .catch(() => {});
    listDMs()
      .then(setDMs)
      .catch(() => {});
  }, []);

  // Refresh whenever the route changes (user navigated away after mutation)
  useEffect(() => {
    refreshSidebar();
  }, [location.pathname, refreshSidebar]);

  const handleLogout = () => {
    logout();
    navigate('/login', { replace: true });
  };

  return (
    <div className="app-layout">
      <nav className="sidebar">
        <div className="sidebar-header">
          <span>💬</span> Messenger
        </div>

        <div className="sidebar-body">
          {/* Public rooms the user has joined */}
          <div className="sidebar-section">
            <div className="sidebar-section-title">Rooms</div>
            <NavLink
              to="/rooms"
              end
              className={({ isActive }) => `sidebar-item${isActive ? ' active' : ''}`}
            >
              <span className="sidebar-item-prefix">#</span> Browse rooms
            </NavLink>
            {rooms.map(r => (
              <NavLink
                key={r.id}
                to={`/rooms/${r.id}`}
                state={{ roomName: r.name }}
                className={({ isActive }) => `sidebar-item${isActive ? ' active' : ''}`}
              >
                <span className="sidebar-item-prefix">#</span>
                {r.name ?? r.id.slice(0, 8)}
              </NavLink>
            ))}
          </div>

          {/* Direct messages */}
          <div className="sidebar-section">
            <div className="sidebar-section-title">Direct Messages</div>
            <NavLink
              to="/dms"
              end
              className={({ isActive }) => `sidebar-item${isActive ? ' active' : ''}`}
            >
              <span className="sidebar-item-prefix">+</span> New message
            </NavLink>
            {dms.map(dm => (
              <NavLink
                key={dm.room.id}
                to={`/dms/${dm.room.id}`}
                state={{ otherUser: dm.other_user }}
                className={({ isActive }) => `sidebar-item${isActive ? ' active' : ''}`}
              >
                <span className="sidebar-item-prefix">@</span>
                {dm.other_user.username}
              </NavLink>
            ))}
          </div>
        </div>

        <div className="sidebar-footer">
          <span className="username">{username}</span>
          <NavLink
            to="/account"
            className={({ isActive }) => `btn btn-ghost${isActive ? ' active' : ''}`}
            title="Account"
            style={{ padding: '4px 8px', fontSize: 13 }}
          >
            ⚙
          </NavLink>
          <button className="btn btn-ghost" onClick={handleLogout} title="Log out" style={{ padding: '4px 8px', fontSize: 13 }}>
            ⏏
          </button>
        </div>
      </nav>

      <main className="main-content">
        <Outlet context={{ refreshSidebar } satisfies SidebarContext} />
      </main>
    </div>
  );
}
