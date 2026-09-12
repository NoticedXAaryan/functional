import { useEffect, useState } from 'react';
import { API_BASE, staffFetch } from './auth';
import type { StaffClaims } from './auth';

type Revision = { id: string; alert_id: string; status: string; alert_status: string; description_text: string; issuer_name: string; expiry_at: string; prepared_by: string; area_name: string; verification_reference: string; is_test: boolean; queued: number; provider_accepted: number; failed: number };
type MissingCase = { case_id?: string; id?: string; route_type: string };
export default function AlertConsole({ staff }: { staff: StaffClaims }) {
  const [revisions, setRevisions] = useState<Revision[]>([]);
  const [areas, setAreas] = useState<Array<{ id: string; name: string }>>([]);
  const [cases, setCases] = useState<MissingCase[]>([]);
  const [mode, setMode] = useState('');
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const [draft, setDraft] = useState({ case_id: '', area_id: '', description_text: '', issuer_name: '', verification_reference: '', expiry_at: '' });
  const [closing, setClosing] = useState<{ id: string; action: 'withdraw' | 'resolve' } | null>(null);
  const [reason, setReason] = useState('');
  const prepare = ['alert_preparer', 'admin'].includes(staff.role);
  const approve = ['alert_approver', 'admin'].includes(staff.role);
  const close = ['alert_approver', 'supervisor', 'admin'].includes(staff.role);
  const api = async (path: string, body?: unknown) => {
    const response = await staffFetch(`${API_BASE}${path}`, { method: body === undefined ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${staff.token}` }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'The action could not be completed');
    return data;
  };
  const refresh = async () => {
    try { const data = await api('/staff/alert-approvals'); setRevisions(data.drafts); setMode(data.notification_mode); } catch (e) { setError((e as Error).message); }
  };
  useEffect(() => {
    void refresh();
    api('/subscriptions/config').then(data => setAreas(data.areas)).catch(e => setError(e.message));
    if (prepare) api('/staff/cases').then(data => setCases((data.cases ?? []).filter((c: MissingCase) => c.route_type === 'missing_child'))).catch(e => setError(e.message));
  }, [staff.staffId]);
  const act = async (path: string, body: unknown, success: string) => {
    setBusy(true); setError(''); setMessage('');
    try { await api(path, body); setMessage(success); await refresh(); return true; } catch (e) { setError((e as Error).message); return false; } finally { setBusy(false); }
  };
  return <section>
    <div className="ops-page-header"><div><h2 className="ops-title">Savera Alert</h2><p className="ops-subtitle">Prepare → independent review → publish → monitor → close</p></div><button className="ops-btn-secondary" disabled={busy} onClick={() => void refresh()}>Refresh</button></div>
    <p className="ops-subtitle">{mode === 'DISABLED' ? 'Publishing and notifications are currently turned off. You can prepare and review drafts.' : mode === 'TEST_ALLOWLIST' ? 'Practice alerts go only to invited people who have signed up.' : 'Approved alerts can be published to people who signed up for their area.'}</p>
    {error && <p className="ops-error" role="alert">{error}</p>}{message && <p role="status">{message}</p>}
    {prepare && <form className="ops-card ops-form" onSubmit={async e => {
      e.preventDefault();
      if (await act('/staff/alert-drafts', { ...draft, expiry_at: new Date(draft.expiry_at).toISOString() }, 'Draft saved. Another authorized person must approve it before publication.')) setDraft({ case_id: '', area_id: '', description_text: '', issuer_name: '', verification_reference: '', expiry_at: '' });
    }}><h3 className="ops-title">Prepare a public alert</h3><p className="ops-subtitle">Use only approved public details. The private report is never copied automatically.</p>
      <label className="ops-label" htmlFor="alert-case">Missing-child report</label><select id="alert-case" className="ops-input" required value={draft.case_id} onChange={e => setDraft({ ...draft, case_id: e.target.value })}><option value="">Choose case</option>{cases.map(c => <option key={c.case_id ?? c.id} value={c.case_id ?? c.id}>{c.case_id ?? c.id}</option>)}</select>
      <label className="ops-label" htmlFor="alert-area">Area that should receive the alert</label><select id="alert-area" className="ops-input" required value={draft.area_id} onChange={e => setDraft({ ...draft, area_id: e.target.value })}><option value="">Choose area</option>{areas.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}</select>
      <label className="ops-label" htmlFor="alert-description">Public description (maximum 500 characters)</label><textarea id="alert-description" className="ops-input" required maxLength={500} value={draft.description_text} onChange={e => setDraft({ ...draft, description_text: e.target.value })}/>
      <label className="ops-label" htmlFor="alert-issuer">Organization publishing this alert</label><input id="alert-issuer" className="ops-input" required maxLength={255} value={draft.issuer_name} onChange={e => setDraft({ ...draft, issuer_name: e.target.value })}/>
      <label className="ops-label" htmlFor="alert-reference">How was this report checked? (staff only)</label><input id="alert-reference" className="ops-input" required maxLength={500} value={draft.verification_reference} onChange={e => setDraft({ ...draft, verification_reference: e.target.value })}/>
      <label className="ops-label" htmlFor="alert-expiry">When should the alert end? (within 24 hours)</label><input id="alert-expiry" className="ops-input" type="datetime-local" required value={draft.expiry_at} onChange={e => setDraft({ ...draft, expiry_at: e.target.value })}/>
      <button className="ops-btn-primary" disabled={busy}>Save for review</button>
    </form>}
    {revisions.length === 0 && <p className="ops-empty">No alerts have been prepared yet. An alert preparer can create one from a missing-child report.</p>}
    {revisions.map(d => <article className="ops-card ops-alert-card" key={d.id}><div className="ops-detail-header"><h3 className="ops-title">{d.area_name}</h3><span className="ops-badge">{d.alert_status}{d.is_test ? ' · TEST / FICTIONAL' : ''}</span></div><p className="ops-alert-body">{d.description_text}</p><p>Issuer: {d.issuer_name}</p><p>Verification reference: {d.verification_reference || 'Missing — cannot publish'}</p><p>Expires: {new Date(d.expiry_at).toLocaleString()}</p><p className="ops-muted">Queued: {d.queued} · Accepted by notification service: {d.provider_accepted} · Not sent / unconfirmed: {d.failed}</p>
      <div className="ops-alert-actions">
        {approve && d.status === 'DRAFT' && d.alert_status === 'DRAFT' && <><button className="ops-btn-primary" disabled={busy || d.prepared_by === staff.staffId} onClick={() => void act('/staff/alert-approvals', { revision_id: d.id, decision: 'approve' }, 'Alert approved. Choose Publish to make it visible.')}>Approve alert</button><button className="ops-btn-secondary" disabled={busy || d.prepared_by === staff.staffId} onClick={() => void act('/staff/alert-approvals', { revision_id: d.id, decision: 'reject' }, 'Revision rejected. Prepare a new draft with corrected information.')}>Reject</button></>}
        {approve && d.alert_status === 'APPROVED' && <button className="ops-btn-primary" disabled={busy || mode === 'DISABLED'} onClick={() => void act(`/staff/alerts/${d.alert_id}/activate`, { approved_revision_id: d.id }, 'Published. Delivery is queued; refresh to inspect provider outcomes.')}>Publish {d.is_test ? 'TEST ' : ''}alert</button>}
        {d.alert_status === 'ACTIVE' && close && <><button className="ops-btn-secondary" onClick={() => { setClosing({ id: d.alert_id, action: 'resolve' }); setReason(''); }}>Mark resolved</button><button className="ops-btn-secondary" onClick={() => { setClosing({ id: d.alert_id, action: 'withdraw' }); setReason(''); }}>Withdraw alert</button></>}
      </div>
    </article>)}
    {closing && <form className="ops-card ops-form" onSubmit={async e => { e.preventDefault(); if (await act(`/staff/alerts/${closing.id}/${closing.action}`, { reason }, 'Status updated and pending sends cancelled. Already accepted notifications cannot be recalled.')) setClosing(null); }}><h3 className="ops-title">{closing.action === 'resolve' ? 'Resolve alert' : 'Withdraw alert'}</h3><label htmlFor="close-reason">Brief non-identifying reason</label><input id="close-reason" autoFocus className="ops-input" required maxLength={500} value={reason} onChange={e => setReason(e.target.value)}/><button className="ops-btn-primary" disabled={busy}>Confirm status change</button><button type="button" className="ops-btn-secondary" onClick={() => setClosing(null)}>Cancel</button></form>}
  </section>;
}
