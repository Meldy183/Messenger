import React, { createContext, useCallback, useContext, useState } from 'react';
import * as api from '../api/client';

interface AuthState {
  token: string | null;
  userId: string | null;
  username: string | null;
}

interface AuthContextValue extends AuthState {
  isAuthenticated: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function loadState(): AuthState {
  return {
    token:    localStorage.getItem('token'),
    userId:   localStorage.getItem('userId'),
    username: localStorage.getItem('username'),
  };
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>(loadState);

  const login = useCallback(async (username: string, password: string) => {
    const { token } = await api.login(username, password);
    // Store token first so subsequent calls include the Authorization header.
    localStorage.setItem('token', token);
    const me = await api.getMe();
    localStorage.setItem('userId', me.id);
    localStorage.setItem('username', me.username);
    setState({ token, userId: me.id, username: me.username });
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem('token');
    localStorage.removeItem('userId');
    localStorage.removeItem('username');
    setState({ token: null, userId: null, username: null });
  }, []);

  return (
    <AuthContext.Provider value={{ ...state, isAuthenticated: !!state.token, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
