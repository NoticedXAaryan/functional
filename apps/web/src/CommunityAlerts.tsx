import { useEffect, useState } from 'react';
import './CommunityAlerts.css';

const API = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080/api/v1';
const STORAGE = 'savera.subscription.v1';
type Area = { id: string; name: string };
type Config = { areas: Area[]; is_test: boolean; enrollment_available: boolean; public_key: string; mode: string };
type Alert = { alert_id: string; status: string; description_text: string; issuer_name: string; area_name: string; expiry_at: string; is_test: boolean; tip_route_available: boolean };
type Enrollment = { secret: string; subscription_id?: string; area_id?: string };
function readEnrollment(): Enrollment | null { try { return JSON.parse(localStorage.getItem(STORAGE) ?? 'null'); } catch { return null; } }
function decodeKey(key: string): Uint8Array<ArrayBuffer> { return Uint8Array.from(atob(key.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0)); }
function makeSecret() { return btoa(String.fromCharCode(...crypto.getRandomValues(new Uint8Array(32)))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, ''); }
async function request(path: string, init: RequestInit = {}) {
  const res = await fetch(`${API}${path}`, { ...init, cache: 'no-store' });
  if (res.status === 204) return null;
  const data = await res.json();
  if (!res.ok) throw new Error(typeof data.error === 'string' ? data.error : 'The service is unavailable. Please try again.');
  return data;
}
export default function CommunityAlerts() {
  const [config, setConfig] = useState<Config | null>(null);
  const [area, setArea] = useState('');
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [enrollment, setEnrollment] = useState<Enrollment | null>(readEnrollment);
  const [consent, setConsent] = useState(false);
  const [invite, setInvite] = useState('');
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');
  const [loaded, setLoaded] = useState(false);
  const [deviceReady, setDeviceReady] = useState(false);
  const [tip, setTip] = useState('');
  const [location, setLocation] = useState('');
  const [tipReceipt, setTipReceipt] = useState('');
  const [tipKey, setTipKey] = useState(() => crypto.randomUUID());
  const alertID = window.location.pathname.match(/^\/alerts\/([a-f0-9-]{36})(?:\/status)?\/?$/i)?.[1];
  const supported = 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window && window.isSecureContext;
  const refresh = async (selected = area) => {
    setError('');
    try {
      const data = await request(alertID ? `/public/alerts/${alertID}` : `/public/alerts?area_id=${encodeURIComponent(selected)}`);
      setAlerts(alertID ? [data] : data.alerts); setLoaded(true);
    } catch (e) { setError((e as Error).message); setLoaded(false); }
  };
  useEffect(() => {
    document.title = 'Savera Alert · Local child safety';
    request('/subscriptions/config').then(setConfig).catch(e => setError(e.message));
    if (supported) navigator.serviceWorker.register('/sw.js').then(async reg => setDeviceReady(Notification.permission === 'granted' && !!await reg.pushManager.getSubscription())).catch(() => setDeviceReady(false));
    const rotated = (event: MessageEvent) => { if (event.data?.type === 'PUSH_SUBSCRIPTION_CHANGED') { setDeviceReady(false); setNotice('Your browser subscription changed. Re-enable notifications.'); } };
    navigator.serviceWorker?.addEventListener('message', rotated);
    return () => navigator.serviceWorker?.removeEventListener('message', rotated);
  }, []);
  useEffect(() => {
    void refresh(area);
    const timer = window.setInterval(() => { if (!document.hidden) void refresh(area); }, 30000);
    return () => window.clearInterval(timer);
  }, [area]);
  const enroll = async () => {
    setBusy(true); setError(''); setNotice('');
    try {
      if (!config?.enrollment_available || !area || !consent || !supported) throw new Error('Choose an area and confirm notification consent first.');
      if (await Notification.requestPermission() !== 'granted') throw new Error('Notifications are blocked. You can still read alerts here. Change browser permissions to enable them.');
      const registration = await navigator.serviceWorker.ready;
      let sub = await registration.pushManager.getSubscription();
      if (!sub) sub = await registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: decodeKey(config.public_key) });
      const saved = enrollment ?? { secret: makeSecret() };
      // Preserve the capability before submitting, so a dropped response is retryable.
      localStorage.setItem(STORAGE, JSON.stringify(saved)); setEnrollment(saved);
      const json = sub.toJSON();
      const result = await request('/subscriptions', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ area_id: area, endpoint: sub.endpoint, auth: json.keys?.auth, p256dh: json.keys?.p256dh, consent_confirmed: true, management_secret: saved.secret, enrollment_code: invite }) });
      const record = { ...saved, subscription_id: result.subscription_id, area_id: area };
      localStorage.setItem(STORAGE, JSON.stringify(record)); setEnrollment(record); setDeviceReady(true);
      setNotice('Registered for this area. Push delivery depends on your browser and connection; no notification has been sent yet.');
    } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  };
  const stop = async () => {
    if (!enrollment?.subscription_id) return;
    setBusy(true); setError('');
    try {
      await request(`/subscriptions/${enrollment.subscription_id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${enrollment.secret}` } });
      const reg = await navigator.serviceWorker.ready; const sub = await reg.pushManager.getSubscription(); await sub?.unsubscribe();
      localStorage.removeItem(STORAGE); setEnrollment(null); setDeviceReady(false); setNotice('Notifications stopped for this browser.');
    } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  };
  const submitTip = async (e: React.FormEvent) => {
    e.preventDefault(); setBusy(true); setError('');
    try {
      const result = await request('/tips', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': tipKey }, body: JSON.stringify({ alert_id: alertID, sighting_description: tip, approximate_location: location, contact_preference: 'no_contact' }) });
      setTipReceipt(result.receipt_id); setTip(''); setLocation(''); setTipKey(crypto.randomUUID());
    } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  };
  return <div className="savera">
    <header className="sa-header"><a className="sa-brand" href="/alerts"><img src="/savera.svg" alt="" width="46" height="46"/><span>Savera <strong>Alert</strong><small>Missing-child alerts in your area</small></span></a><nav className="workspace-links" aria-label="Other spaces"><a href="/">Home & private help</a><a href={import.meta.env.VITE_OPS_URL ?? 'http://localhost:5174'}>Staff sign in</a></nav></header>
    {config?.is_test && <div className="sa-test">Practice alerts only — no real missing-child cases are published here.</div>}
    <main className="sa-main">
      <section className="sa-intro"><p className="sa-eyebrow">Local missing-child alerts</p><h1>{alertID ? 'Read this alert' : 'Missing-child alerts\nnear you'}</h1><p>Choose an area to read alerts published by the reviewing team. Open an alert to see its latest details or share something you saw.</p><div className="sa-principles"><span>Staff review each alert</span><span>You choose the area</span><span>You can read alerts without signing in</span></div></section>
      {error && <div className="sa-error" role="alert">{error} <button onClick={() => void refresh()}>Refresh status</button></div>}
      {notice && <div className="sa-notice" role="status">{notice}</div>}
      <div className="sa-layout"><section className="sa-feed" aria-label="Current alerts"><div className="sa-section-heading"><h2>{alertID ? 'Alert details' : 'Alerts'}</h2><button onClick={() => void refresh()}>Refresh</button></div>
        {!alertID && <label className="sa-filter">Choose an area<select value={area} onChange={e => setArea(e.target.value)}><option value="">All available areas</option>{config?.areas.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}</select></label>}
        {!loaded && !error && <p role="status">Checking current alerts…</p>}
        {loaded && alerts.length === 0 && <div className="sa-empty"><img src="/savera.svg" width="56" height="56" alt=""/><h3>No alerts to show</h3><p>No alerts are currently published for this selection. Choose another area or check again later.</p></div>}
        {alerts.map(a => <article className="sa-alert" key={a.alert_id}><div className="sa-alert-top"><span className={`sa-state ${a.status === 'ACTIVE' ? 'sa-active' : ''}`}>{a.status === 'ACTIVE' ? 'Active alert' : a.status.toLowerCase()}</span>{a.is_test && <span className="sa-fictional">TEST · FICTIONAL</span>}</div><h3>{a.area_name}</h3>{a.status === 'ACTIVE' ? <p className="sa-description">{a.description_text}</p> : <p>This alert is no longer active. Identifying details have been removed. Do not continue circulating an older copy.</p>}<dl><div><dt>Published by</dt><dd>{a.issuer_name}</dd></div><div><dt>Expires</dt><dd>{new Date(a.expiry_at).toLocaleString()}</dd></div></dl>{!alertID && <a className="sa-button" href={`/alerts/${a.alert_id}`}>Read alert and share a tip →</a>}</article>)}
        {alertID && alerts[0]?.tip_route_available && <section className="sa-tip"><h2>Share what you saw</h2><p>Describe only what you observed. Do not approach, follow or confront anyone. Your tip is private to the authorized reviewing team.</p>{tipReceipt ? <div className="sa-notice" role="status">Tip received. Keep this receipt: <code>{tipReceipt}</code>. This confirms receipt, not that someone has reviewed it.</div> : <form onSubmit={submitTip}><label>What did you see?<textarea value={tip} onChange={e => setTip(e.target.value)} required maxLength={5000}/></label><label>Approximate place and time<input value={location} onChange={e => setLocation(e.target.value)} maxLength={500}/></label><p>No contact details are requested here.</p><button className="sa-button" disabled={busy}>{busy ? 'Sending…' : 'Send private tip'}</button></form>}</section>}
      </section><aside className="sa-subscribe"><p className="sa-eyebrow">Optional notifications</p><h2>Get alerts on this device</h2>{config?.enrollment_available && <p>Choose an area and allow browser notifications to receive new alerts here.</p>}{(config?.enrollment_available || enrollment?.subscription_id) && <div className="sa-readiness"><span className={deviceReady && enrollment?.subscription_id ? 'sa-dot ready' : 'sa-dot'}/>{deviceReady && enrollment?.subscription_id ? 'Notifications are set up on this browser' : 'Notifications are not set up on this browser'}</div>}
        {!supported && <p className="sa-notice">This browser cannot enable push here. On iPhone or iPad, add this page to your Home Screen and open it there. You can still read alerts on this page.</p>}
        {config && !config.enrollment_available && <p className="sa-notice">Notifications are turned off for this preview. You can still read any published alerts on this page.</p>}
        {config?.enrollment_available && <><label>Area for notifications<select value={area} onChange={e => setArea(e.target.value)}><option value="">Choose an area</option>{config?.areas.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}</select></label>
        {config?.is_test && <label>Invitation code<input type="password" autoComplete="off" value={invite} onChange={e => setInvite(e.target.value)} placeholder="Enter the code from the person running your trial"/></label>}
        <label className="sa-consent"><input type="checkbox" checked={consent} onChange={e => setConsent(e.target.checked)}/><span>I am an adult and want {config?.is_test ? 'clearly labeled test ' : ''}notifications for this area. Alerts may appear on my lock screen.</span></label>
        <button className="sa-button" disabled={busy || !consent || !area || !supported || !config?.enrollment_available} onClick={() => void enroll()}>{busy ? 'Working…' : enrollment?.subscription_id ? 'Save notification settings' : 'Turn on notifications'}</button></>}
        {enrollment?.subscription_id && <button className="sa-stop" disabled={busy} onClick={() => void stop()}>Stop notifications</button>}
        {(config?.enrollment_available || enrollment?.subscription_id) && <p className="sa-fine">Notifications contain public alert information only. Your browser or phone may delay or block them. Use “Stop notifications” here to unsubscribe.</p>}
      </aside></div>
      <footer className="sa-footer"><strong>Immediate danger?</strong> Call <a href="tel:112">112</a>. For child support, call <a href="tel:1098">1098</a>.<p>Savera is an independent project. No automatic police dispatch or government broadcast is connected.</p></footer>
    </main>
  </div>;
}
