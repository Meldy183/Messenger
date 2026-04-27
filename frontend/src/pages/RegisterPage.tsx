import { useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { register } from '../api/client';
import { useAuth } from '../contexts/AuthContext';

function getErrorHint(error: string): string {
  const e = error.toLowerCase();
  if (e.includes('taken') || e.includes('already exists') || e.includes('already')) {
    return 'Try a different username.';
  }
  if (e.includes('connect') || e.includes('internet') || e.includes('network')) {
    return 'Check your internet connection and try again.';
  }
  if (e.includes('server error') || e.includes('try again in')) {
    return 'Our servers are experiencing issues. Please wait a moment and try again.';
  }
  if (e.includes('too many')) {
    return 'Wait a few minutes before trying again.';
  }
  return 'Please check your details and try again.';
}

export default function RegisterPage() {
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
      await register(username.trim(), password);
      await login(username.trim(), password);
      navigate('/rooms', { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <h1>Create account</h1>
        <p className="subtitle">Join the conversation</p>

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
              autoComplete="new-password"
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
            {loading ? 'Creating account…' : 'Create account'}
          </button>
        </form>

        <p style={{ marginTop: 20, color: 'var(--text-2)', fontSize: 13 }}>
          Already have an account? <Link to="/login">Sign in</Link>
        </p>
      </div>
    </div>
  );
}
