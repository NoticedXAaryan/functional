import { useState, useEffect, useCallback } from 'react';
import './App.css';
import { API_BASE, initializeStaffAuth, signInWithOrganization, signOutOfOrganization, staffFetch as fetch } from './auth';
import type { StaffClaims } from './auth';

interface CaseRow {
  id: string;
  status: string;
  route_type: string;
  conflict_flag: string | null;
  created_at: string;
  organization_id: string;
}

interface Assignment {
  id: string;
  case_id: string;
  responder_id: string | null;
  status: string;
  accepted_at: string | null;
  assigned_at: string;
}

interface Message {
  id: string;
  body: string;
  sender_type: string;
  is_staff_note: boolean;
  sent_at: string;
}

interface AlertDraft {
  id: string;
  alert_id: string;
  status: string;
  description_text: string;
  issuer_name: string;
  expiry_at: string;
  prepared_by: string;
}

type Screen = 'login' | 'cases' | 'case_detail' | 'alerts';

function App() {
  const [staff, setStaff] = useState<StaffClaims | null>(null);
  const [authMode, setAuthMode] = useState<'demo' | 'oidc' | null>(null);

  const [screen, setScreen] = useState<Screen>(staff ? 'cases' : 'login');
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    initializeStaffAuth().then(auth => {
      if (!active) return;
      setAuthMode(auth.mode);
      setStaff(auth.staff);
      setScreen(auth.staff ? 'cases' : 'login');
      if (auth.client?.authenticated && !auth.staff) {
        setErrorMsg('Your organization account has no active staff access. Ask your administrator to assign it.');
      }
    }).catch(error => { if (active) setErrorMsg(error instanceof Error ? error.message : 'Sign-in is unavailable.'); });
    return () => { active = false; };
  }, []);

  // Login form
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  // Cases
  const [cases, setCases] = useState<CaseRow[]>([]);
  const [selectedCase, setSelectedCase] = useState<CaseRow | null>(null);
  const [assignments, setAssignments] = useState<Assignment[]>([]);
  const [messages, setMessages] = useState<Message[]>([]);
  const [newMessage, setNewMessage] = useState('');
  const [isStaffNote, setIsStaffNote] = useState(false);

  // Alert review
  const [alertDrafts, setAlertDrafts] = useState<AlertDraft[]>([]);

  const authHeaders = useCallback(() => ({
    'Content-Type': 'application/json',
    ...(staff ? { 'Authorization': `Bearer ${staff.token}` } : {}),
  }), [staff]);

  // ── Login ──────────────────────────────────────────────────────────────
  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setErrorMsg(null);
    try {
      const res = await fetch(`${API_BASE}/staff/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Login failed');
      const claims: StaffClaims = {
        token: data.token,
        staffId: data.staff_id,
        orgId: data.organization_id,
        role: data.role,
      };
      setPassword('');
      setStaff(claims);
      setScreen('cases');
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleLogout = () => {
    sessionStorage.removeItem('staff_token');
    setStaff(null);
    setCases([]);
    setSelectedCase(null);
    setMessages([]);
    setAlertDrafts([]);
    setScreen('login');
    if (authMode === 'oidc') {
      void signOutOfOrganization().catch(() => setErrorMsg('Signed out of this screen. Organization logout failed; close this tab and end your organization session.'));
    }
  };

  // ── Fetch case queue ────────────────────────────────────────────────────
  const fetchCases = useCallback(async () => {
    if (!staff) return;
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/staff/cases`, { headers: authHeaders() });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to load cases');
      setCases(data.cases || []);
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  }, [staff, authHeaders]);

  useEffect(() => {
    if (screen === 'cases' && staff) fetchCases();
  }, [screen, staff, fetchCases]);

  // ── Open case detail ────────────────────────────────────────────────────
  const openCase = async (caseRow: CaseRow) => {
    setSelectedCase(caseRow);
    setErrorMsg(null);
    setLoading(true);
    try {
      const [aRes, mRes] = await Promise.all([
        fetch(`${API_BASE}/staff/assignments?case_id=${caseRow.id}`, { headers: authHeaders() }),
        fetch(`${API_BASE}/staff/messages?case_id=${caseRow.id}`, { headers: authHeaders() }),
      ]);
      const aData = await aRes.json();
      const mData = await mRes.json();
      setAssignments(aData.assignments || []);
      setMessages(mData.messages || []);
      setScreen('case_detail');
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  // ── Explicit accept assignment ─────────────────────────────────────────
  const handleAccept = async (assignmentId: string) => {
    if (!staff) return;
    setLoading(true);
    setErrorMsg(null);
    try {
      const res = await fetch(`${API_BASE}/staff/assignments/${assignmentId}/accept`, {
        method: 'POST',
        headers: authHeaders(),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to accept assignment');
      // Refresh assignments
      if (selectedCase) {
        const aRes = await fetch(`${API_BASE}/staff/assignments?case_id=${selectedCase.id}`, {
          headers: authHeaders(),
        });
        const aData = await aRes.json();
        setAssignments(aData.assignments || []);
      }
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  // ── Send message ────────────────────────────────────────────────────────
  const handleSendMessage = async () => {
    if (!selectedCase || !newMessage.trim()) return;
    setLoading(true);
    setErrorMsg(null);
    try {
      const res = await fetch(`${API_BASE}/staff/messages`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({
          case_id: selectedCase.id,
          body: newMessage.trim(),
          is_staff_note: isStaffNote,
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to send message');
      setNewMessage('');
      // Refresh messages
      const mRes = await fetch(`${API_BASE}/staff/messages?case_id=${selectedCase.id}`, {
        headers: authHeaders(),
      });
      const mData = await mRes.json();
      setMessages(mData.messages || []);
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  // ── Alert drafts ────────────────────────────────────────────────────────
  const fetchAlertDrafts = useCallback(async () => {
    if (!staff) return;
    setLoading(true);
    try {
      // Approver role: list drafts in review
      const res = await fetch(`${API_BASE}/staff/alert-approvals`, { headers: authHeaders() });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to load alerts');
      setAlertDrafts(data.drafts || []);
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  }, [staff, authHeaders]);

  useEffect(() => {
    if (screen === 'alerts' && staff) fetchAlertDrafts();
  }, [screen, staff, fetchAlertDrafts]);

  const handleApprove = async (revisionId: string) => {
    setLoading(true);
    setErrorMsg(null);
    try {
      const res = await fetch(`${API_BASE}/staff/alert-approvals`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ revision_id: revisionId, decision: 'approve' }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Approval failed');
      fetchAlertDrafts();
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  const statusColor = (s: string) => {
    const map: Record<string, string> = {
      RECEIVED: 'var(--badge-cyan)',
      TRIAGE: 'var(--badge-amber)',
      ASSIGNED: 'var(--badge-purple)',
      ACCEPTED: 'var(--badge-emerald)',
      IN_PROGRESS: 'var(--badge-blue)',
      FOLLOW_UP: 'var(--badge-orange)',
      CLOSED: 'var(--badge-gray)',
    };
    return map[s] || 'var(--badge-gray)';
  };

  // ── Render ────────────────────────────────────────────────────────────
  return (
    <div className="ops-shell">
      {/* Header */}
      <header className="ops-header">
        <div className="ops-brand">
          <span className="ops-brand-icon">🛡️</span>
          <span className="ops-brand-name">Bal Suraksha <span className="ops-badge-ops">OPS</span></span>
        </div>
        {staff && (
          <nav className="ops-nav">
            <button className={`ops-nav-btn${screen === 'cases' || screen === 'case_detail' ? ' active' : ''}`}
              onClick={() => { setScreen('cases'); setSelectedCase(null); }}>
              📋 Cases
            </button>
            <button className={`ops-nav-btn${screen === 'alerts' ? ' active' : ''}`}
              onClick={() => setScreen('alerts')}>
              🚨 Alert Review
            </button>
            <button className="ops-nav-btn ops-logout" onClick={handleLogout}>
              🔒 Logout
            </button>
          </nav>
        )}
      </header>

      <main className="ops-main">
        {errorMsg && (
          <div className="ops-error">⚠️ {errorMsg}</div>
        )}

        {/* ── LOGIN ── */}
        {screen === 'login' && (
          <div className="ops-card ops-login-card">
            <h1 className="ops-title">Staff Sign In</h1>
            <p className="ops-subtitle">Authorized responders and supervisors only.</p>
            {authMode === null && !errorMsg && <p role="status">Loading sign-in…</p>}
            {authMode === 'oidc' && (
              <button type="button" className="ops-btn-primary" onClick={() => {
                void signInWithOrganization().catch(() => setErrorMsg('Organization sign-in is unavailable. Please try again.'));
              }}>Sign in with organization</button>
            )}
            {authMode === 'demo' && <p className="ops-subtitle">Demo accounts · fictional cases only</p>}
            {authMode === 'demo' && <form onSubmit={handleLogin} className="ops-form">
              <label htmlFor="username" className="ops-label">Username</label>
              <input id="username" type="text" className="ops-input" value={username}
                onChange={e => setUsername(e.target.value)} autoComplete="username" required />
              <label htmlFor="password" className="ops-label">Password</label>
              <input id="password" type="password" className="ops-input" value={password}
                onChange={e => setPassword(e.target.value)} autoComplete="current-password" required />
              <button id="login-submit" type="submit" className="ops-btn-primary" disabled={loading}>
                {loading ? 'Signing in…' : 'Sign In →'}
              </button>
            </form>}
          </div>
        )}

        {/* ── CASE QUEUE ── */}
        {screen === 'cases' && (
          <div>
            <div className="ops-page-header">
              <h2 className="ops-title">Case Queue</h2>
              <button className="ops-btn-secondary" onClick={fetchCases} disabled={loading}>
                {loading ? '…' : '↻ Refresh'}
              </button>
            </div>
            {cases.length === 0 && !loading && (
              <div className="ops-empty">No cases in queue for your organization.</div>
            )}
            <div className="ops-case-list">
              {cases.map(c => (
                <div key={c.id} className="ops-case-row" onClick={() => openCase(c)} id={`case-${c.id}`}>
                  <div className="ops-case-meta">
                    <span className="ops-case-id">{c.id.slice(0, 8)}…</span>
                    <span className="ops-badge" style={{ background: statusColor(c.status) }}>{c.status}</span>
                    {c.conflict_flag && <span className="ops-badge ops-badge-warn">⚠️ {c.conflict_flag}</span>}
                  </div>
                  <div className="ops-case-sub">
                    <span>{c.route_type}</span>
                    <span className="ops-muted">{new Date(c.created_at).toLocaleString()}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* ── CASE DETAIL ── */}
        {screen === 'case_detail' && selectedCase && (
          <div>
            <button className="ops-btn-back" onClick={() => { setScreen('cases'); setSelectedCase(null); }}>
              ← Back to Queue
            </button>
            <div className="ops-card">
              <div className="ops-detail-header">
                <div>
                  <h2 className="ops-title">Case {selectedCase.id.slice(0, 8)}…</h2>
                  <p className="ops-subtitle">{selectedCase.route_type}</p>
                </div>
                <span className="ops-badge" style={{ background: statusColor(selectedCase.status) }}>
                  {selectedCase.status}
                </span>
              </div>

              {/* Assignments */}
              <section className="ops-section">
                <h3 className="ops-section-title">Assignment</h3>
                {assignments.length === 0 ? (
                  <p className="ops-muted">No assignment yet.</p>
                ) : assignments.map(a => (
                  <div key={a.id} className="ops-assignment-row" id={`assignment-${a.id}`}>
                    <div>
                      <span className="ops-badge">{a.status}</span>
                      <span className="ops-muted" style={{ marginLeft: 8 }}>
                        Assigned {new Date(a.assigned_at).toLocaleString()}
                      </span>
                    </div>
                    {!a.accepted_at && (
                      <button className="ops-btn-primary ops-btn-sm" id={`accept-${a.id}`}
                        onClick={() => handleAccept(a.id)} disabled={loading}>
                        ✓ Explicitly Accept Case
                      </button>
                    )}
                    {a.accepted_at && (
                      <span className="ops-badge ops-badge-green">
                        ✓ Accepted {new Date(a.accepted_at).toLocaleString()}
                      </span>
                    )}
                  </div>
                ))}
              </section>

              {/* Message thread */}
              <section className="ops-section">
                <h3 className="ops-section-title">Messages</h3>
                <div className="ops-message-thread" id="message-thread">
                  {messages.length === 0 && <p className="ops-muted">No messages yet.</p>}
                  {messages.map(m => (
                    <div key={m.id} className={`ops-message ${m.sender_type === 'responder' ? 'ops-msg-staff' : 'ops-msg-reporter'} ${m.is_staff_note ? 'ops-msg-note' : ''}`}>
                      <div className="ops-msg-meta">
                        <span className="ops-msg-sender">
                          {m.is_staff_note ? '🔒 Staff Note' : m.sender_type === 'responder' ? '👤 Staff' : '🧒 Reporter'}
                        </span>
                        <span className="ops-muted">{new Date(m.sent_at).toLocaleString()}</span>
                      </div>
                      <div className="ops-msg-body">{m.body}</div>
                    </div>
                  ))}
                </div>
                <div className="ops-compose">
                  <textarea id="message-compose" className="ops-input ops-textarea" rows={3}
                    placeholder="Write a message to the reporter, or a private staff note…"
                    value={newMessage} onChange={e => setNewMessage(e.target.value)} />
                  <div className="ops-compose-actions">
                    <label className="ops-checkbox-label">
                      <input type="checkbox" id="staff-note-toggle"
                        checked={isStaffNote} onChange={e => setIsStaffNote(e.target.checked)} />
                      <span>Private staff note (not visible to reporter)</span>
                    </label>
                    <button id="send-message" className="ops-btn-primary ops-btn-sm"
                      onClick={handleSendMessage} disabled={loading || !newMessage.trim()}>
                      {loading ? '…' : isStaffNote ? '🔒 Save Note' : '📨 Send to Reporter'}
                    </button>
                  </div>
                </div>
              </section>
            </div>
          </div>
        )}

        {/* ── ALERT REVIEW ── */}
        {screen === 'alerts' && (
          <div>
            <div className="ops-page-header">
              <h2 className="ops-title">Alert Review</h2>
              <button className="ops-btn-secondary" onClick={fetchAlertDrafts} disabled={loading}>
                {loading ? '…' : '↻ Refresh'}
              </button>
            </div>
            <p className="ops-subtitle">
              You are reviewing alert revisions. Approval is binding and immutable —
              any field change after approval creates a new revision.
            </p>
            {alertDrafts.length === 0 && !loading && (
              <div className="ops-empty">No alert revisions pending review.</div>
            )}
            {alertDrafts.map(d => (
              <div key={d.id} className="ops-card ops-alert-card" id={`alert-revision-${d.id}`}>
                <div className="ops-detail-header">
                  <div>
                    <h3 className="ops-title">{d.issuer_name}</h3>
                    <span className="ops-badge">{d.status}</span>
                  </div>
                  <span className="ops-muted">Expires {new Date(d.expiry_at).toLocaleString()}</span>
                </div>
                <p className="ops-alert-body">{d.description_text}</p>
                <div className="ops-alert-actions">
                  <button id={`approve-${d.id}`} className="ops-btn-primary"
                    onClick={() => handleApprove(d.id)} disabled={loading}>
                    ✓ Approve Revision
                  </button>
                  <span className="ops-muted ops-note">
                    Two-person rule: you cannot approve a revision you prepared.
                  </span>
                </div>
              </div>
            ))}
          </div>
        )}
      </main>
    </div>
  );
}

export default App;
