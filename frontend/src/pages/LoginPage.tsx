import { useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

function getErrorHint(error: string): string {
  const e = error.toLowerCase();
  if (e.includes('connect') || e.includes('internet') || e.includes('network')) {
    return 'Check your internet connection and try again.';
  }
  if (e.includes('server error') || e.includes('try again in')) {
    return 'Our servers are experiencing issues. Please wait a moment and try again.';
  }
  if (e.includes('too many')) {
    return 'Wait a few minutes before trying again.';
  }
  return 'Double-check your username and password.';
}

export default function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await login(username.trim(), password);
      navigate('/rooms', { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <h1>Welcome back</h1>
        <p className="subtitle">Sign in to your account</p>

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="username">Username</label>
            <input
              id="username"
              className="form-input"
              type="text"
              placeholder="your_username"
              value={username}
              onChange={e => setUsername(e.target.value)}
              autoComplete="username"
              autoFocus
            />
          </div>

          <div className="form-group">
            <label htmlFor="password">Password</label>
            <input
              id="password"
              className="form-input"
              type="password"
              placeholder="••••••••"
              value={password}
              onChange={e => setPassword(e.target.value)}
              autoComplete="current-password"
            />
          </div>

          {error && (
            <div className="error-box">
              <p className="error-msg">{error}</p>
              <p className="error-hint">{getErrorHint(error)}</p>
            </div>
          )}

          <button
            type="submit"
            className="btn btn-primary btn-full"
            disabled={loading || !username || !password}
            style={{ marginTop: 20 }}
          >
            {loading ? 'Signing in…' : 'Sign in'}
          </button>
        </form>

        <p style={{ marginTop: 20, color: 'var(--text-2)', fontSize: 13 }}>
          No account? <Link to="/register">Create one</Link>
        </p>
      </div>
    </div>
  );
}
