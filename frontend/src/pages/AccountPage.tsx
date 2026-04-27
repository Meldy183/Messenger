import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getMe, type UserProfile } from '../api/client';
import { useAuth } from '../contexts/AuthContext';

export default function AccountPage() {
  const { logout } = useAuth();
  const navigate = useNavigate();
  const [profile, setProfile] = useState<UserProfile | null>(null);

  useEffect(() => {
    getMe().then(setProfile).catch(() => {});
  }, []);

  const handleLogout = () => {
    logout();
    navigate('/login', { replace: true });
  };

  return (
    <>
      <div className="page-header">
        <h2>Account</h2>
      </div>

      <div className="page-body">
        {profile ? (
          <div className="account-card">
            <div className="account-avatar">
              {profile.username.charAt(0).toUpperCase()}
            </div>
            <div className="account-username">{profile.username}</div>
            <div className="account-id">ID: {profile.id}</div>

            <button className="btn btn-danger" onClick={handleLogout}>
              Log out
            </button>
          </div>
        ) : (
          <p className="loading">Loading…</p>
        )}
      </div>
    </>
  );
}
