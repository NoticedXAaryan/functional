"""Synthetic local browser rehearsal. Never enrolls a device or sends a real push."""
import json
import uuid
from datetime import datetime, timedelta
from pathlib import Path
from urllib.request import Request, urlopen
from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parents[1]
API = 'http://127.0.0.1:8082/api/v1'
WEB = 'http://127.0.0.1:5173'
OPS = 'http://127.0.0.1:5174'
EVIDENCE = ROOT / 'artifacts' / 'evidence'
EVIDENCE.mkdir(parents=True, exist_ok=True)

def api(path, body=None, token=None, idempotency=None):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    if idempotency:
        headers['Idempotency-Key'] = idempotency
    request = Request(API + path, data=json.dumps(body).encode() if body is not None else None, headers=headers)
    with urlopen(request, timeout=15) as response:
        return json.load(response)

config = api('/subscriptions/config')
assert config['is_test'] and config['mode'] == 'TEST_ALLOWLIST', 'Requires local TEST configuration'
session = api('/sessions', {'mode': 'with_return_access', 'language': 'en'})
receipt = api('/cases', {'route_type': 'missing_child', 'account_text': '[FICTIONAL] Browser rehearsal only.'}, session['session_token'], str(uuid.uuid4()))
description = '[FICTIONAL] Savera browser rehearsal ' + uuid.uuid4().hex[:8] + '. No real child is missing.'

with sync_playwright() as playwright:
    browser = playwright.chromium.launch(headless=True)
    errors = []
    def sign_in(username):
        page = browser.new_page(viewport={'width': 1280, 'height': 1000})
        page.on('pageerror', lambda error: errors.append(str(error)))
        page.goto(OPS)
        page.wait_for_load_state('networkidle')
        page.get_by_label('Username', exact=True).fill(username)
        page.get_by_label('Password', exact=True).fill('demo-password')
        page.locator('#login-submit').click()
        expect(page.get_by_role('heading', name='Case Queue')).to_be_visible()
        page.get_by_role('button', name='Alert Review').click()
        expect(page.get_by_role('heading', name='Savera Alert')).to_be_visible()
        return page

    preparer = sign_in('preparer.demo')
    preparer.get_by_label('Missing-child report').select_option(receipt['case_id'])
    preparer.get_by_label('Area that should receive the alert').select_option('demo-nagar-north')
    preparer.get_by_label('Public description').fill(description)
    preparer.get_by_label('Organization publishing this alert').fill('[FICTIONAL] Browser Test Team')
    preparer.get_by_label('How was this report checked?').fill('Synthetic fixture reviewed for demonstration')
    preparer.get_by_label('When should the alert end?').fill((datetime.now() + timedelta(hours=2)).strftime('%Y-%m-%dT%H:%M'))
    preparer.get_by_role('button', name='Save for review').click()
    expect(preparer.get_by_role('status')).to_contain_text('Draft saved')

    approver = sign_in('approver.demo')
    card = approver.locator('article.ops-alert-card').filter(has_text=description)
    card.get_by_role('button', name='Approve alert').click()
    expect(approver.get_by_role('status')).to_contain_text('Alert approved')
    card.get_by_role('button', name='Publish TEST alert').click()
    expect(approver.get_by_role('status')).to_contain_text('Published')

    public = browser.new_page(viewport={'width': 1280, 'height': 1000})
    public.on('pageerror', lambda error: errors.append(str(error)))
    public.goto(WEB + '/alerts')
    public.wait_for_load_state('networkidle')
    public.get_by_label('Choose an area').select_option('demo-nagar-north')
    alert = public.locator('article.sa-alert').filter(has_text=description)
    expect(alert).to_be_visible()
    public.screenshot(path=str(EVIDENCE / 'savera-desktop.png'), full_page=True)
    alert.get_by_role('link', name='Read alert and share a tip').click()
    public.wait_for_load_state('networkidle')
    public.get_by_label('What did you see?').fill('[FICTIONAL] Observation used only for the local test.')
    public.get_by_label('Approximate place and time').fill('[FICTIONAL] Demo area, now')
    public.get_by_role('button', name='Send private tip').click()
    expect(public.get_by_role('status')).to_contain_text('Tip received')
    public.set_viewport_size({'width': 390, 'height': 844})
    public.screenshot(path=str(EVIDENCE / 'savera-mobile.png'), full_page=True)
    assert public.evaluate('document.documentElement.scrollWidth <= window.innerWidth'), 'Mobile overflow'

    card.get_by_role('button', name='Withdraw alert').click()
    approver.get_by_label('Brief non-identifying reason').fill('Synthetic rehearsal complete')
    approver.get_by_role('button', name='Confirm status change').click()
    expect(approver.get_by_role('status')).to_contain_text('Status updated')
    public.reload()
    expect(public.locator('.sa-state')).to_have_text('withdrawn')
    expect(public.get_by_text(description, exact=True)).to_have_count(0)
    expect(public.get_by_label('What did you see?')).to_have_count(0)
    public.screenshot(path=str(EVIDENCE / 'savera-withdrawn.png'), full_page=True)

    # Export the original vector logo into installable raster app icons.
    icon = browser.new_page()
    for size in (192, 512):
        icon.set_viewport_size({'width': size, 'height': size})
        icon.goto(WEB + '/savera.svg')
        icon.screenshot(path=str(ROOT / 'apps' / 'web' / 'public' / f'savera-{size}.png'), omit_background=True)
    browser.close()
    assert not errors, errors
print('PASS: prepare, independent approval, publication, public status, private tip, withdrawal, mobile layout; no real push sent.')
