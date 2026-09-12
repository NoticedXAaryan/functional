import React, { useState, useEffect, useRef } from 'react';
import './bs-tokens.css';
import './App.css';
import { BRAND } from './brand';
import { ChildShell, ActionButton, InlineError } from './ui/Primitives';

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080/api/v1';

// ─── Types ──────────────────────────────────────────────────────────────────

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

type Screen =
  | 'home'        // C01 — Home / action picker
  | 'intent'      // C02 — Intent picker (what kind of help)
  | 'describe'    // C03 — Describe situation (text only)
  | 'consent'     // C04 — Reply consent
  | 'review'      // C05 — Review before submit
  | 'receipt'     // C06 — Durable receipt
  | 'return'      // C08 — Return access (enter long code)
  | 'conversation'// C09 — Conversation / messages
  | 'missing'     // M01-M05 — Missing child intake
  | 'other_help'  // C11 — Other help (1098 / 112)
  | 'privacy';    // C12 — Privacy summary

type IntentChoice = 'ask_for_help' | 'worried_about_someone' | 'missing_child' | 'not_sure';

// Map server status codes to human-readable strings (COMPONENTS.md)
function mapStatus(status: string): string {
  switch (status) {
    case 'not_sent':  return 'Unsent';
    case 'in_flight': return 'Sending\u2026';
    case 'committed': return 'Your message was received';
    case 'unknown':   return 'We could not confirm it yet';
    default:          return status;
  }
}

// ─── Missing intake step type ────────────────────────────────────────────────
interface MissingIntake {
  childName: string;
  age: string;
  lastSeen: string;
  lastLocation: string;
  description: string;
}

// ─── App ─────────────────────────────────────────────────────────────────────

function App() {
  const submissionKey = useRef<string | null>(null);

  // Screen navigation
  const [screen, setScreen] = useState<Screen>('home');

  // Session & Auth
  const [session, setSession] = useState<PrivateSession | null>(null);
  const [language, setLanguage] = useState('en');
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  // Service config / entry point
  const [invitation, setInvitation] = useState('');
  const [service, setService] = useState<{
    mode: string;
    invitation_required: boolean;
    intake_open: boolean;
    assessment_available: boolean;
  } | null>(null);
  const [entryID, setEntryID] = useState<string | null>(null);
  const [entryReady, setEntryReady] = useState(false);

  // C02 Intent
  const [intent, setIntent] = useState<IntentChoice | null>(null);

  // C03 Describe
  const [accountText, setAccountText] = useState('');
  const [conflictFlag, setConflictFlag] = useState('');

  // C04 Reply consent
  const [replyConsent, setReplyConsent] = useState<boolean | null>(null);
  const [preferredChannel, setPreferredChannel] = useState<'message_in_app' | 'no_contact'>('message_in_app');
  const [safeHoursStart, setSafeHoursStart] = useState('09:00');
  const [safeHoursEnd, setSafeHoursEnd] = useState('17:00');

  // C06 Receipt
  const [receipt, setReceipt] = useState<CaseReceipt | null>(null);

  // C08 Return access
  const [returnCode, setReturnCode] = useState('');
  const [returnCaseView, setReturnCaseView] = useState<CaseSafeView | null>(null);
  const [messages, setMessages] = useState<Array<{
    id: string;
    sender_type: string;
    body: string;
    sent_at: string;
  }>>([]);
  const [reply, setReply] = useState('');

  // Missing intake (M01-M05)
  const [missingStep, setMissingStep] = useState(0);
  const [missingIntake, setMissingIntake] = useState<MissingIntake>({
    childName: '', age: '', lastSeen: '', lastLocation: '', description: '',
  });
  const [missingReceipt, setMissingReceipt] = useState<CaseReceipt | null>(null);

  // ── Service config + entry point on mount ──────────────────────────────
  useEffect(() => {
    fetch(`${API_BASE}/config`)
      .then(r => { if (!r.ok) throw new Error('Service unavailable'); return r.json(); })
      .then(setService)
      .catch(() => setErrorMsg('The service is unavailable. Please try again later.'));

    const entry = new URLSearchParams(window.location.search).get('entry');
    if (!entry) { setEntryReady(true); return; }
    fetch(`${API_BASE}/entry-points/${encodeURIComponent(entry)}`)
      .then(r => { if (!r.ok) throw new Error('This support link is unavailable.'); return r.json(); })
      .then(data => { setEntryID(data.id); setEntryReady(true); })
      .catch(e => setErrorMsg(e.message));
  }, []);

  // ── Inactivity teardown — 15 min (P2 Privacy Guarantee) ──────────────
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout>;
    const resetTimer = () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        if (session) handleQuickExit();
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
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session]);

  // ── Service Worker Registration (Web Push / VAPID) ───────────────────
  useEffect(() => {
    if (!('serviceWorker' in navigator)) return;
    const register = async () => {
      try {
        const reg = await navigator.serviceWorker.register('/sw.js', { scope: '/' });
        console.info('[sw] Registered:', reg.scope);
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

  // ── pageshow — clear restored bfcache state ───────────────────────────
  useEffect(() => {
    const clearOnRestore = (event: PageTransitionEvent) => {
      if (event.persisted) window.location.reload();
    };
    window.addEventListener('pageshow', clearOnRestore);
    return () => window.removeEventListener('pageshow', clearOnRestore);
  }, []);

  // ── Quick Exit — full exit sequence ──────────────────────────────────
  const handleQuickExit = () => {
    if (session) {
      void fetch(`${API_BASE}/sessions/${session.session_id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${session.session_token}` },
        keepalive: true,
      }).catch(() => {});
    }
    setSession(null);
    setScreen('home');
    setIntent(null);
    setAccountText('');
    setConflictFlag('');
    setReplyConsent(null);
    setReturnCode('');
    setReply('');
    setReceipt(null);
    setReturnCaseView(null);
    setMessages([]);
    setMissingIntake({ childName: '', age: '', lastSeen: '', lastLocation: '', description: '' });
    setMissingStep(0);
    setMissingReceipt(null);
    setErrorMsg(null);
    submissionKey.current = null;
    window.location.replace(BRAND.exitUrl);
  };

  // ── Start private session ─────────────────────────────────────────────
  const handleStartSession = async (nextScreen: Screen = 'intent') => {
    if (!service?.intake_open || !entryReady) {
      setErrorMsg('Reporting is not available through this link yet.');
      return;
    }
    setLoading(true);
    setErrorMsg(null);
    try {
      const res = await fetch(`${API_BASE}/sessions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          mode: 'with_return_access',
          language,
          invitation_code: invitation,
          ...(entryID ? { entry_point_id: entryID } : {}),
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(
          (typeof data.error === 'string' ? data.error : data.error?.message) ||
          'Failed to initialize session'
        );
      }
      setSession(data);
      submissionKey.current = null;
      setReceipt(null);
      setReturnCaseView(null);
      setMessages([]);
      setScreen(nextScreen);
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  // ── Submit help case (C05 -> C06) ─────────────────────────────────────
  const handleSubmitCase = async () => {
    if (!session) { setErrorMsg('No active private session found. Please restart.'); return; }
    if (!intent) { setErrorMsg('Please choose the type of help you need.'); return; }

    setLoading(true);
    setErrorMsg(null);

    submissionKey.current ??= crypto.randomUUID();
    const idempotencyKey = submissionKey.current;

    try {
      const res = await fetch(`${API_BASE}/cases`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${session.session_token}`,
          'Idempotency-Key': idempotencyKey,
        },
        body: JSON.stringify({
          account_text: accountText.trim() || `[Intent: ${intent}]`,
          route_type: intent === 'not_sure' ? 'ask_for_help' : intent,
          conflict_flag: conflictFlag || null,
          in_app_reply_consent: replyConsent,
          safe_contact_preference: {
            preferred_channel: replyConsent === false ? 'no_contact' : preferredChannel,
            safe_hours_start: safeHoursStart,
            safe_hours_end: safeHoursEnd,
          },
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to submit report');
      setReceipt(data);
      setScreen('receipt');
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  // ── Return access (C08) ───────────────────────────────────────────────
  const handleReturnAccess = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!returnCode.trim()) { setErrorMsg('Enter the return code shown with your receipt'); return; }

    setLoading(true);
    setErrorMsg(null);

    try {
      const res = await fetch(`${API_BASE}/session-access`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ return_code: returnCode }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Return code is invalid or expired');

      const newToken = data.session_token;
      const caseID = data.case_id;
      setSession({ session_id: data.session_id, session_token: newToken, expires_at: data.expires_at });

      const caseRes = await fetch(`${API_BASE}/cases/${caseID}/safe-view`, {
        headers: { Authorization: `Bearer ${newToken}` },
      });
      const caseData = await caseRes.json();
      if (caseRes.ok) setReturnCaseView(caseData);

      const msgRes = await fetch(`${API_BASE}/cases/${caseID}/messages`, {
        headers: { Authorization: `Bearer ${newToken}` },
      });
      if (msgRes.ok) {
        const msgData = await msgRes.json();
        setMessages(msgData.messages || []);
      }
      setScreen('conversation');
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  // ── Send reply (C09) ──────────────────────────────────────────────────
  const handleSendReply = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!session || !reply.trim() || !returnCaseView) return;
    setLoading(true);
    setErrorMsg(null);
    try {
      const res = await fetch(`${API_BASE}/cases/${returnCaseView.case_id}/messages`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${session.session_token}`,
        },
        body: JSON.stringify({ body: reply.trim() }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Message was not confirmed');
      setReply('');
      const refreshed = await fetch(`${API_BASE}/cases/${returnCaseView.case_id}/messages`, {
        headers: { Authorization: `Bearer ${session.session_token}` },
      });
      if (!refreshed.ok)
        throw new Error('Your message was saved, but replies could not be refreshed. Reopen your report.');
      setMessages((await refreshed.json()).messages || []);
    } catch (err: any) {
      setErrorMsg((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  // ── Submit missing child report (M05) ────────────────────────────────
  const handleSubmitMissing = async () => {
    if (!session) { setErrorMsg('No active session. Please restart.'); return; }
    setLoading(true);
    setErrorMsg(null);
    const missingKey = crypto.randomUUID();
    try {
      const res = await fetch(`${API_BASE}/cases`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${session.session_token}`,
          'Idempotency-Key': missingKey,
        },
        body: JSON.stringify({
          route_type: 'missing_child',
          in_app_reply_consent: null,
          account_text: [
            `Child name: ${missingIntake.childName}`,
            `Age: ${missingIntake.age}`,
            `Last seen: ${missingIntake.lastSeen}`,
            `Last location: ${missingIntake.lastLocation}`,
            `Description: ${missingIntake.description}`,
          ].join('\n'),
          safe_contact_preference: {
            preferred_channel: 'message_in_app',
            safe_hours_start: '00:00',
            safe_hours_end: '23:59',
          },
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to submit missing report');
      setMissingReceipt(data);
      setMissingStep(5);
    } catch (err: any) {
      setErrorMsg(err.message);
    } finally {
      setLoading(false);
    }
  };

  // ── Language control ──────────────────────────────────────────────────
  const languageControl = (
    <div className="bs-field" style={{ display: 'inline-grid' }}>
      <label htmlFor="lang-select" className="bs-sr-only">Language</label>
      <select
        id="lang-select"
        className="bs-input"
        value={language}
        onChange={e => setLanguage(e.target.value)}
        style={{ minBlockSize: 'auto', padding: '6px 10px', fontSize: '0.95rem' }}
        aria-label="Support language"
      >
        <option value="en">English</option>
        <option value="hi" disabled title="Review in progress — not yet available">
          Hindi (Review in progress)
        </option>
        <option value="gu" disabled title="Review in progress — not yet available">
          Gujarati (Review in progress)
        </option>
      </select>
    </div>
  );

  // ── Footer ────────────────────────────────────────────────────────────
  const footer = (
    <>
      <a href="/alerts">{BRAND.alertLabel}</a>
      <a href={import.meta.env.VITE_OPS_URL ?? 'http://localhost:5174'}>Staff sign in</a>
      <button
        type="button"
        className="bs-button bs-button--secondary"
        style={{ fontSize: '0.95rem', minBlockSize: '44px' }}
        onClick={() => setScreen('privacy')}
      >
        Privacy
      </button>
      <button
        type="button"
        className="bs-button bs-button--secondary"
        style={{ fontSize: '0.95rem', minBlockSize: '44px' }}
        onClick={() => setScreen('other_help')}
      >
        Other help
      </button>
    </>
  );

  // ── Brand mark ────────────────────────────────────────────────────────
  const brand = (
    <>
      <img
        src="/brand/bal-setu-app-icon-master.png"
        alt=""
        width={32}
        height={32}
        style={{ borderRadius: 8, flexShrink: 0 }}
        onError={e => { (e.currentTarget as HTMLImageElement).style.display = 'none'; }}
      />
      <span style={{ fontWeight: 750, fontSize: '1.25rem', color: 'var(--bs-primary)' }}>
        {BRAND.name}
      </span>
    </>
  );

  // ── Inline error banner ───────────────────────────────────────────────
  const errorBanner = errorMsg ? (
    <InlineError
      message={errorMsg}
      retryLabel="Dismiss"
      onRetry={() => setErrorMsg(null)}
    />
  ) : null;

  // ═══════════════════════════════════════════════════════════════════════
  // C01 HOME
  // ═══════════════════════════════════════════════════════════════════════
  const renderHome = () => (
    <section className="bs-stack">
      <div>
        <p style={{
          color: 'var(--bs-teal)', fontWeight: 700, fontSize: '0.9rem',
          textTransform: 'uppercase', letterSpacing: '0.04em', margin: '0 0 8px',
        }}>
          {BRAND.name}
        </p>
        <h1 style={{ margin: '0 0 12px' }}>You can ask for help here.</h1>
        <p className="bs-help" style={{ margin: '0 0 24px' }}>
          You can type, use your voice, or choose a button.
        </p>
      </div>

      {errorBanner}

      {service?.invitation_required && (
        <div className="bs-field">
          <label htmlFor="beta-invite">Organisation invitation code</label>
          <input
            id="beta-invite"
            type="password"
            className="bs-input"
            autoComplete="off"
            value={invitation}
            onChange={e => setInvitation(e.target.value)}
          />
          <span className="bs-help">Needed during the limited beta.</span>
        </div>
      )}

      <div role="list" className="bs-stack">

        {/* Ask for help — navigates to C02 intent, which creates session on proceed */}
        <button
          type="button"
          role="listitem"
          className="bs-choice"
          style={{ background: 'var(--bs-lilac)', minBlockSize: 90 }}
          disabled={loading || !entryReady}
          onClick={() => setScreen('intent')}
          aria-pressed={undefined}
        >
          <span style={{ fontSize: '2rem' }} aria-hidden="true">&#x1F64B;</span>
          <span>
            <strong>Ask for help</strong>
            <span style={{ display: 'block', fontWeight: 400, fontSize: '0.9em', marginTop: 2 }}>
              For yourself or someone you care about
            </span>
          </span>
        </button>

        {/* My messages / return */}
        <button
          type="button"
          role="listitem"
          className="bs-choice bs-choice--sky"
          style={{ minBlockSize: 90 }}
          onClick={() => setScreen('return')}
          aria-pressed={undefined}
        >
          <span style={{ fontSize: '2rem' }} aria-hidden="true">&#x1F4AC;</span>
          <span>
            <strong>My messages</strong>
            <span style={{ display: 'block', fontWeight: 400, fontSize: '0.9em', marginTop: 2 }}>
              Check replies using your return code
            </span>
          </span>
        </button>

        {/* Someone is missing */}
        <button
          type="button"
          role="listitem"
          className="bs-choice bs-choice--peach"
          style={{ minBlockSize: 90 }}
          disabled={loading || !service?.intake_open || !entryReady}
          onClick={() => {
            setIntent('missing_child');
            setMissingStep(0);
            if (session) setScreen('missing');
            else void handleStartSession('missing');
          }}
          aria-pressed={undefined}
        >
          <span style={{ fontSize: '2rem' }} aria-hidden="true">&#x1F50D;</span>
          <span>
            <strong>Someone is missing</strong>
            <span style={{ display: 'block', fontWeight: 400, fontSize: '0.9em', marginTop: 2 }}>
              Report a missing child privately
            </span>
          </span>
        </button>

        {/* Nearby alerts */}
        <a
          role="listitem"
          className="bs-choice"
          href="/alerts"
          style={{ background: '#fce9dc', textDecoration: 'none', minBlockSize: 90 }}
        >
          <span style={{ fontSize: '2rem' }} aria-hidden="true">&#x1F4E2;</span>
          <span>
            <strong>{BRAND.alertLabel}</strong>
            <span style={{ display: 'block', fontWeight: 400, fontSize: '0.9em', marginTop: 2 }}>
              {BRAND.alertName} — read alerts near you
            </span>
          </span>
        </a>
      </div>

      {/* Demo / beta notice */}
      {service?.mode === 'demo'
        ? <p className="bs-notice" style={{ fontSize: '0.9rem' }}>Local preview — use fictional details only.</p>
        : <p className="bs-notice" style={{ fontSize: '0.9rem' }}>Reports go to your configured support organisation. This is not an emergency dispatch service.</p>
      }

      <details style={{ fontSize: '0.95rem', color: 'var(--bs-muted)' }}>
        <summary style={{ cursor: 'pointer', color: 'var(--bs-ink)', fontWeight: 600 }}>
          How does a private report work?
        </summary>
        <ol style={{ paddingLeft: '1.4em', marginTop: 10 }}>
          <li>Describe what happened. You do not need an account.</li>
          <li>Review and send. Your receipt confirms the report was saved.</li>
          <li>Keep your return code privately to check for replies.</li>
        </ol>
        <p>Quick exit leaves the page. It does not erase browser history or a submitted report.</p>
      </details>
    </section>
  );

  // ═══════════════════════════════════════════════════════════════════════
  // C02 INTENT
  // ═══════════════════════════════════════════════════════════════════════
  const renderIntent = () => (
    <section className="bs-stack">
      <div>
        <h1>What would you like help with?</h1>
        <p className="bs-help">You do not need to know what to call it.</p>
      </div>
      {errorBanner}
      <fieldset style={{ border: 'none', padding: 0, margin: 0 }}>
        <legend className="bs-sr-only">Type of help needed</legend>
        <div className="bs-stack">
          {([
            { id: 'ask_for_help' as IntentChoice, label: 'Something happened to me', desc: 'You want support or protection for yourself.' },
            { id: 'worried_about_someone' as IntentChoice, label: 'I am worried about someone', desc: 'Tell the team about a friend or family member who may need help.' },
            { id: 'not_sure' as IntentChoice, label: 'I am not sure', desc: "That's okay. You can describe what's on your mind." },
          ] as const).map(({ id, label, desc }) => (
            <button
              key={id}
              type="button"
              className="bs-choice"
              aria-pressed={intent === id}
              onClick={() => setIntent(id)}
            >
              <span>
                <strong>{label}</strong>
                <span style={{ display: 'block', fontWeight: 400, fontSize: '0.9em', marginTop: 2 }}>{desc}</span>
              </span>
              {intent === id && <span aria-hidden="true" style={{ marginInlineStart: 'auto' }}>&#x2713;</span>}
            </button>
          ))}
        </div>
      </fieldset>
      <div className="bs-actions">
        <button type="button" className="bs-button bs-button--secondary" onClick={() => setScreen('home')}>
          &#x2190; Back
        </button>
        <button
          type="button"
          className="bs-button"
          disabled={!intent || loading || !entryReady}
          onClick={() => {
            if (!service?.intake_open) { setErrorMsg('Online help is not currently available. See other help options below.'); return; }
            if (session) { setScreen('describe'); }
            else void handleStartSession('describe');
          }}
        >
          {loading ? 'Starting\u2026' : 'Next \u2192'}
        </button>
      </div>
    </section>
  );

  // ═══════════════════════════════════════════════════════════════════════
  // C03 DESCRIBE (text only — no voice UI yet per Task 09/08 pending)
  // ═══════════════════════════════════════════════════════════════════════
  const renderDescribe = () => (
    <section className="bs-stack">
      <div>
        <h1>Describe what happened</h1>
        <p className="bs-help">
          Share as much or as little as you feel comfortable with.
        </p>
      </div>
      {errorBanner}
      <div className="bs-field">
        <label htmlFor="account-text">What would you like to tell us?</label>
        <textarea
          id="account-text"
          className="bs-input"
          rows={6}
          placeholder="Tell us what's happening&#x2026;"
          value={accountText}
          onChange={e => setAccountText(e.target.value)}
        />
        <span className="bs-help">This text is only readable by the support team.</span>
      </div>
      <div className="bs-field">
        <label htmlFor="conflict-flag">Are any of these involved?</label>
        <select
          id="conflict-flag"
          className="bs-input"
          value={conflictFlag}
          onChange={e => setConflictFlag(e.target.value)}
        >
          <option value="">None / Not applicable</option>
          <option value="school_implicated">School staff or administration</option>
          <option value="caregiver_implicated">A caregiver or family member</option>
          <option value="staff_implicated">An organisation responder or officer</option>
        </select>
      </div>
      <div className="bs-actions">
        <button type="button" className="bs-button bs-button--secondary" onClick={() => setScreen('intent')}>
          &#x2190; Back
        </button>
        <button type="button" className="bs-button" onClick={() => setScreen('consent')}>
          Continue &#x2192;
        </button>
      </div>
    </section>
  );

  // ═══════════════════════════════════════════════════════════════════════
  // C04 REPLY CONSENT
  // ═══════════════════════════════════════════════════════════════════════
  const renderConsent = () => (
    <section className="bs-stack">
      <div>
        <h1>Would you like a reply?</h1>
        <p className="bs-help">
          We can send messages back through this private app. No phone call, email, or push notification is sent from your report.
        </p>
      </div>
      {errorBanner}
      <fieldset style={{ border: 'none', padding: 0, margin: 0 }}>
        <legend className="bs-sr-only">Reply consent</legend>
        <div className="bs-stack">
          <button
            type="button"
            className="bs-choice"
            aria-pressed={replyConsent === true}
            onClick={() => { setReplyConsent(true); setPreferredChannel('message_in_app'); }}
          >
            <span style={{ fontSize: '1.75rem' }} aria-hidden="true">&#x2705;</span>
            <span>
              <strong>Yes &#x2014; message me in the app</strong>
              <span style={{ display: 'block', fontWeight: 400, fontSize: '0.9em', marginTop: 2 }}>
                I'll check back here using my return code.
              </span>
            </span>
            {replyConsent === true && <span aria-hidden="true" style={{ marginInlineStart: 'auto' }}>&#x2713;</span>}
          </button>
          <button
            type="button"
            className="bs-choice bs-choice--sky"
            aria-pressed={replyConsent === false}
            onClick={() => { setReplyConsent(false); setPreferredChannel('no_contact'); }}
          >
            <span style={{ fontSize: '1.75rem' }} aria-hidden="true">&#x1F512;</span>
            <span>
              <strong>No &#x2014; record only, don't contact me</strong>
              <span style={{ display: 'block', fontWeight: 400, fontSize: '0.9em', marginTop: 2 }}>
                The report will be saved but the team won't send replies.
              </span>
            </span>
            {replyConsent === false && <span aria-hidden="true" style={{ marginInlineStart: 'auto' }}>&#x2713;</span>}
          </button>
        </div>
      </fieldset>

      {replyConsent === true && (
        <div className="bs-card" style={{ marginTop: 4 }}>
          <p style={{ fontWeight: 650, marginBottom: 12 }}>When may staff reply? (India time)</p>
          <div style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
            <div className="bs-field">
              <label htmlFor="safe-start">From</label>
              <input
                id="safe-start"
                type="time"
                className="bs-input"
                style={{ minBlockSize: 'auto', padding: '8px 12px' }}
                value={safeHoursStart}
                onChange={e => setSafeHoursStart(e.target.value)}
              />
            </div>
            <div className="bs-field">
              <label htmlFor="safe-end">To</label>
              <input
                id="safe-end"
                type="time"
                className="bs-input"
                style={{ minBlockSize: 'auto', padding: '8px 12px' }}
                value={safeHoursEnd}
                onChange={e => setSafeHoursEnd(e.target.value)}
              />
            </div>
          </div>
        </div>
      )}

      <div className="bs-actions">
        <button type="button" className="bs-button bs-button--secondary" onClick={() => setScreen('describe')}>
          &#x2190; Back
        </button>
        <ActionButton
          disabled={replyConsent === null}
          disabledReason="Please choose whether you'd like a reply."
          onClick={() => setScreen('review')}
        >
          Review my report &#x2192;
        </ActionButton>
      </div>
    </section>
  );

  // ═══════════════════════════════════════════════════════════════════════
  // C05 REVIEW
  // ═══════════════════════════════════════════════════════════════════════
  const intentLabels: Record<IntentChoice, string> = {
    ask_for_help: 'I need help',
    worried_about_someone: 'Worried about someone',
    missing_child: 'A child is missing',
    not_sure: 'Not sure — just need to talk',
  };

  const renderReview = () => (
    <section className="bs-stack">
      <h1>Review before sending</h1>
      {errorBanner}
      <div className="bs-card">
        <dl style={{ display: 'grid', gap: 12, margin: 0 }}>
          <div>
            <dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Type of help</dt>
            <dd style={{ margin: '4px 0 0' }}>{intent ? intentLabels[intent] : '&#x2014;'}</dd>
          </div>
          {accountText.trim() && (
            <div>
              <dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Your message</dt>
              <dd style={{ margin: '4px 0 0', whiteSpace: 'pre-wrap' }}>{accountText}</dd>
            </div>
          )}
          {conflictFlag && (
            <div>
              <dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Conflict flag</dt>
              <dd style={{ margin: '4px 0 0' }}>{conflictFlag.replace(/_/g, ' ')}</dd>
            </div>
          )}
          <div>
            <dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Replies</dt>
            <dd style={{ margin: '4px 0 0' }}>
              {replyConsent === true
                ? `Yes — in-app messages (${safeHoursStart}–${safeHoursEnd} IST)`
                : replyConsent === false
                  ? 'No — record only'
                  : 'Not specified'}
            </dd>
          </div>
        </dl>
      </div>
      <div className="bs-notice" style={{ fontSize: '0.95rem' }}>
        Once you send, your report is saved. You can check replies using the return code shown on the next screen.
      </div>
      <div className="bs-actions">
        <button type="button" className="bs-button bs-button--secondary" onClick={() => setScreen('consent')}>
          &#x2190; Edit
        </button>
        <ActionButton disabled={loading} onClick={handleSubmitCase}>
          {loading ? 'Sending\u2026' : 'Send report'}
        </ActionButton>
      </div>
    </section>
  );

  // ═══════════════════════════════════════════════════════════════════════
  // C06 RECEIPT
  // ═══════════════════════════════════════════════════════════════════════
  const renderReceipt = () => {
    if (!receipt) return null;
    return (
      <section className="bs-stack" style={{ textAlign: 'center' }}>
        <div>
          <div style={{ fontSize: '3rem', marginBottom: 8 }} aria-hidden="true">&#x2705;</div>
          <h1 style={{ marginBottom: 8 }}>Your report is saved</h1>
          <p className="bs-help">
            {mapStatus(receipt.status)} — this does not mean a responder has accepted it yet.
          </p>
        </div>
        <div className="bs-card" style={{ textAlign: 'left' }}>
          <dl style={{ display: 'grid', gap: 12, margin: 0 }}>
            <div>
              <dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Receipt ID</dt>
              <dd style={{ fontFamily: 'monospace', fontSize: '1.1rem', margin: '4px 0 0', overflowWrap: 'anywhere' }}>{receipt.receipt_id}</dd>
            </div>
            <div>
              <dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Case ID</dt>
              <dd style={{ fontFamily: 'monospace', fontSize: '1.1rem', margin: '4px 0 0', overflowWrap: 'anywhere' }}>{receipt.case_id}</dd>
            </div>
            <div>
              <dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Status</dt>
              <dd style={{ margin: '4px 0 0' }}>{mapStatus(receipt.status)}</dd>
            </div>
          </dl>
        </div>

        {session?.return_code && (
          <div className="bs-card" style={{ textAlign: 'left' }}>
            <p style={{ fontWeight: 650 }}>Your private return code</p>
            <p className="bs-help">Save this somewhere only you can access. It opens your report for 30 days. If lost, it cannot be recovered.</p>
            <code
              style={{
                display: 'block',
                overflowWrap: 'anywhere',
                padding: '16px 0',
                userSelect: 'all',
                fontSize: '1.05rem',
                letterSpacing: '0.04em',
              }}
            >
              {session.return_code.match(/.{1,16}/g)?.join(' \u2013 ')}
            </code>
            <p className="bs-help">Your receipt ID alone cannot open your report.</p>
          </div>
        )}

        <div className="bs-actions">
          <button type="button" className="bs-button bs-button--secondary" onClick={() => setScreen('home')}>
            Back to home
          </button>
          <button type="button" className="bs-button" onClick={() => setScreen('return')}>
            Check my report &#x2192;
          </button>
        </div>
      </section>
    );
  };

  // ═══════════════════════════════════════════════════════════════════════
  // C08 RETURN ACCESS
  // ═══════════════════════════════════════════════════════════════════════
  const renderReturn = () => (
    <section className="bs-stack">
      <div>
        <h1>My messages</h1>
        <p className="bs-help">Paste the long return code from your receipt to open your report.</p>
      </div>
      {errorBanner}
      <form onSubmit={handleReturnAccess} className="bs-stack">
        <div className="bs-field">
          <label htmlFor="return-code">Private return code</label>
          <textarea
            id="return-code"
            className="bs-input"
            value={returnCode}
            onChange={e => setReturnCode(e.target.value)}
            autoComplete="off"
            spellCheck={false}
            rows={3}
            placeholder="Paste your return code here&#x2026;"
          />
          <span className="bs-help">Use the long code from your receipt. Older short codes no longer work.</span>
        </div>
        <div className="bs-actions">
          <button type="button" className="bs-button bs-button--secondary" onClick={() => setScreen('home')}>
            &#x2190; Home
          </button>
          <button type="submit" className="bs-button" disabled={loading || !returnCode.trim()}>
            {loading ? 'Opening\u2026' : 'Open my report \u2192'}
          </button>
        </div>
      </form>
    </section>
  );

  // ═══════════════════════════════════════════════════════════════════════
  // C09 CONVERSATION
  // ═══════════════════════════════════════════════════════════════════════
  const renderConversation = () => {
    if (!returnCaseView) return renderReturn();
    return (
      <section className="bs-stack">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: 8 }}>
          <div>
            <h1>Your report</h1>
            <p className="bs-help">Case {returnCaseView.case_id}</p>
          </div>
          <span style={{
            background: 'var(--bs-sky)',
            color: 'var(--bs-teal)',
            borderRadius: 8,
            padding: '4px 12px',
            fontWeight: 650,
            fontSize: '0.9rem',
          }}>
            {mapStatus(returnCaseView.status)}
          </span>
        </div>
        {errorBanner}
        {returnCaseView.safe_progress_message && (
          <div className="bs-notice">{returnCaseView.safe_progress_message}</div>
        )}

        <section aria-label="Messages from the support team">
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4, minHeight: 80 }}>
            {messages.length === 0 && (
              <p className="bs-help">No messages yet — the team will reply here if you chose in-app replies.</p>
            )}
            {messages.map(msg => (
              <article
                key={msg.id}
                className={`bs-message${msg.sender_type === 'reporter' ? ' bs-message--mine' : ''}`}
              >
                <p style={{ margin: 0, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{msg.body}</p>
                <small>{new Date(msg.sent_at).toLocaleString()}</small>
              </article>
            ))}
          </div>
        </section>

        {returnCaseView.status !== 'CLOSED' && (
          <form onSubmit={handleSendReply} className="bs-stack">
            <div className="bs-field">
              <label htmlFor="reply-text">Add a message</label>
              <textarea
                id="reply-text"
                className="bs-input"
                rows={3}
                maxLength={5000}
                value={reply}
                onChange={e => setReply(e.target.value)}
              />
            </div>
            <div className="bs-actions">
              <button type="submit" className="bs-button" disabled={loading || !reply.trim()}>
                {loading ? 'Sending\u2026' : 'Send message'}
              </button>
            </div>
          </form>
        )}

        <div className="bs-actions">
          <button
            type="button"
            className="bs-button bs-button--secondary"
            onClick={() => { setReturnCaseView(null); setScreen('return'); }}
          >
            &#x2190; Back
          </button>
        </div>
      </section>
    );
  };

  // ═══════════════════════════════════════════════════════════════════════
  // M01-M05 MISSING CHILD INTAKE
  // ═══════════════════════════════════════════════════════════════════════
  const MISSING_STEPS = ['Child details', 'Last seen', 'Description', 'Review', 'Receipt'];

  const renderMissing = (): React.ReactElement | null => {
    // M05 receipt
    if (missingStep === 5 && missingReceipt) {
      return (
        <section className="bs-stack" style={{ textAlign: 'center' }}>
          <div style={{ fontSize: '3rem' }} aria-hidden="true">&#x2705;</div>
          <h1>Missing report submitted</h1>
          <p className="bs-help">
            {mapStatus(missingReceipt.status)} — staff will review before any public alert is issued.
          </p>
          <div className="bs-card" style={{ textAlign: 'left' }}>
            <dl style={{ display: 'grid', gap: 10, margin: 0 }}>
              <div>
                <dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Receipt ID</dt>
                <dd style={{ fontFamily: 'monospace', margin: '4px 0 0', overflowWrap: 'anywhere' }}>{missingReceipt.receipt_id}</dd>
              </div>
              <div>
                <dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Status</dt>
                <dd style={{ margin: '4px 0 0' }}>{mapStatus(missingReceipt.status)}</dd>
              </div>
            </dl>
          </div>
          <p className="bs-notice" style={{ fontSize: '0.95rem' }}>
            For immediate danger, call <strong>{BRAND.emergency}</strong> (emergency) or <strong>{BRAND.childHelpline}</strong> (child helpline).
          </p>
          <div className="bs-actions">
            <button type="button" className="bs-button" onClick={() => setScreen('home')}>
              Back to home
            </button>
          </div>
        </section>
      );
    }

    const stepBar = (
      <nav aria-label="Missing report steps" style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginBottom: 8 }}>
        {MISSING_STEPS.map((s, i) => (
          <span key={s} style={{
            padding: '4px 10px',
            borderRadius: 8,
            fontSize: '0.85rem',
            fontWeight: i === missingStep ? 750 : 400,
            background: i === missingStep ? 'var(--bs-primary)' : i < missingStep ? 'var(--bs-teal)' : '#e7e3ed',
            color: i <= missingStep ? 'white' : 'var(--bs-muted)',
          }}>
            {s}
          </span>
        ))}
      </nav>
    );

    // M01 — child details
    if (missingStep === 0) return (
      <section className="bs-stack">
        {stepBar}
        <h1>Child's details</h1>
        {errorBanner}
        <div className="bs-field">
          <label htmlFor="m-name">Child's first name (or description)</label>
          <input id="m-name" className="bs-input" value={missingIntake.childName}
            onChange={e => setMissingIntake(p => ({ ...p, childName: e.target.value }))}
            placeholder="First name or nickname" />
        </div>
        <div className="bs-field">
          <label htmlFor="m-age">Approximate age</label>
          <input id="m-age" className="bs-input" value={missingIntake.age}
            onChange={e => setMissingIntake(p => ({ ...p, age: e.target.value }))}
            placeholder="e.g. 9 years old" />
        </div>
        <div className="bs-actions">
          <button type="button" className="bs-button bs-button--secondary" onClick={() => setScreen('home')}>&#x2190; Cancel</button>
          <button type="button" className="bs-button"
            disabled={!missingIntake.childName.trim()}
            onClick={() => setMissingStep(1)}>
            Next &#x2192;
          </button>
        </div>
      </section>
    );

    // M02 — last seen
    if (missingStep === 1) return (
      <section className="bs-stack">
        {stepBar}
        <h1>When and where were they last seen?</h1>
        {errorBanner}
        <div className="bs-field">
          <label htmlFor="m-lastseen">When were they last seen?</label>
          <input id="m-lastseen" className="bs-input" value={missingIntake.lastSeen}
            onChange={e => setMissingIntake(p => ({ ...p, lastSeen: e.target.value }))}
            placeholder="e.g. Yesterday at 3 pm" />
        </div>
        <div className="bs-field">
          <label htmlFor="m-location">Last known location</label>
          <input id="m-location" className="bs-input" value={missingIntake.lastLocation}
            onChange={e => setMissingIntake(p => ({ ...p, lastLocation: e.target.value }))}
            placeholder="e.g. Near the school on MG Road" />
        </div>
        <div className="bs-actions">
          <button type="button" className="bs-button bs-button--secondary" onClick={() => setMissingStep(0)}>&#x2190; Back</button>
          <button type="button" className="bs-button" onClick={() => setMissingStep(2)}>Next &#x2192;</button>
        </div>
      </section>
    );

    // M03 — description
    if (missingStep === 2) return (
      <section className="bs-stack">
        {stepBar}
        <h1>Describe the child</h1>
        {errorBanner}
        <div className="bs-field">
          <label htmlFor="m-desc">Clothing, appearance, any other details</label>
          <textarea id="m-desc" className="bs-input" rows={5} value={missingIntake.description}
            onChange={e => setMissingIntake(p => ({ ...p, description: e.target.value }))}
            placeholder="What were they wearing? Any identifying features?" />
        </div>
        <div className="bs-actions">
          <button type="button" className="bs-button bs-button--secondary" onClick={() => setMissingStep(1)}>&#x2190; Back</button>
          <button type="button" className="bs-button" onClick={() => setMissingStep(3)}>Review &#x2192;</button>
        </div>
      </section>
    );

    // M04 — review
    if (missingStep === 3) return (
      <section className="bs-stack">
        {stepBar}
        <h1>Review before sending</h1>
        {errorBanner}
        <div className="bs-card">
          <dl style={{ display: 'grid', gap: 10, margin: 0 }}>
            <div><dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Name</dt><dd style={{ margin: '4px 0 0' }}>{missingIntake.childName}</dd></div>
            <div><dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Age</dt><dd style={{ margin: '4px 0 0' }}>{missingIntake.age || '—'}</dd></div>
            <div><dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Last seen</dt><dd style={{ margin: '4px 0 0' }}>{missingIntake.lastSeen || '—'}</dd></div>
            <div><dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Location</dt><dd style={{ margin: '4px 0 0' }}>{missingIntake.lastLocation || '—'}</dd></div>
            <div><dt style={{ fontWeight: 650, fontSize: '0.85rem', color: 'var(--bs-muted)', textTransform: 'uppercase' }}>Description</dt><dd style={{ margin: '4px 0 0', whiteSpace: 'pre-wrap' }}>{missingIntake.description || '—'}</dd></div>
          </dl>
        </div>
        <div className="bs-notice" style={{ fontSize: '0.95rem' }}>
          Staff will review this report before issuing any public alert. For immediate danger, call <strong>{BRAND.emergency}</strong>.
        </div>
        <div className="bs-actions">
          <button type="button" className="bs-button bs-button--secondary" onClick={() => setMissingStep(2)}>&#x2190; Edit</button>
          <ActionButton disabled={loading} onClick={handleSubmitMissing}>
            {loading ? 'Sending\u2026' : 'Send report'}
          </ActionButton>
        </div>
      </section>
    );

    return null;
  };

  // ═══════════════════════════════════════════════════════════════════════
  // C11 OTHER HELP
  // ═══════════════════════════════════════════════════════════════════════
  const renderOtherHelp = () => (
    <section className="bs-stack">
      <h1>Other ways to get help</h1>
      <p className="bs-help">
        These are phone helplines. Calling may appear in your call history and on your phone bill.
        If that could be unsafe, use the private report instead.
      </p>
      {errorBanner}
      <div className="bs-stack">
        <div className="bs-card">
          <p style={{ fontWeight: 750, fontSize: '1.25rem', margin: '0 0 6px' }}>
            <a href={`tel:${BRAND.childHelpline}`}>{BRAND.childHelpline}</a>
          </p>
          <p style={{ margin: 0 }}>Child helpline &#x2014; free, 24/7, confidential.</p>
          <p className="bs-help" style={{ margin: '6px 0 0' }}>
            Calling may appear in your call history.
          </p>
        </div>
        <div className="bs-card">
          <p style={{ fontWeight: 750, fontSize: '1.25rem', margin: '0 0 6px' }}>
            <a href={`tel:${BRAND.emergency}`}>{BRAND.emergency}</a>
          </p>
          <p style={{ margin: 0 }}>Emergency &#x2014; police, fire, or ambulance.</p>
          <p className="bs-help" style={{ margin: '6px 0 0' }}>
            Calling may appear in your call history.
          </p>
        </div>
      </div>
      <div className="bs-actions">
        <button type="button" className="bs-button bs-button--secondary" onClick={() => setScreen('home')}>
          &#x2190; Home
        </button>
      </div>
    </section>
  );

  // ═══════════════════════════════════════════════════════════════════════
  // C12 PRIVACY
  // ═══════════════════════════════════════════════════════════════════════
  const renderPrivacy = () => (
    <section className="bs-stack">
      <h1>How we protect your information</h1>
      <p className="bs-help">A plain-language summary &#x2014; three layers.</p>
      {errorBanner}
      <div className="bs-card">
        <h2 style={{ margin: '0 0 8px' }}>Layer 1 &#x2014; No account needed</h2>
        <p style={{ margin: 0 }}>
          You never give us your name, phone number, or email. Reports are identified only by a random session ID. If you close the tab, that link is gone.
        </p>
      </div>
      <div className="bs-card">
        <h2 style={{ margin: '0 0 8px' }}>Layer 2 &#x2014; Private return access</h2>
        <p style={{ margin: 0 }}>
          If you want to check replies, you get a long return code (shown once). Only someone who has that code can open the report. We don't store it in a way we can recover for you.
        </p>
      </div>
      <div className="bs-card">
        <h2 style={{ margin: '0 0 8px' }}>Layer 3 &#x2014; Quick Exit</h2>
        <p style={{ margin: 0 }}>
          The "Quick exit" button immediately takes you to a neutral news page. It clears the session. It does not erase your browser history &#x2014; use private browsing if you need that.
        </p>
      </div>
      <div className="bs-notice" style={{ fontSize: '0.95rem' }}>
        This summary is not a legal privacy policy. The full policy is held by the support organisation operating this service.
      </div>
      <div className="bs-actions">
        <button type="button" className="bs-button bs-button--secondary" onClick={() => setScreen('home')}>
          &#x2190; Home
        </button>
      </div>
    </section>
  );

  // ═══════════════════════════════════════════════════════════════════════
  // ROUTE RENDERING
  // ═══════════════════════════════════════════════════════════════════════
  const renderScreen = () => {
    switch (screen) {
      case 'home':         return renderHome();
      case 'intent':       return renderIntent();
      case 'describe':     return renderDescribe();
      case 'consent':      return renderConsent();
      case 'review':       return renderReview();
      case 'receipt':      return renderReceipt();
      case 'return':       return renderReturn();
      case 'conversation': return renderConversation();
      case 'missing':      return renderMissing();
      case 'other_help':   return renderOtherHelp();
      case 'privacy':      return renderPrivacy();
      default:             return renderHome();
    }
  };

  // ═══════════════════════════════════════════════════════════════════════
  // RENDER
  // ═══════════════════════════════════════════════════════════════════════
  return (
    <ChildShell
      brand={brand}
      languageControl={languageControl}
      leaveLabel="Quick exit \u2197"
      onLeave={handleQuickExit}
      footer={footer}
    >
      {renderScreen()}
    </ChildShell>
  );
}

export default App;
