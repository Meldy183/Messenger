import {
  useCallback, useEffect, useRef, useState,
  type DragEvent, type FormEvent, type ChangeEvent,
} from 'react';
import { useParams, useLocation } from 'react-router-dom';
import { listMessages, type Message } from '../api/client';
import { useAuth } from '../contexts/AuthContext';
import { useWebSocket, type AttachmentMeta } from '../contexts/WebSocketContext';
import EmojiPicker from '../components/EmojiPicker';
import ImageViewer from '../components/ImageViewer';

// Extends the base Message with an optional local-only attachment field
interface AppMessage extends Message {
  _attachment?: AttachmentMeta;
}

interface PendingFile {
  id: string;
  meta: AttachmentMeta;
}

interface Props {
  type: 'room' | 'dm';
}

function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function formatDate(iso: string) {
  const d = new Date(iso);
  const today = new Date();
  if (d.toDateString() === today.toDateString()) return 'Today';
  const yesterday = new Date(today);
  yesterday.setDate(today.getDate() - 1);
  if (d.toDateString() === yesterday.toDateString()) return 'Yesterday';
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
}

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1048576).toFixed(1)} MB`;
}

function fileToMeta(file: File): AttachmentMeta {
  return {
    type: file.type.startsWith('image/') ? 'image' : file.type.startsWith('video/') ? 'video' : 'file',
    url: URL.createObjectURL(file),
    name: file.name,
    size: file.size,
  };
}

export default function ChatPage({ type }: Props) {
  const { id: roomId } = useParams<{ id: string }>();
  const location = useLocation();
  const { userId } = useAuth();
  const { connected, reconnecting, reconnectIn, reconnectNow, sendMessage, subscribe } = useWebSocket();

  const stateTitle: string | undefined =
    type === 'dm'
      ? (location.state as { otherUser?: { username: string } } | null)?.otherUser?.username
      : (location.state as { roomName?: string | null } | null)?.roomName ?? undefined;
  const title = stateTitle ?? roomId?.slice(0, 8) ?? '';
  const prefix = type === 'dm' ? '@' : '#';

  const [messages, setMessages] = useState<AppMessage[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [showEmoji, setShowEmoji] = useState(false);
  const [pendingFiles, setPendingFiles] = useState<PendingFile[]>([]);
  const [showConsent, setShowConsent] = useState(false);
  const [isDragOver, setIsDragOver] = useState(false);
  const [lightboxSrc, setLightboxSrc] = useState<string | null>(null);

  const seenIds = useRef<Set<string>>(new Set());
  const bottomRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const dragCounter = useRef(0);
  // Store files dropped before consent is granted so we can process them after
  const pendingDropRef = useRef<File[]>([]);

  // ── Load history ────────────────────────────────────────────
  const fetchMessages = useCallback(() => {
    if (!roomId) return;
    setLoading(true);
    setLoadError(null);
    seenIds.current.clear();
    setMessages([]);

    listMessages(roomId)
      .then(history => {
        const sorted = [...history].reverse();
        sorted.forEach(m => seenIds.current.add(m.id));
        setMessages(sorted as AppMessage[]);
      })
      .catch(err => setLoadError(err instanceof Error ? err.message : 'Failed to load messages'))
      .finally(() => setLoading(false));
  }, [roomId]);

  useEffect(() => { fetchMessages(); }, [fetchMessages]);

  // ── Real-time subscription ───────────────────────────────────
  useEffect(() => {
    if (!roomId) return;
    const unsub = subscribe(roomId, (msg: Message) => {
      if (seenIds.current.has(msg.id)) return;
      seenIds.current.add(msg.id);
      setMessages(prev => [...prev, msg as AppMessage]);
    });
    return unsub;
  }, [roomId, subscribe]);

  // ── Auto-scroll ──────────────────────────────────────────────
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // ── Close emoji picker on outside click ─────────────────────
  useEffect(() => {
    if (!showEmoji) return;
    const handle = (e: MouseEvent) => {
      const t = e.target as HTMLElement;
      if (!t.closest('.emoji-picker') && !t.closest('.emoji-trigger-btn')) {
        setShowEmoji(false);
      }
    };
    document.addEventListener('mousedown', handle);
    return () => document.removeEventListener('mousedown', handle);
  }, [showEmoji]);

  // ── Send ─────────────────────────────────────────────────────
  const handleSend = (e: FormEvent) => {
    e.preventDefault();
    const text = input.trim();
    if (!text && pendingFiles.length === 0) return;
    if (!roomId) return;

    if (pendingFiles.length > 0) {
      // First file gets the text, rest go without text
      sendMessage(roomId, text, pendingFiles[0].meta);
      pendingFiles.slice(1).forEach(pf => sendMessage(roomId, '', pf.meta));
      setPendingFiles([]);
    } else {
      sendMessage(roomId, text);
    }
    setInput('');
  };

  // ── Emoji ────────────────────────────────────────────────────
  const handleEmojiSelect = (emoji: string) => {
    const el = inputRef.current;
    if (!el) { setInput(p => p + emoji); return; }
    const start = el.selectionStart ?? input.length;
    const end   = el.selectionEnd   ?? input.length;
    const next  = input.slice(0, start) + emoji + input.slice(end);
    setInput(next);
    setTimeout(() => {
      el.focus();
      el.setSelectionRange(start + emoji.length, start + emoji.length);
    }, 0);
  };

  // ── Files ─────────────────────────────────────────────────────
  const processFiles = (files: File[]) => {
    const added = files.map(f => ({ id: `pf-${Date.now()}-${Math.random()}`, meta: fileToMeta(f) }));
    setPendingFiles(prev => [...prev, ...added]);
  };

  const openFilePicker = () => { fileInputRef.current?.click(); };

  const handleAttachClick = () => {
    if (localStorage.getItem('fileConsentGiven') === 'true') {
      openFilePicker();
    } else {
      setShowConsent(true);
    }
  };

  const handleConsentAllow = () => {
    localStorage.setItem('fileConsentGiven', 'true');
    setShowConsent(false);
    if (pendingDropRef.current.length > 0) {
      processFiles(pendingDropRef.current);
      pendingDropRef.current = [];
    } else {
      openFilePicker();
    }
  };

  const handleConsentDeny = () => {
    pendingDropRef.current = [];
    setShowConsent(false);
  };

  const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    if (e.target.files?.length) {
      processFiles(Array.from(e.target.files));
      e.target.value = '';
    }
  };

  // ── Drag & Drop ──────────────────────────────────────────────
  const handleDragEnter = (e: DragEvent) => {
    e.preventDefault();
    if (++dragCounter.current === 1) setIsDragOver(true);
  };
  const handleDragLeave = (e: DragEvent) => {
    e.preventDefault();
    if (--dragCounter.current === 0) setIsDragOver(false);
  };
  const handleDragOver = (e: DragEvent) => { e.preventDefault(); };
  const handleDrop = (e: DragEvent) => {
    e.preventDefault();
    dragCounter.current = 0;
    setIsDragOver(false);
    const files = Array.from(e.dataTransfer.files);
    if (!files.length) return;
    if (localStorage.getItem('fileConsentGiven') === 'true') {
      processFiles(files);
    } else {
      pendingDropRef.current = files;
      setShowConsent(true);
    }
  };

  // ── Group by date ────────────────────────────────────────────
  const grouped: { date: string; items: AppMessage[] }[] = [];
  for (const msg of messages) {
    const date = formatDate(msg.created_at);
    const last = grouped[grouped.length - 1];
    if (last?.date === date) last.items.push(msg);
    else grouped.push({ date, items: [msg] });
  }

  const canSend = connected && (input.trim().length > 0 || pendingFiles.length > 0);

  return (
    <div
      className="chat-layout"
      onDragEnter={handleDragEnter}
      onDragLeave={handleDragLeave}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
    >

      {/* Drag overlay */}
      {isDragOver && (
        <div className="drag-overlay">
          <div className="drag-overlay-box">
            <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
              <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>
            </svg>
            <p>Drop files here to attach</p>
          </div>
        </div>
      )}

      {/* Header */}
      <div className="page-header">
        <h2>{prefix}{title}</h2>
      </div>

      {/* WS reconnect banner */}
      {!connected && (
        <div className={`ws-banner${!reconnecting ? ' error' : ''}`}>
          {reconnecting ? (
            <>Reconnecting in {reconnectIn}s… <button className="link-btn" onClick={reconnectNow}>Connect now</button></>
          ) : (
            <>Connection lost. <button className="link-btn" onClick={reconnectNow}>Reconnect</button> or <button className="link-btn" onClick={() => window.location.reload()}>refresh the page</button>.</>
          )}
        </div>
      )}

      {/* Messages */}
      <div className="messages-area">
        <div className="messages-inner">

          {loading && <p className="loading">Loading messages…</p>}

          {loadError && !loading && (
            <div className="load-error">
              <p>⚠ {loadError}</p>
              <div style={{ display: 'flex', gap: 8, justifyContent: 'center', flexWrap: 'wrap' }}>
                <button className="btn btn-secondary" onClick={fetchMessages}>Try again</button>
                <button className="btn btn-ghost" onClick={() => window.location.reload()}>Refresh page</button>
              </div>
            </div>
          )}

          {!loading && !loadError && messages.length === 0 && (
            <p className="empty-state" style={{ margin: 'auto', textAlign: 'center' }}>
              No messages yet — send the first one!
            </p>
          )}

          {grouped.map(group => (
            <div key={group.date}>
              <div className="date-divider">
                <span className="date-divider-line" />
                <span className="date-divider-label">{group.date}</span>
                <span className="date-divider-line" />
              </div>

              {group.items.map((msg, i) => {
                const isOwn = msg.sender_id === userId;
                const prevMsg = group.items[i - 1];
                const showMeta = !prevMsg || prevMsg.sender_id !== msg.sender_id;

                return (
                  <div key={msg.id} className={`message-row ${isOwn ? 'own' : 'other'}`}>
                    {/* Avatar only for others */}
                    {!isOwn && (
                      <div className="message-avatar">
                        {msg.sender_username.charAt(0).toUpperCase()}
                      </div>
                    )}

                    <div className={`message-col ${isOwn ? 'own' : ''}`}>
                      {showMeta && (
                        <div className="message-meta">
                          {!isOwn && <span className="message-sender">{msg.sender_username}</span>}
                          <span className="message-time">{formatTime(msg.created_at)}</span>
                        </div>
                      )}

                      {/* Attachment */}
                      {msg._attachment && (
                        <div className="msg-attachment">
                          {msg._attachment.type === 'image' && (
                            <img
                              src={msg._attachment.url}
                              alt={msg._attachment.name}
                              className="msg-media msg-media-clickable"
                              onClick={() => setLightboxSrc(msg._attachment!.url)}
                            />
                          )}
                          {msg._attachment.type === 'video' && (
                            <video
                              src={msg._attachment.url}
                              className="msg-media"
                              controls
                            />
                          )}
                          {msg._attachment.type === 'file' && (
                            <div className="msg-file">
                              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                                <polyline points="14 2 14 8 20 8"/>
                              </svg>
                              <div>
                                <div className="msg-file-name">{msg._attachment.name}</div>
                                <div className="msg-file-size">{formatSize(msg._attachment.size)}</div>
                              </div>
                            </div>
                          )}
                        </div>
                      )}

                      {/* Text bubble */}
                      {msg.content && (
                        <div className="message-bubble">{msg.content}</div>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          ))}

          <div ref={bottomRef} />
        </div>
      </div>

      {/* Input area */}
      <div className="message-input-area">
        <div className="message-input-inner">

          {/* Pending file chips */}
          {pendingFiles.length > 0 && (
            <div className="attachment-chips">
              {pendingFiles.map(pf => (
                <div key={pf.id} className="attachment-chip">
                  {pf.meta.type === 'image'
                    ? <img src={pf.meta.url} alt={pf.meta.name} className="chip-thumb" />
                    : <span className="chip-icon">{pf.meta.type === 'video' ? '🎬' : '📄'}</span>
                  }
                  <span className="chip-name">{pf.meta.name}</span>
                  <button
                    type="button"
                    className="chip-remove"
                    onClick={() => setPendingFiles(p => p.filter(f => f.id !== pf.id))}
                    aria-label="Remove attachment"
                  >✕</button>
                </div>
              ))}
            </div>
          )}

          <form className="message-input-row" onSubmit={handleSend}>

            {/* Emoji button + popup */}
            <div className="input-action-wrap">
              <button
                type="button"
                className="input-action-btn emoji-trigger-btn"
                title="Emoji"
                onClick={() => setShowEmoji(v => !v)}
              >
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="10"/>
                  <path d="M8 14s1.5 2 4 2 4-2 4-2"/>
                  <line x1="9" y1="9" x2="9.01" y2="9"/>
                  <line x1="15" y1="9" x2="15.01" y2="9"/>
                </svg>
              </button>
              {showEmoji && (
                <EmojiPicker onSelect={(e) => { handleEmojiSelect(e); }} />
              )}
            </div>

            {/* Text input */}
            <input
              ref={inputRef}
              className="form-input"
              type="text"
              placeholder={
                connected
                  ? `Message ${prefix}${title}`
                  : reconnecting
                    ? `Reconnecting in ${reconnectIn}s…`
                    : 'Disconnected'
              }
              value={input}
              onChange={e => setInput(e.target.value)}
              disabled={!connected}
              autoComplete="off"
            />

            {/* Attach button */}
            <button
              type="button"
              className="input-action-btn"
              title="Attach file"
              onClick={handleAttachClick}
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>
              </svg>
            </button>

            <input
              ref={fileInputRef}
              type="file"
              multiple
              accept="image/*,video/*,*/*"
              style={{ display: 'none' }}
              onChange={handleFileChange}
            />

            <button
              type="submit"
              className="btn btn-primary"
              disabled={!canSend}
            >
              Send
            </button>
          </form>
        </div>
      </div>

      {/* Image lightbox */}
      {lightboxSrc && (
        <ImageViewer src={lightboxSrc} onClose={() => setLightboxSrc(null)} />
      )}

      {/* File access consent modal */}
      {showConsent && (
        <div className="consent-overlay" onClick={handleConsentDeny}>
          <div className="consent-modal" onClick={e => e.stopPropagation()}>
            <div className="consent-icon">
              <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>
              </svg>
            </div>
            <h3>Allow access to files</h3>
            <p>
              Messenger would like to access photos, videos, and files on your device
              so you can share media in conversations.
            </p>
            <p className="consent-note">
              Files you share will be visible to all participants in this conversation.
              You can revoke access at any time in your browser settings.
            </p>
            <div className="consent-actions">
              <button className="btn btn-secondary" onClick={handleConsentDeny}>
                Don't allow
              </button>
              <button className="btn btn-primary" onClick={handleConsentAllow}>
                Allow access
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
