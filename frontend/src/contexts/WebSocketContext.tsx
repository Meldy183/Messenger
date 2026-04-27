import React, { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react';
import type { Message } from '../api/client';

type MessageHandler = (msg: Message) => void;

export interface AttachmentMeta {
  type: 'image' | 'video' | 'file';
  url: string;
  name: string;
  size: number;
}

export interface WSContextValue {
  connected: boolean;
  reconnecting: boolean;
  reconnectIn: number;
  reconnectNow: () => void;
  sendMessage: (roomId: string, content: string, attachment?: AttachmentMeta) => void;
  subscribe: (roomId: string, handler: MessageHandler) => () => void;
}

const WSContext = createContext<WSContextValue | null>(null);

const BACKOFF_MS = [1500, 3000, 6000, 12000, 25000, 30000];
const MOCK = import.meta.env.VITE_MOCK === 'true';

export function WebSocketProvider({ token, children }: { token: string | null; children: React.ReactNode }) {
  const wsRef = useRef<WebSocket | null>(null);
  const handlersRef = useRef<Map<string, Set<MessageHandler>>>(new Map());
  const attemptRef = useRef(0);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const countdownIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const [connected, setConnected] = useState(MOCK);
  const [reconnecting, setReconnecting] = useState(false);
  const [reconnectIn, setReconnectIn] = useState(0);
  const [retryKey, setRetryKey] = useState(0);

  const clearScheduled = useCallback(() => {
    if (reconnectTimerRef.current) { clearTimeout(reconnectTimerRef.current); reconnectTimerRef.current = null; }
    if (countdownIntervalRef.current) { clearInterval(countdownIntervalRef.current); countdownIntervalRef.current = null; }
  }, []);

  useEffect(() => {
    if (MOCK) return;

    if (!token) {
      clearScheduled();
      wsRef.current?.close();
      wsRef.current = null;
      setConnected(false);
      setReconnecting(false);
      setReconnectIn(0);
      attemptRef.current = 0;
      return;
    }

    clearScheduled();
    setReconnecting(false);
    setReconnectIn(0);

    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const ws = new WebSocket(`${protocol}://${window.location.host}/ws?token=${token}`);
    wsRef.current = ws;
    let cancelled = false;

    ws.onopen = () => {
      attemptRef.current = 0;
      setConnected(true);
    };

    ws.onclose = () => {
      setConnected(false);
      wsRef.current = null;
      if (cancelled) return;

      const delay = BACKOFF_MS[Math.min(attemptRef.current, BACKOFF_MS.length - 1)];
      attemptRef.current += 1;

      let remaining = Math.ceil(delay / 1000);
      setReconnecting(true);
      setReconnectIn(remaining);

      countdownIntervalRef.current = setInterval(() => {
        remaining -= 1;
        setReconnectIn(Math.max(0, remaining));
      }, 1000);

      reconnectTimerRef.current = setTimeout(() => {
        clearInterval(countdownIntervalRef.current!);
        countdownIntervalRef.current = null;
        setRetryKey(k => k + 1);
      }, delay);
    };

    ws.onerror = () => {};

    ws.onmessage = (event: MessageEvent<string>) => {
      try {
        const data = JSON.parse(event.data) as { type: string; message: Message };
        if (data.type === 'new_message' && data.message) {
          const msg = data.message;
          handlersRef.current.get(msg.room_id)?.forEach(h => h(msg));
        }
      } catch {
        // ignore malformed frames
      }
    };

    return () => {
      cancelled = true;
      clearScheduled();
      ws.close();
      wsRef.current = null;
    };
  }, [token, retryKey, clearScheduled]);

  const reconnectNow = useCallback(() => {
    if (MOCK) return;
    clearScheduled();
    attemptRef.current = 0;
    setReconnecting(false);
    setReconnectIn(0);
    setRetryKey(k => k + 1);
  }, [clearScheduled]);

  const sendMessage = useCallback((roomId: string, content: string, attachment?: AttachmentMeta) => {
    if (MOCK) {
      const msg = {
        id: `mock-${Date.now()}-${Math.random()}`,
        room_id: roomId,
        sender_id: 'u-me',
        sender_username: localStorage.getItem('username') ?? 'you',
        content,
        created_at: new Date().toISOString(),
        _attachment: attachment,
      } as unknown as Message;
      handlersRef.current.get(roomId)?.forEach(h => h(msg));
      return;
    }
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type: 'send_message', room_id: roomId, content }));
  }, []);

  const subscribe = useCallback((roomId: string, handler: MessageHandler) => {
    const map = handlersRef.current;
    if (!map.has(roomId)) map.set(roomId, new Set());
    map.get(roomId)!.add(handler);
    return () => { map.get(roomId)?.delete(handler); };
  }, []);

  return (
    <WSContext.Provider value={{ connected, reconnecting, reconnectIn, reconnectNow, sendMessage, subscribe }}>
      {children}
    </WSContext.Provider>
  );
}

export function useWebSocket(): WSContextValue {
  const ctx = useContext(WSContext);
  if (!ctx) throw new Error('useWebSocket must be used within WebSocketProvider');
  return ctx;
}
