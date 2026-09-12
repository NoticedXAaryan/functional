import React, { useState, useEffect, useRef } from 'react';
import './App.css';

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080/api/v1';

interface PrivateSession {
 session_id: string;
 return_code?: string;
  session_token: string;
  expires_at: string;
  }

interface CaseReceipt {
  case_id: string;
  receipt_id: string;
  status: string;
  received_at: string;
}

interface CaseSafeView {
  safe_progress_message?: string;
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
  const [returnCode, setReturnCode] = useState('');
 const [invitation,setInvitation]=useState('');
 const [service,setService]=useState<{mode:string;invitation_required:boolean;intake_open:boolean;assessment_available:boolean}|null>(null);
 const [entryID,setEntryID]=useState<string|null>(null);
 const [entryReady,setEntryReady]=useState(false);
 useEffect(()=>{ fetch(`${API_BASE}/config`).then(r=>{if(!r.ok)throw new Error('Service unavailable');return r.json()}).then(setService).catch(()=>setErrorMsg('The service is unavailable. Please try again later.'));
 const entry=new URLSearchParams(window.location.search).get('entry');
 if(!entry){setEntryReady(true);return}
 fetch(`${API_BASE}/entry-points/${encodeURIComponent(entry)}`).then(r=>{if(!r.ok)throw new Error('This support link is unavailable.');return r.json()}).then(data=>{setEntryID(data.id);setEntryReady(true)}).catch(e=>setErrorMsg(e.message));
 },[]);

  // Case Return Access State
  const [returnCaseView, setReturnCaseView] = useState<CaseSafeView | null>(null);
  const [messages, setMessages] = useState<Array<{ id: string; sender_type: string; body: string; sent_at: string }>>([]);
  const [reply, setReply] = useState('');

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
    if(session) void fetch(`${API_BASE}/sessions/${session.session_id}`,{method:'DELETE',headers:{Authorization:`Bearer ${session.session_token}`},keepalive:true}).catch(()=>{});
    setReturnCode(''); setReply(''); setAccountText(''); setAssessmentText(''); setSession(null);
    setReceipt(null);
    setReturnCaseView(null);
    setMessages([]);
    setAiResult(null);
    window.location.replace('https://news.google.com');
  };

  useEffect(()=>{ const clearOnRestore=(event:PageTransitionEvent)=>{if(event.persisted) window.location.reload()};window.addEventListener('pageshow',clearOnRestore);return()=>window.removeEventListener('pageshow',clearOnRestore)},[]);
  // ── Step 1: Start Private Session ────────────────────────────────────
  const handleStartSession = async (targetTab: 'intake' | 'assessment' = 'intake') => {
    if(!service?.intake_open || !entryReady){setErrorMsg('Reporting is not available through this link yet.');return}
    setLoading(true);
    setErrorMsg(null);
    try {
      const res = await fetch(`${API_BASE}/sessions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          mode: 'with_return_access',
          language,
          invitation_code:invitation,
          ...(entryID?{entry_point_id:entryID}:{}),
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        throw new Error((typeof data.error === 'string' ? data.error : data.error?.message) || 'Failed to initialize session');
      }
      setSession(data);
      submissionKey.current=null; setReceipt(null); setReturnCaseView(null); setMessages([]);
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
    if (!returnCode.trim()) {
      setErrorMsg('Enter the return code shown with your receipt');
      return;
    }

    setLoading(true);
    setErrorMsg(null);

    try {
      const res = await fetch(`${API_BASE}/session-access`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ return_code: returnCode }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error?.message || 'Return code is invalid or expired');
      }

      const newToken = data.session_token;
      const caseID = data.case_id;
      setSession({ session_id:data.session_id, session_token: newToken, expires_at: data.expires_at });

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
          <div className="brand-icon" aria-hidden="true">✿</div>
          <div>
            <div className="brand-title">Bal Suraksha</div>
            <div style={{ fontSize: '11px', color: 'var(--accent-cyan)', letterSpacing: '0.5px' }}>
              Private reports and local alerts
            </div>
          </div>
        </div>
        <nav className="workspace-links" aria-label="Other spaces"><a href="/alerts">Local alerts</a><a href={import.meta.env.VITE_OPS_URL ?? 'http://localhost:5174'}>Staff sign in</a></nav>
        <button className="btn-danger" onClick={handleQuickExit} title="Leave this page. Browser history is not erased.">
          <span>Quick exit ↗</span>
        </button>
      </header>

      {/* ── Navigation Tabs ── */}
      <nav className="nav-tabs">
        <button
          className={`nav-tab ${activeTab === 'scan' ? 'active' : ''}`}
          onClick={() => setActiveTab('scan')}
        >
          Start here
        </button>
        <button
          className={`nav-tab ${activeTab === 'intake' ? 'active' : ''}`}
          onClick={() => {
            if (!session) handleStartSession('intake');
            else setActiveTab('intake');
          }}
        >
          Ask for help
        </button>
        <button
          className={`nav-tab ${activeTab === 'receipt' ? 'active' : ''}`}
          onClick={() => setActiveTab('receipt')}
          disabled={!receipt}
        >
          Your receipt
        </button>
        <button
          className={`nav-tab ${activeTab === 'return' ? 'active' : ''}`}
          onClick={() => setActiveTab('return')}
        >
          Check a report
        </button>
      </nav>

      {/* ── Error Banner ── */}
      {errorMsg && (
        <div role="alert" className="glass-panel" style={{ padding: '12px 16px', marginBottom: '20px', borderColor: 'var(--accent-rose)', color: 'var(--accent-rose)' }}>
          ⚠️ {errorMsg}
        </div>
      )}

      {/* ── TAB 1: QR LANDING ── */}
      {activeTab === 'scan' && (
        <main className="glass-panel welcome">
          <p className="welcome-kicker">Help for children and young people</p>
          <h1>What would you like to do?</h1>
          <p className="welcome-lead">Send a private report, check a report you already sent, or read missing-child alerts in your area.</p>
          {service?.invitation_required && <div className="form-group"><label htmlFor="beta-invite" className="form-label">Your organization's invitation code</label><input id="beta-invite" type="password" className="glass-input" autoComplete="off" value={invitation} onChange={e=>setInvitation(e.target.value)}/><small>Needed to start a report during the limited beta.</small></div>}
          <div className="choice-grid">
            <button className="choice choice-help" disabled={loading || !service?.intake_open || !entryReady} onClick={() => session && !receipt && !returnCaseView ? setActiveTab('intake') : void handleStartSession('intake')}>
              <strong>Ask for help</strong><span>For yourself, someone you care about, or a missing child.</span><em>{loading ? 'Opening…' : 'Start a private report →'}</em>
            </button>
            <button className="choice choice-understand" disabled={loading || !service?.intake_open || !entryReady} onClick={() => { setRouteType('worried_about_someone'); if(session && !receipt && !returnCaseView) setActiveTab('intake'); else void handleStartSession('intake'); }}>
              <strong>Worried about someone?</strong><span>Tell the support team about a friend or child who may need help.</span><em>Share your concern →</em>
            </button>
            <button className="choice choice-return" onClick={() => setActiveTab('return')}>
              <strong>Check your report</strong><span>Already sent a report? Enter the private return code you received to check for replies.</span><em>Open your report →</em>
            </button>
            <a className="choice choice-alerts" href="/alerts"><strong>Local alerts</strong><span>Read Savera alerts for your area and share a private tip.</span><em>View community alerts →</em></a>
          </div>
          <div className="form-group">
            <label htmlFor="support-language" className="form-label">Preferred support language</label>
            <select id="support-language" className="glass-input" value={language} onChange={e => setLanguage(e.target.value)} style={{ maxWidth: 300 }}>
              <option value="en">English</option><option value="hi">हिंदी (Hindi)</option><option value="mr">मराठी (Marathi)</option><option value="ta">தமிழ் (Tamil)</option>
            </select>
            <small>Screens are currently in English.</small>
          </div>
          <details className="getting-started"><summary>How does a private report work?</summary>
            <ol><li>Describe what happened. You do not need an account.</li><li>Review and send it. Your receipt confirms the report was saved.</li><li>Keep your return code privately to check for replies.</li></ol>
            <p>Quick exit leaves the page. It does not erase browser history or a submitted report.</p>
          </details>
          {service?.mode === 'demo' ? <p className="preview-note">Local preview · use fictional details only.</p> : <p className="preview-note">Limited beta · reports go to your configured support organization. This is not an emergency dispatch service.</p>}
        </main>
      )}

      {/* ── TAB 2: AI ASSESSMENT ── */}
      {activeTab === 'assessment' && (
        <main className="glass-panel" style={{ padding: '32px 28px' }}>
          <div className="badge badge-cyan" style={{ marginBottom: '12px' }}>
            Guidance preview · simulated
          </div>
          <h2 className="section-title">Is this situation okay?</h2>
          <p className="section-desc" style={{ marginBottom: '20px' }}>
            Try a fictional example to explore the guidance format. This demonstration cannot decide whether a real situation is safe. You can ask for help without using it.
          </p>

          <div className="form-group">
            <textarea
              aria-label="Situation to explore"
              className="glass-input"
              rows={4}
              placeholder="Example: A teacher asked me to stay back alone after class and take pictures without telling my parents..."
              value={assessmentText}
              onChange={(e) => setAssessmentText(e.target.value)}
            />
          </div>

          <div style={{ marginBottom: '24px' }}>
            <button className="btn-primary" onClick={handleRunAssessment} disabled={loading || !assessmentText.trim()}>
              {loading ? 'Loading...' : 'Show example guidance'}
            </button>
            <button className="nav-tab" onClick={() => setActiveTab('intake')}>Skip this and ask for help →</button>
          </div>

          {/* AI Result Presentation */}
          {aiResult && (
            <div style={{ background: '#f0f4f9', border: '1px solid var(--accent-cyan)', borderRadius: 'var(--radius-md)', padding: '24px', textAlign: 'left' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px' }}>
                <span className="badge badge-emerald">You can also send a report</span>
                {aiResult.is_simulated && <span className="badge badge-amber">Example response</span>}
              </div>

              {aiResult.observed_behaviors && aiResult.observed_behaviors.length > 0 && (
                <div style={{ marginBottom: '16px' }}>
                  <div style={{ fontSize: '12px', color: 'var(--accent-cyan)', fontWeight: 700, marginBottom: '6px' }}>What the example highlights</div>
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
                  <div style={{ fontSize: '12px', color: 'var(--accent-teal)', fontWeight: 700, marginBottom: '6px' }}>Possible next steps</div>
                  <ul style={{ paddingLeft: '20px', fontSize: '14px' }}>
                    {aiResult.suggested_options.map((opt, idx) => (
                      <li key={idx} style={{ marginBottom: '4px', fontWeight: 500 }}>{opt}</li>
                    ))}
                  </ul>
                </div>
              )}

              <div style={{ textAlign: 'right', marginTop: '16px' }}>
                <button className="btn-primary" onClick={handleUseAssessmentInReport}>
                  Use my words in a support request →
                </button>
              </div>
            </div>
          )}
        </main>
      )}

      {/* ── TAB 3: INTAKE FORM ── */}
      {activeTab === 'intake' && (
        <main className="glass-panel" style={{ padding: '32px 28px' }}>
          <div className="form-heading" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
            <div>
              <h2 className="section-title">Ask for help</h2>
              <p className="section-desc" style={{ margin: 0 }}>{service?.mode === 'demo' ? 'Use fictional details in this local preview.' : 'Submitting saves your report for your support organization.'}</p>
            </div>
            <span className="badge badge-emerald">Report open</span>
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
                <div className="route-title">🚨 Report a missing child</div>
                <div className="route-subtitle">Send a private report. Staff review is required before any public alert.</div>
              </div>
            </div>
          </div>

          <div className="form-group">
            <label htmlFor="report-details" className="form-label">Describe what happened or what help is needed *</label>
            <textarea
              id="report-details"
              className="glass-input"
              rows={5}
              placeholder="Tell us as much or as little as you feel comfortable sharing..."
              value={accountText}
              onChange={(e) => setAccountText(e.target.value)}
            />
          </div>

          <div className="form-group">
            <label htmlFor="report-conflict" className="form-label">Are any local authorities or institutions involved?</label>
            <select
              id="report-conflict"
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

          <p className="section-desc">Your choice controls in-app replies. No phone, email or push contact is sent from this report.</p>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '16px' }} className="form-group">
            <div>
              <label htmlFor="contact-channel" className="form-label">How would you like to hear from us?</label>
              <select
                id="contact-channel"
                className="glass-input"
                value={preferredChannel}
                onChange={(e) => setPreferredChannel(e.target.value as any)}
              >
                <option value="message_in_app">In-App Confidential Messages Only</option>
                <option value="no_contact">Do Not Contact (Record Only)</option>
              </select>
            </div>
            <div>
              <label className="form-label">When may staff reply? (India time)</label>
              <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                <input
                  aria-label="Safe contact start time in India"
                  type="time"
                  className="glass-input"
                  value={safeHoursStart}
                  onChange={(e) => setSafeHoursStart(e.target.value)}
                />
                <span>to</span>
                <input
                  aria-label="Safe contact end time in India"
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
              Review my report →
            </button>
          </div>

          {showPreview && (
            <div className="modal-overlay">
              <div className="modal-card">
                <h3 style={{ marginBottom: '12px' }}>Review your report</h3>
                <div style={{ background: '#f0f4f9', padding: '14px', borderRadius: 'var(--radius-md)', marginBottom: '16px' }}>
                  <div style={{ fontSize: '12px', color: 'var(--accent-cyan)', fontWeight: 600 }}>What this report is about</div>
                  <div style={{ fontSize: '14px', fontWeight: 600, marginBottom: '8px' }}>{routeType}</div>
                  <div style={{ fontSize: '12px', color: 'var(--accent-cyan)', fontWeight: 600 }}>Your message</div>
                  <div style={{ fontSize: '14px', color: 'var(--text-primary)', whiteSpace: 'pre-wrap' }}>{accountText}</div>
                </div>

                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px' }}>
                  <button className="btn-tab" style={{ padding: '8px 16px', background: 'transparent', border: '1px solid var(--border-color)', color: 'var(--text-primary)', borderRadius: 'var(--radius-md)', cursor: 'pointer' }} onClick={() => setShowPreview(false)}>
                    Keep editing
                  </button>
                  <button className="btn-primary" onClick={handleSubmitCase} disabled={loading}>
                    {loading ? 'Submitting...' : 'Send report'}
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
          <h2 className="section-title">Your report is saved</h2>
          <p className="section-desc" style={{ maxWidth: '560px', margin: '0 auto 20px' }}>
            Your report has been received. This does not mean a responder has accepted it yet.
          </p>

          <div className="receipt-meta" style={{ display: 'flex', justifyContent: 'center', gap: '20px', marginBottom: '24px' }}>
            <div style={{ background: '#f0f4f9', border: '1px solid var(--border-color)', padding: '12px 20px', borderRadius: 'var(--radius-md)' }}>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>RECEIPT ID</div>
              <div style={{ fontFamily: 'monospace', fontWeight: 700, color: 'var(--accent-cyan)' }}>{receipt.receipt_id}</div>
            </div>
            <div style={{ background: '#f0f4f9', border: '1px solid var(--border-color)', padding: '12px 20px', borderRadius: 'var(--radius-md)' }}>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>STATUS</div>
              <div style={{ fontWeight: 700, color: 'var(--accent-emerald)' }}>{receipt.status}</div>
            </div>
          </div>

          {session?.return_code && <section className="getting-started"><h3>Your private return code</h3><p>Save this code somewhere only you can access. It opens your report for 30 days. If lost, it cannot be recovered.</p><code style={{display:'block',overflowWrap:'anywhere',padding:'16px 0',userSelect:'all'}}>{session.return_code.match(/.{1,16}/g)?.join('-')}</code><p>Your receipt number alone cannot open your report.</p></section>}

          <button className="btn-primary" onClick={() => setActiveTab('return')}>
            Check your report →
          </button>
        </main>
      )}

      {/* ── TAB 5: RETURN ACCESS ── */}
      {activeTab === 'return' && (
        <main className="glass-panel" style={{ padding: '32px 28px' }}>
          <h2 className="section-title" style={{ textAlign: 'center' }}>Check your report</h2>
          {!returnCaseView ? (
            <form onSubmit={handleReturnAccess} style={{ maxWidth: '580px', margin: '0 auto' }}>
              <label htmlFor="return-code" className="form-label">Private return code</label><textarea id="return-code" className="glass-input" value={returnCode} onChange={e=>setReturnCode(e.target.value)} autoComplete="off" spellCheck={false} rows={3} placeholder="Paste the code shown on your receipt"/>
              <p className="section-desc">Use the long code from your receipt. Older four-word preview codes no longer work.</p>
              <div style={{ textAlign: 'center', marginTop: '20px' }}>
                <button type="submit" className="btn-primary" disabled={loading}>
                  {loading ? 'Opening…' : 'Open my report →'}
                </button>
              </div>
            </form>
          ) : (
            <div style={{ maxWidth: '680px', margin: '0 auto' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', background: '#f0f4f9', padding: '16px 20px', borderRadius: 'var(--radius-md)', border: '1px solid var(--border-highlight)', marginBottom: '20px' }}>
                <div>
                  <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>CASE ID</div>
                  <div style={{ fontFamily: 'monospace', fontWeight: 600 }}>{returnCaseView.case_id}</div>
                </div>
                <span className="badge badge-cyan">{returnCaseView.status}</span>
              </div>
              <section aria-label="Messages from the support team">
                <p>{returnCaseView.safe_progress_message}</p>
                <h3>Messages</h3>
                {messages.length === 0 && <p>No messages yet.</p>}
                {messages.map(message => <article key={message.id} style={{ padding: '16px', margin: '12px 0', border: '1px solid var(--border-color)', borderRadius: '8px' }}><p style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{message.body}</p><small>{new Date(message.sent_at).toLocaleString()}</small></article>)}
              </section>
              {returnCaseView.status !== 'CLOSED' && <form onSubmit={async e => {
                e.preventDefault(); if(!session || !reply.trim()) return; setLoading(true); setErrorMsg(null);
                try {
                  const res=await fetch(`${API_BASE}/cases/${returnCaseView.case_id}/messages`,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${session.session_token}`},body:JSON.stringify({body:reply.trim()})});
                  const data=await res.json(); if(!res.ok) throw new Error(data.error || 'Message was not confirmed');
                  setReply('');
                  const refreshed=await fetch(`${API_BASE}/cases/${returnCaseView.case_id}/messages`,{headers:{Authorization:`Bearer ${session.session_token}`}});
                  if(!refreshed.ok) throw new Error('Your message was saved, but replies could not be refreshed. Reopen your report.');
                  setMessages((await refreshed.json()).messages || []);
                } catch(error) {setErrorMsg((error as Error).message)} finally {setLoading(false)}
              }}><label htmlFor="report-reply" className="form-label">Add a message</label><textarea id="report-reply" className="glass-input" rows={3} maxLength={5000} value={reply} onChange={e=>setReply(e.target.value)}/><button className="btn-primary" disabled={loading || !reply.trim()}>{loading ? 'Sending…' : 'Send message'}</button></form>}
            </div>
          )}
        </main>
      )}
    </div>
  );
}

export default App;
