import React, { useState, useEffect, useRef } from 'react';
import './App.css';

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080/api/v1';

interface PrivateSession {
  session_token: string;
  expires_at: string;
  return_secret_words?: string[];
}

interface CaseReceipt {
  case_id: string;
  receipt_id: string;
  status: string;
  received_at: string;
}

interface CaseSafeView {
  case_id: string;
  status: string;
  created_at: string;
  has_unread_messages: boolean;
}

interface AIAssessmentResult {
  status: string;
  human_help_available: boolean;
  observed_behaviors?: string[];
  uncertainty_statement?: string;
  clarifying_questions?: string[];
  suggested_options?: string[];
  provider: string;
  is_simulated: boolean;
}

function App() {
  const submissionKey = useRef<string | null>(null);
  const [activeTab, setActiveTab] = useState<'scan' | 'assessment' | 'intake' | 'receipt' | 'return'>('scan');
  
  // Session & Auth state
  const [session, setSession] = useState<PrivateSession | null>(null);
  const [language, setLanguage] = useState('en');
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  // AI Assessment State
  const [assessmentText, setAssessmentText] = useState('');
  const [aiResult, setAiResult] = useState<AIAssessmentResult | null>(null);

  // Form State
  const [routeType, setRouteType] = useState<'ask_for_help' | 'worried_about_someone' | 'missing_child'>('ask_for_help');
  const [accountText, setAccountText] = useState('');
  const [conflictFlag, setConflictFlag] = useState<string>('');
  const [preferredChannel, setPreferredChannel] = useState<'message_in_app' | 'no_contact'>('message_in_app');
  const [safeHoursStart, setSafeHoursStart] = useState('09:00');
  const [safeHoursEnd, setSafeHoursEnd] = useState('17:00');
  const [showPreview, setShowPreview] = useState(false);

  // Receipt & Case State
  const [receipt, setReceipt] = useState<CaseReceipt | null>(null);
  const [returnSecretWords, setReturnSecretWords] = useState<string[]>(['', '', '', '']);

  // Case Return Access State
  const [returnCaseView, setReturnCaseView] = useState<CaseSafeView | null>(null);
  const [messages, setMessages] = useState<Array<{ id: string; sender_type: string; body: string; sent_at: string }>>([]);

  // ── Auto Inactivity Teardown (P2 Privacy Guarantee) ───────────────────
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout>;
    const resetTimer = () => {
      clearTimeout(timer);
      // 15-minute auto teardown on inactivity
      timer = setTimeout(() => {
        if (session) {
          handleQuickExit();
        }
      }, 15 * 60 * 1000);
    };

    window.addEventListener('mousemove', resetTimer);
    window.addEventListener('keydown', resetTimer);
    resetTimer();

    return () => {
      clearTimeout(timer);
      window.removeEventListener('mousemove', resetTimer);
      window.removeEventListener('keydown', resetTimer);
    };
  }, [session]);

  // ── Service Worker Registration (Web Push / VAPID) ────────────────────
  useEffect(() => {
    if (!('serviceWorker' in navigator)) return;
    const register = async () => {
      try {
        const reg = await navigator.serviceWorker.register('/sw.js', { scope: '/' });
        console.info('[sw] Registered:', reg.scope);
        // Listen for subscription rotation — client must re-subscribe
        navigator.serviceWorker.addEventListener('message', (event) => {
          if (event.data?.type === 'PUSH_SUBSCRIPTION_CHANGED') {
            console.warn('[sw] Push subscription changed — please re-subscribe for alerts.');
          }
        });
      } catch (err) {
        console.warn('[sw] Registration failed (non-critical):', err);
      }
    };
    void register();
  }, []);

  // ── Incognito Quick Exit ──────────────────────────────────────────────
  const handleQuickExit = () => {
    setSession(null);
    setReceipt(null);
    setReturnCaseView(null);
    setMessages([]);
    setAiResult(null);
    window.location.replace('https://news.google.com');
  };

  // ── Step 1: Start Private Session ────────────────────────────────────
  const handleStartSession = async (targetTab: 'intake' | 'assessment' = 'intake') => {
    setLoading(true);
    setErrorMsg(null);
    try {
      const res = await fetch(`${API_BASE}/sessions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          mode: 'with_return_access',
          language,
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error?.message || 'Failed to initialize session');
      }
      setSession(data);
      setActiveTab(targetTab);
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  // ── Step 2: Bounded AI Assessment ("Is this okay?") ───────────────────
  const handleRunAssessment = async () => {
    if (!session) {
      setErrorMsg('No active private session found.');
      return;
    }
    if (!assessmentText.trim()) {
      setErrorMsg('Please describe what happened to analyze.');
      return;
    }

    setLoading(true);
    setErrorMsg(null);

    try {
      const res = await fetch(`${API_BASE}/assessments`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${session.session_token}`,
        },
        body: JSON.stringify({
          selected_text: assessmentText,
          language,
        }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Assessment service error');
      }
      setAiResult(data);
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleUseAssessmentInReport = () => {
    setAccountText(assessmentText);
    setActiveTab('intake');
  };

  // ── Step 3: Submit Case ────────────────────────────────────────────────
  const handleSubmitCase = async () => {
    if (!session) {
      setErrorMsg('No active private session found. Please restart.');
      return;
    }
    if (!accountText.trim()) {
      setErrorMsg('Please enter details about your situation.');
      return;
    }

    setLoading(true);
    setErrorMsg(null);
    setShowPreview(false);

    submissionKey.current ??= crypto.randomUUID();
    const idempotencyKey = submissionKey.current;

    try {
      const res = await fetch(`${API_BASE}/cases`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${session.session_token}`,
          'Idempotency-Key': idempotencyKey,
        },
        body: JSON.stringify({
          account_text: accountText,
          route_type: routeType,
          conflict_flag: conflictFlag ? conflictFlag : null,
          safe_contact_preference: {
            preferred_channel: preferredChannel,
            safe_hours_start: safeHoursStart,
            safe_hours_end: safeHoursEnd,
          },
        }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error?.message || 'Failed to submit intake report');
      }

      setReceipt(data);
      setActiveTab('receipt');
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  // ── Step 4: Return Access by Secret ──────────────────────────────────
  const handleReturnAccess = async (e: React.FormEvent) => {
    e.preventDefault();
    if (returnSecretWords.some(w => !w.trim())) {
      setErrorMsg('Please enter all 4 return secret words');
      return;
    }

    setLoading(true);
    setErrorMsg(null);

    try {
      const res = await fetch(`${API_BASE}/session-access`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ words: returnSecretWords.map(w => w.trim().toLowerCase()) }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error?.message || 'Invalid return words or access expired');
      }

      const newToken = data.session_token;
      const caseID = data.case_id;
      setSession({ session_token: newToken, expires_at: data.expires_at });

      const caseRes = await fetch(`${API_BASE}/cases/${caseID}/safe-view`, {
        headers: { 'Authorization': `Bearer ${newToken}` },
      });
      const caseData = await caseRes.json();
      if (caseRes.ok) {
        setReturnCaseView(caseData);
      }

      const msgRes = await fetch(`${API_BASE}/cases/${caseID}/messages`, {
        headers: { 'Authorization': `Bearer ${newToken}` },
      });
      if (msgRes.ok) {
        const msgData = await msgRes.json();
        setMessages(msgData.messages || []);
      }

    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="app-container">
      {/* ── Top Bar ── */}
      <header className="top-bar">
        <div className="brand">
          <div className="brand-icon">🛡️</div>
          <div>
            <div className="brand-title">Bal Suraksha</div>
            <div style={{ fontSize: '11px', color: 'var(--accent-cyan)', letterSpacing: '0.5px' }}>
              SAFE & ANONYMOUS CHILD INTAKE
            </div>
          </div>
        </div>
        <button className="btn-danger" onClick={handleQuickExit} title="Instantly clear session and exit">
          <span>⚡ QUICK EXIT</span>
        </button>
      </header>

      {/* ── Navigation Tabs ── */}
      <nav className="nav-tabs">
        <button
          className={`nav-tab ${activeTab === 'scan' ? 'active' : ''}`}
          onClick={() => setActiveTab('scan')}
        >
          1. QR Landing
        </button>
        <button
          className={`nav-tab ${activeTab === 'assessment' ? 'active' : ''}`}
          onClick={() => {
            if (!session) handleStartSession('assessment');
            else setActiveTab('assessment');
          }}
        >
          2. AI Assessment ("Is this okay?")
        </button>
        <button
          className={`nav-tab ${activeTab === 'intake' ? 'active' : ''}`}
          onClick={() => {
            if (!session) handleStartSession('intake');
            else setActiveTab('intake');
          }}
        >
          3. Safe Intake
        </button>
        <button
          className={`nav-tab ${activeTab === 'receipt' ? 'active' : ''}`}
          onClick={() => setActiveTab('receipt')}
          disabled={!receipt}
        >
          4. Receipt
        </button>
        <button
          className={`nav-tab ${activeTab === 'return' ? 'active' : ''}`}
          onClick={() => setActiveTab('return')}
        >
          5. Return Access
        </button>
      </nav>

      {/* ── Error Banner ── */}
      {errorMsg && (
        <div className="glass-panel" style={{ padding: '12px 16px', marginBottom: '20px', borderColor: 'var(--accent-rose)', color: 'var(--accent-rose)' }}>
          ⚠️ {errorMsg}
        </div>
      )}

      {/* ── TAB 1: QR LANDING ── */}
      {activeTab === 'scan' && (
        <main className="glass-panel" style={{ padding: '36px 28px', textAlign: 'center' }}>
          <div className="badge badge-cyan" style={{ marginBottom: '16px' }}>
            PRIVATE INTAKE ACCESS POINT
          </div>
          <h1 className="section-title">Scan QR or Start Private Intake</h1>
          <p className="section-desc" style={{ maxWidth: '560px', margin: '0 auto 28px' }}>
            Ask for help without creating an account. A submitted report is retained for the support team. Quick exit does not erase browser history or copies saved by your device.
          </p>

          <div style={{ background: 'rgba(15,23,42,0.8)', border: '1px dashed var(--border-highlight)', borderRadius: 'var(--radius-md)', padding: '24px', maxWidth: '340px', margin: '0 auto 28px' }}>
            <div style={{ fontSize: '48px', marginBottom: '8px' }}>📱</div>
            <div style={{ fontWeight: 600, fontSize: '14px', color: 'var(--text-primary)' }}>Demo entry point</div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>Fictional placement. A real venue has not been verified here.</div>
          </div>

          <div className="form-group" style={{ maxWidth: '340px', margin: '0 auto 24px' }}>
            <label className="form-label">Preferred Language</label>
            <select
              className="glass-input"
              value={language}
              onChange={(e) => setLanguage(e.target.value)}
            >
              <option value="en">English</option>
              <option value="hi">हिंदी (Hindi)</option>
              <option value="mr">मराठी (Marathi)</option>
              <option value="ta">தமிழ் (Tamil)</option>
            </select>
          </div>

          <div style={{ display: 'flex', justifyContent: 'center', gap: '12px' }}>
            <button className="btn-primary" onClick={() => handleStartSession('intake')} disabled={loading}>
              {loading ? 'Starting...' : 'Direct Intake Form →'}
            </button>
            <button className="btn-tab" style={{ padding: '12px 20px', background: 'rgba(6, 182, 212, 0.15)', border: '1px solid var(--accent-cyan)', color: 'var(--accent-cyan)', borderRadius: 'var(--radius-md)', fontWeight: 600, cursor: 'pointer' }} onClick={() => handleStartSession('assessment')} disabled={loading}>
              🤖 Try "Is this okay?" AI Tool
            </button>
          </div>
        </main>
      )}

      {/* ── TAB 2: AI ASSESSMENT ── */}
      {activeTab === 'assessment' && (
        <main className="glass-panel" style={{ padding: '32px 28px' }}>
          <div className="badge badge-cyan" style={{ marginBottom: '12px' }}>
            BOUNDED AI HELP ASSISTANT
          </div>
          <h2 className="section-title">Is this situation okay?</h2>
          <p className="section-desc" style={{ marginBottom: '20px' }}>
            Share what happened. Our private system will highlight key patterns and suggest safe next steps.
          </p>

          <div className="form-group">
            <textarea
              className="glass-input"
              rows={4}
              placeholder="Example: A teacher asked me to stay back alone after class and take pictures without telling my parents..."
              value={assessmentText}
              onChange={(e) => setAssessmentText(e.target.value)}
            />
          </div>

          <div style={{ marginBottom: '24px' }}>
            <button className="btn-primary" onClick={handleRunAssessment} disabled={loading || !assessmentText.trim()}>
              {loading ? 'Analyzing...' : 'Analyze My Situation'}
            </button>
          </div>

          {/* AI Result Presentation */}
          {aiResult && (
            <div style={{ background: 'rgba(15, 23, 42, 0.9)', border: '1px solid var(--accent-cyan)', borderRadius: 'var(--radius-md)', padding: '24px', textAlign: 'left' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px' }}>
                <span className="badge badge-emerald">HUMAN HELP PATH ALWAYS AVAILABLE</span>
                {aiResult.is_simulated && <span className="badge badge-amber">SIMULATED ADAPTER</span>}
              </div>

              {aiResult.observed_behaviors && aiResult.observed_behaviors.length > 0 && (
                <div style={{ marginBottom: '16px' }}>
                  <div style={{ fontSize: '12px', color: 'var(--accent-cyan)', fontWeight: 700, marginBottom: '6px' }}>OBSERVED BEHAVIORS</div>
                  <ul style={{ paddingLeft: '20px', fontSize: '14px', color: 'var(--text-primary)' }}>
                    {aiResult.observed_behaviors.map((item, idx) => (
                      <li key={idx} style={{ marginBottom: '4px' }}>{item}</li>
                    ))}
                  </ul>
                </div>
              )}

              {aiResult.uncertainty_statement && (
                <div style={{ fontSize: '13px', color: 'var(--text-secondary)', fontStyle: 'italic', marginBottom: '16px', background: 'rgba(255,255,255,0.03)', padding: '10px', borderRadius: '8px' }}>
                  ℹ️ {aiResult.uncertainty_statement}
                </div>
              )}

              {aiResult.suggested_options && (
                <div style={{ marginBottom: '20px' }}>
                  <div style={{ fontSize: '12px', color: 'var(--accent-teal)', fontWeight: 700, marginBottom: '6px' }}>RECOMMENDED SAFE ACTIONS</div>
                  <ul style={{ paddingLeft: '20px', fontSize: '14px' }}>
                    {aiResult.suggested_options.map((opt, idx) => (
                      <li key={idx} style={{ marginBottom: '4px', fontWeight: 500 }}>{opt}</li>
                    ))}
                  </ul>
                </div>
              )}

              <div style={{ textAlign: 'right', marginTop: '16px' }}>
                <button className="btn-primary" onClick={handleUseAssessmentInReport}>
                  Pre-fill into Official Intake Report →
                </button>
              </div>
            </div>
          )}
        </main>
      )}

      {/* ── TAB 3: INTAKE FORM ── */}
      {activeTab === 'intake' && (
        <main className="glass-panel" style={{ padding: '32px 28px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
            <div>
              <h2 className="section-title">Confidential Support Request</h2>
              <p className="section-desc" style={{ margin: 0 }}>Describe your situation. All information is encrypted and safe.</p>
            </div>
            <span className="badge badge-emerald">SESSION ACTIVE</span>
          </div>

          <div className="form-group">
            <label className="form-label">What brings you here today?</label>
            <div className="route-options">
              <div
                className={`route-card ${routeType === 'ask_for_help' ? 'selected' : ''}`}
                onClick={() => setRouteType('ask_for_help')}
              >
                <div className="route-title">✋ I need help</div>
                <div className="route-subtitle">Request direct support or protection for yourself.</div>
              </div>

              <div
                className={`route-card ${routeType === 'worried_about_someone' ? 'selected' : ''}`}
                onClick={() => setRouteType('worried_about_someone')}
              >
                <div className="route-title">👥 Worried about a friend</div>
                <div className="route-subtitle">Report concerns about a peer or family member.</div>
              </div>

              <div
                className={`route-card ${routeType === 'missing_child' ? 'selected' : ''}`}
                onClick={() => setRouteType('missing_child')}
              >
                <div className="route-title">🚨 Missing child alert</div>
                <div className="route-subtitle">Report a child who is missing or uncontactable.</div>
              </div>
            </div>
          </div>

          <div className="form-group">
            <label className="form-label">Describe what happened or what help is needed *</label>
            <textarea
              className="glass-input"
              rows={5}
              placeholder="Tell us as much or as little as you feel comfortable sharing..."
              value={accountText}
              onChange={(e) => setAccountText(e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="form-label">Are any local authorities or institutions involved?</label>
            <select
              className="glass-input"
              value={conflictFlag}
              onChange={(e) => setConflictFlag(e.target.value)}
            >
              <option value="">None / Not applicable</option>
              <option value="school_implicated">School staff or school administration implicated</option>
              <option value="caregiver_implicated">Primary caregiver or family member implicated</option>
              <option value="staff_implicated">Organization responder / officer implicated</option>
            </select>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }} className="form-group">
            <div>
              <label className="form-label">Preferred Channel</label>
              <select
                className="glass-input"
                value={preferredChannel}
                onChange={(e) => setPreferredChannel(e.target.value as any)}
              >
                <option value="message_in_app">In-App Confidential Messages Only</option>
                <option value="no_contact">Do Not Contact (Record Only)</option>
              </select>
            </div>
            <div>
              <label className="form-label">Safe Contact Hours</label>
              <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                <input
                  type="time"
                  className="glass-input"
                  value={safeHoursStart}
                  onChange={(e) => setSafeHoursStart(e.target.value)}
                />
                <span>to</span>
                <input
                  type="time"
                  className="glass-input"
                  value={safeHoursEnd}
                  onChange={(e) => setSafeHoursEnd(e.target.value)}
                />
              </div>
            </div>
          </div>

          <div style={{ textAlign: 'right', marginTop: '28px' }}>
            <button className="btn-primary" onClick={() => setShowPreview(true)} disabled={!accountText.trim()}>
              Preview & Submit →
            </button>
          </div>

          {showPreview && (
            <div className="modal-overlay">
              <div className="modal-card">
                <h3 style={{ marginBottom: '12px' }}>Review Your Information</h3>
                <div style={{ background: 'rgba(15,23,42,0.6)', padding: '14px', borderRadius: 'var(--radius-md)', marginBottom: '16px' }}>
                  <div style={{ fontSize: '12px', color: 'var(--accent-cyan)', fontWeight: 600 }}>ROUTE TYPE</div>
                  <div style={{ fontSize: '14px', fontWeight: 600, marginBottom: '8px' }}>{routeType}</div>
                  <div style={{ fontSize: '12px', color: 'var(--accent-cyan)', fontWeight: 600 }}>NARRATIVE</div>
                  <div style={{ fontSize: '14px', color: 'var(--text-primary)', whiteSpace: 'pre-wrap' }}>{accountText}</div>
                </div>

                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px' }}>
                  <button className="btn-tab" style={{ padding: '8px 16px', background: 'transparent', border: '1px solid var(--border-color)', color: 'white', borderRadius: 'var(--radius-md)', cursor: 'pointer' }} onClick={() => setShowPreview(false)}>
                    Edit Form
                  </button>
                  <button className="btn-primary" onClick={handleSubmitCase} disabled={loading}>
                    {loading ? 'Submitting...' : 'Confirm & Submit Case'}
                  </button>
                </div>
              </div>
            </div>
          )}
        </main>
      )}

      {/* ── TAB 4: DURABLE RECEIPT ── */}
      {activeTab === 'receipt' && receipt && (
        <main className="glass-panel" style={{ padding: '36px 28px', textAlign: 'center' }}>
          <div className="badge badge-emerald" style={{ marginBottom: '16px' }}>
            ✓ REPORT RECEIVED
          </div>
          <h2 className="section-title">Intake Receipt</h2>
          <p className="section-desc" style={{ maxWidth: '560px', margin: '0 auto 20px' }}>
            Your report has been received. This does not mean a responder has accepted it yet.
          </p>

          <div style={{ display: 'flex', justifyContent: 'center', gap: '20px', marginBottom: '24px' }}>
            <div style={{ background: 'rgba(15,23,42,0.7)', border: '1px solid var(--border-color)', padding: '12px 20px', borderRadius: 'var(--radius-md)' }}>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>RECEIPT ID</div>
              <div style={{ fontFamily: 'monospace', fontWeight: 700, color: 'var(--accent-cyan)' }}>{receipt.receipt_id}</div>
            </div>
            <div style={{ background: 'rgba(15,23,42,0.7)', border: '1px solid var(--border-color)', padding: '12px 20px', borderRadius: 'var(--radius-md)' }}>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>STATUS</div>
              <div style={{ fontWeight: 700, color: 'var(--accent-emerald)' }}>{receipt.status}</div>
            </div>
          </div>

          {session?.return_secret_words && (
            <div style={{ background: 'rgba(6, 182, 212, 0.05)', border: '1px solid var(--accent-cyan)', padding: '24px', borderRadius: 'var(--radius-lg)', margin: '0 auto 28px', maxWidth: '640px' }}>
              <div style={{ fontSize: '14px', fontWeight: 700, color: 'var(--accent-cyan)', marginBottom: '4px' }}>
                🔑 YOUR 4-WORD RETURN SECRET
              </div>
              <div className="mnemonic-grid">
                {session.return_secret_words.map((word, idx) => (
                  <div className="mnemonic-card" key={idx}>
                    <div className="mnemonic-num">WORD #{idx + 1}</div>
                    <div className="mnemonic-word">{word}</div>
                  </div>
                ))}
              </div>
            </div>
          )}

          <button className="btn-primary" onClick={() => setActiveTab('return')}>
            Go to Return Access Portal →
          </button>
        </main>
      )}

      {/* ── TAB 5: RETURN ACCESS ── */}
      {activeTab === 'return' && (
        <main className="glass-panel" style={{ padding: '32px 28px' }}>
          <h2 className="section-title" style={{ textAlign: 'center' }}>Return Access Portal</h2>
          {!returnCaseView ? (
            <form onSubmit={handleReturnAccess} style={{ maxWidth: '580px', margin: '0 auto' }}>
              <div className="secret-input-grid">
                {[0, 1, 2, 3].map((idx) => (
                  <input
                    key={idx}
                    type="text"
                    className="glass-input secret-word-input"
                    placeholder={`Word #${idx + 1}`}
                    value={returnSecretWords[idx]}
                    onChange={(e) => {
                      const updated = [...returnSecretWords];
                      updated[idx] = e.target.value;
                      setReturnSecretWords(updated);
                    }}
                  />
                ))}
              </div>
              <div style={{ textAlign: 'center', marginTop: '20px' }}>
                <button type="submit" className="btn-primary" disabled={loading}>
                  {loading ? 'Validating Secret...' : 'Access My Case →'}
                </button>
              </div>
            </form>
          ) : (
            <div style={{ maxWidth: '680px', margin: '0 auto' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', background: 'rgba(15,23,42,0.8)', padding: '16px 20px', borderRadius: 'var(--radius-md)', border: '1px solid var(--border-highlight)', marginBottom: '20px' }}>
                <div>
                  <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>CASE ID</div>
                  <div style={{ fontFamily: 'monospace', fontWeight: 600 }}>{returnCaseView.case_id}</div>
                </div>
                <span className="badge badge-cyan">{returnCaseView.status}</span>
              </div>
              <section aria-label="Messages from the support team">
                <h3>Messages</h3>
                {messages.length === 0 && <p>No messages yet.</p>}
                {messages.map(message => <article key={message.id} style={{ padding: '16px', margin: '12px 0', border: '1px solid var(--border-color)', borderRadius: '8px' }}><p style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{message.body}</p><small>{new Date(message.sent_at).toLocaleString()}</small></article>)}
              </section>
            </div>
          )}
        </main>
      )}
    </div>
  );
}

export default App;
