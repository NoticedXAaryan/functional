// Deterministic documentation/asset generator. No network, app API or real data.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const root = path.dirname(fileURLToPath(import.meta.url));
const write = (name, value) => { const p = path.join(root, name); fs.mkdirSync(path.dirname(p), {recursive:true}); fs.writeFileSync(p, value); };
const escape = s => String(s).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&apos;'}[c]));
const icons = {
  help:'<path d="M5 17H3V5h18v12H10l-5 4z"/><path d="M9 9a3 3 0 0 1 6 0c0 2-3 2-3 4"/><path d="M12 15h.01"/>',
  messages:'<path d="M3 4h16v12H8l-5 4z"/><path d="M7 8h8M7 12h5"/>',
  missing:'<circle cx="9" cy="7" r="3"/><path d="M3 19v-2a6 6 0 0 1 8-5"/><circle cx="17" cy="16" r="4"/><path d="m20 19 2 3"/>',
  alerts:'<path d="M6 17V9a6 6 0 0 1 12 0v8l2 2H4zM10 22h4"/>',
  microphone:'<rect x="8" y="2" width="8" height="13" rx="4"/><path d="M5 11v1a7 7 0 0 0 14 0v-1M12 19v3M8 22h8"/>',
  play:'<path d="m8 4 12 8-12 8z"/>',
  pause:'<path d="M8 4v16M16 4v16"/>',
  stop:'<rect x="5" y="5" width="14" height="14" rx="2"/>',
  listen:'<path d="m4 9 4 0 5-5v16l-5-5H4zM17 8a6 6 0 0 1 0 8M20 5a10 10 0 0 1 0 14"/>',
  location:'<path d="M19 10c0 5-7 12-7 12S5 15 5 10a7 7 0 0 1 14 0z"/><circle cx="12" cy="10" r="2"/>',
  back:'<path d="m10 5-7 7 7 7M3 12h18"/>',
  next:'<path d="m14 5 7 7-7 7M21 12H3"/>',
  exit:'<path d="M10 3H3v18h7M9 12h12m-5-5 5 5-5 5"/>',
  language:'<path d="M2 5h12M8 2v3M4 5c0 5 4 9 9 11M12 5c0 5-4 9-9 11M14 22l4-11 4 11M16 18h4"/>',
  check:'<path d="m4 12 5 5L20 6"/>',
  retry:'<path d="M20 7V2l-3 3a9 9 0 1 0 3 11M20 7h-5"/>',
  close:'<path d="m5 5 14 14M19 5 5 19"/>',
  privacy:'<rect x="4" y="10" width="16" height="12" rx="2"/><path d="M7 10V7a5 5 0 0 1 10 0v3M12 15v3"/>',
  edit:'<path d="m4 15-1 6 6-1L21 8l-5-5zM13 6l5 5"/>',
  send:'<path d="m3 3 18 9-18 9 3-9zM6 12h15"/>',
  person:'<circle cx="12" cy="7" r="4"/><path d="M4 22v-3a8 8 0 0 1 16 0v3"/>',
  clock:'<circle cx="12" cy="12" r="9"/><path d="M12 6v6l4 2"/>',
  share:'<circle cx="18" cy="4" r="3"/><circle cx="5" cy="12" r="3"/><circle cx="18" cy="20" r="3"/><path d="m8 10 7-4M8 14l7 4"/>',
  bridge:'<path d="M2 17Q12 3 22 17M2 21h20M4 15v6M8 11v10M12 10v11M16 11v10M20 15v6"/>',
  boat:'<path d="m3 16 4 5h10l4-5zM12 3v13M12 4l7 9h-7M9 7l-5 6h5"/>',
  cloud:'<path d="M6 19a5 5 0 0 1-1-10 7 7 0 0 1 13-2 6 6 0 0 1 0 12z"/>',
  mango:'<path d="M17 5C8 2 2 9 4 16c2 8 14 6 16-2 1-4-1-7-3-9zM15 5c0-3 4-4 7-3-1 3-4 4-7 3"/>',
  leaf:'<path d="M21 3C7 1 1 9 5 17s18 3 16-14zM5 20 17 7M10 15v-5M14 11h5"/>',
  moon:'<path d="M20 15A9 9 0 0 1 9 3a9 9 0 1 0 11 12z"/>',
  star:'<path d="m12 2 3 6 7 1-5 5 1 7-6-3-6 3 1-7-5-5 7-1z"/>',
  tree:'<path d="M12 2 4 12h4l-5 6h18l-5-6h4zM10 18v4h4v-4"/>',
  book:'<path d="M12 5C9 2 5 2 2 3v16c4-1 7-1 10 2 3-3 6-3 10-2V3c-3-1-7-1-10 2zM12 5v16"/>',
  cup:'<path d="M3 5h13v11a5 5 0 0 1-5 5H8a5 5 0 0 1-5-5zM16 7h2a4 4 0 0 1 0 8h-2"/>',
  kite:'<path d="m12 2 8 8-8 8-8-8zM12 2v16M4 10h16M12 18c-5 0-3 5 2 4"/>',
  ball:'<circle cx="12" cy="12" r="10"/><path d="M5 5c8 2 12 6 14 14M19 5C11 7 7 11 5 19"/>',
  fish:'<path d="M3 12c4-8 12-8 16-2l3-3v10l-3-3C15 20 7 20 3 12zM14 9h.01"/>',
};
for (const [name, body] of Object.entries(icons)) write(`assets/icons/${name}.svg`, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#603b91" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${body}</svg>\n`);
write('assets/icons/sprite.svg', `<svg xmlns="http://www.w3.org/2000/svg">${Object.entries(icons).map(([name,body])=>`<symbol id="${name}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${body}</symbol>`).join('')}</svg>`);

// ID, route, title, helper, ordered content, primary label, primary action, secondary label/action, dependency.
const rows = [
['C01','/','You can ask for help here.','You can type, use your voice, or choose a button.','Ask for help|My messages|Someone is missing|Nearby alerts','Ask for help','navigate:C02','My messages → C08','public_config'],
['C02','/help/start','What would you like help with?','You do not need to know what to call it.','Something happened to me|Something online worries me|I am worried about someone|I am not sure','Next','create_private_session_then:C03','Ask for a different team → C10','intake_open_and_real_routing'],
['C03','/help/message','What would you like us to know?','Share as much or as little as you want.','Type a message|Record my voice|Just ask for help','Next','save_memory_draft_then:C04','Back → C02','intent_only_case_api;voice_optional'],
['C04','/help/replies','Would you like messages here?','We will not send notifications about this conversation.','Yes, when I come back|Send this without replies','Next','save_explicit_reply_choice_then:C05','Back → C03','reply_preference_v2'],
['C05','/help/review','Check what you are sending','This goes to {operatorName}.','Your choices — Change|Your message — Change|Your reply choice — Change','Send to the support team','commit_case_idempotently_then:C06','Back → C04','private_case_commit'],
['C06','/help/received','Your message was received.','It was saved for the team. They may not have read it yet.','You choose how to come back|You can also leave now','Choose how to come back','navigate:C07','Leave this page → quick_exit','durable_receipt'],
['C07','/help/return-setup','Choose how to come back','Only save access on a device that is safe for you.','Use this device again|Use a return card|Continue without saving access','Continue','branch_to_device_words_or_card_then:C09','Not now → C09','secure_return_device_and_card'],
['C08','/my-messages','Open your messages','Your words work with the key saved on this device.','First word|Second word|Third word','Open my messages','exchange_device_proof_then:C09','Use my return card → card_exchange','secure_return_device_and_card'],
['C09','/conversation','Your messages','You can send a message now. A support worker may reply later.','Message history from API|Type a message|Record my voice','Send message','commit_message_and_refresh_history','My reply choice → C04_in_settings_mode','private_messages'],
['C10','/help/another-team','Would you like a different team?','You can ask for help from someone outside this place.','Yes|No|I am not sure|Skip','Continue','resolve_independent_team_then_return_to_origin','Back → originating screen','independent_routing'],
['C11','/help/options','Other ways to get help','Calling may appear in your phone’s call history.','Call 1098 for child support|Call 112 for immediate danger','Back to my message','navigate:originating_screen','Leave this page → quick_exit','verified_local_resources'],
['C12','/privacy','You choose what to share','You do not need to give your name.','Who can read or hear it|What stays on this device|How to leave and come back','Back','navigate:originating_screen','Listen → explicit_local_or_public_audio','reviewed_privacy_copy'],
['M01','/missing/start','Who are you worried about?','Tell us what you know. It is okay to be unsure.','A child is missing|I saw a child who may need help|I am lost','Next','missing_then:M02;lost_then:C02;concern_then:C03','Back → C01','real_routing'],
['M02','/missing/message','What do you know?','A short message is enough to start.','Type a message|Record my voice|Help me explain','Next','save_memory_draft_then:M03','Back → M01','minimal_private_intake'],
['M03','/missing/place','Where were they last seen?','Your phone may be somewhere different.','Choose a place|Use my location once|I do not know','Next','confirm_last_seen_place_then:M04','Back → M02','supported_areas;location_optional'],
['M04','/missing/review','Check what you are sending','The team reviews this before sharing an alert.','Your message — Change|Place and time — Change|Receiving team','Send privately','commit_private_missing_report_then:M05','Back → M03','private_case_commit'],
['M05','/missing/received','Your message was received.','This has not automatically created a public alert.','Choose how to come back|Other ways to get help','Choose how to come back','navigate:C07','Other ways to get help → C11','durable_receipt'],
['A01','/alerts','Nearby missing-child alerts','Choose an area to read current alerts.','Search area|Use my location once|Current alerts from API','Choose an area','select_area_and_refresh_feed','Get alerts for my area → A05','public_alerts_and_areas'],
['A02','/alerts/{id}','Read this alert','Use the latest details shown here.','Reviewed details from API|Area and last seen time|Issuer and last updated time','I saw something','navigate:A03','Share this link → native_share_or_copy_after_tap','active_public_alert'],
['A03','/alerts/{id}/sighting','What did you see?','Do not follow or confront anyone.','Type or record what you saw|Approximate place and time — optional','Send privately','commit_tip_idempotently_then:A04','Back → A02','active_alert_private_tip_api'],
['A04','/alerts/{id}/sighting/received','Your message was received.','The reviewing team may not have read it yet.','Receipt saved by server','Back to the alert','navigate:A02','Share another observation → A03_new_operation','durable_tip_receipt'],
['A05','/alerts/notifications','Get alerts for this area','Public alerts may appear on your lock screen.','Choose an area|Confirm notification consent|Device support guidance when needed','Allow notifications','request_permission_then_register_subscription','Not now → A01','enrollment_available_and_supported_device'],
['A06','/alerts/settings','Your alert settings','These settings only affect this browser.','Saved area from API|Permission state|Change area','Stop notifications','revoke_server_then_unsubscribe_browser','Back to alerts → A01','existing_subscription_management'],
['S01','/sign-in','Staff sign in','Use your own staff account.','Username or organization sign-in|Password when session auth is deployed|Show password','Sign in','deployed_auth_then:S02','Back to Bal Setu → public_home','deployed_staff_auth'],
['S02','/cases','My work','Open a request to see what needs attention.','New|Waiting for me|Waiting for child|Escalated|Authorized queue from API','Open request','select_authorized_case_then:S03','Alert review → S10','staff_case_scope'],
['S03','/cases/{id}','Request details','Read or listen before responding.','Original message|Reply permission|Team and assignment|Actual timeline','Open conversation','navigate:S06','Assign → S04; Accept → S05 when eligible','case_permission'],
['S04','/cases/{id}/assign','Choose a responder','Choose someone who can take responsibility.','Eligible active responders from API','Assign request','atomic_assignment_then:S03','Cancel → S03','supervisor_scope'],
['S05','/assignments/{id}/accept','Accept this request','Your acceptance tells the team you are taking responsibility.','Case and assignment summary','Accept request','atomic_acceptance_then:S06','Back → S03','current_assignment'],
['S06','/cases/{id}/conversation','Conversation','Sound may be heard by people nearby.','Actual message history|Reply to child / Private team note|Type or record|Text equivalent for a voice reply','Send reply','scoped_message_commit','Transfer or escalate → S07','case_permission_and_reply_consent'],
['S07','/cases/{id}/transfer','Ask another team for help','Keep responsibility until the receiving team accepts.','Authorized receiving team|Reason|Minimum necessary summary','Request transfer','create_transfer_and_show_pending_ack','Cancel → S03','authorized_transfer_targets'],
['S08','/cases/{id}/close','Close this request','Closing a request does not mean the child is no longer at risk.','Reason|Child-visible message if allowed|Remaining support options','Close request','confirmed_closure_then:S03','Cancel → S03','closure_permission'],
['S09','/alerts/{id}/tips','Private sightings','These observations are not public.','Authorized tips from API|Listen on tap|Follow-up status','Mark reviewed','versioned_tip_review','Back to alert → S12','issuer_tip_permission'],
['S10','/alerts/new','Prepare a public alert','Only approved details will be shared.','Case selection|Minimum public description|Reviewed area|Expiry|Verification basis|Public preview','Send for review','create_revision_then:S11_pending','Save draft → persist_unpublished_draft','alert_preparer'],
['S11','/alerts/{id}/review','Review this alert','Check the exact information people will see.','Public preview|Private verification basis|Area and expiry|Publication risks','Approve this revision','independent_approval_then:S12','Reject with reason → revision_rejected','independent_alert_approver'],
['S12','/alerts/{id}/manage','Manage this alert','Publication and notification delivery are different states.','Approved preview|Audience estimate|Delivery state|Expiry','Publish alert','confirm_then_atomic_activate','Withdraw / Resolve → terminal_state_confirmation','approved_immutable_revision'],
['S13','/settings/service','Service settings','Only display coverage the team can actually provide.','Operator details|Intake availability|Independent routing|Reviewed languages|Feature states','Save settings','authorized_versioned_config_update','Cancel → S02','operator_admin'],
['S14','/settings/access','Staff and area access','Keep access limited to current responsibilities.','Active people and roles|Organizations|Reviewed areas|Entry points','Save changes','audited_role_or_area_mutation','Cancel → S02','access_admin'],
['S15','/sign-out','End staff access','Signing out ends access on this browser.','Unsaved work warning if relevant','Sign out','revoke_session_clear_private_memory_then:S01','Cancel → originating_staff_screen','staff_session'],
];
const common = {
  sourceStatus:'Design specification only; no real reports or success states are seeded.',
  layout:{phoneWidth:390,minWidth:320,contentMaxWidth:640,gutterPhone:16,gutterDesktop:24,primaryMinHeight:56,choiceMinHeight:80,gap:16,bodyFontPx:18},
  header:['Bal Setu','Language','Leave this page'],
  loading:'Connecting to the support service…',
  offline:'This has not been sent. Try again when you have internet.',
  unknownCommit:'We could not confirm it yet. Try again to check.',
  retry:'Try again',
  emptyFeed:'No alerts are published for this area right now.',
  emptyMessages:'No messages yet. You can leave a message here.',
};
const exceptions = {
 C01:'Render exactly four navigation actions. Ask for help is the first purple action, not a second button below the four choices. No Back control on the home route. Hide redundant My messages secondary action because it already appears in the four choices.',
 C04:'Initial flow: require an explicit choice, then Next to review. Opened from an existing conversation: show current preference, primary becomes Save reply choice; PATCH the preference with expected version and return to C09. Never create a new case from settings.',
 C05:'The Change controls navigate to the corresponding step and preserve all other fields. Escape report text as plain text. If operatorName is unavailable, do not show the template or allow Send. No receipt until commit.',
 C06:'Do not show Back if it could submit the report again. This screen requires a real receipt. Reload without an active session goes to return access, not a fabricated receipt.',
 C07:'Branch into device or card setup exactly as DECISIONS.md describes. Choose each of the three word slots and confirm them. Not now bypasses persistence. No static example word is a credential. Do not enable this flow until secure backend and reviewed vocabulary exist.',
 C08:'If no device proof is present, explain that words alone cannot open messages and promote Use my return card. Provide existing secure-code entry during migration. Wrong words, expired and revoked records share a generic error.',
 C09:'Only committed API messages populate history. Send requires text or ready audio. When closed, replace composer with read-only history and Ask for help again. Changing reply choice opens C04 in settings mode. No private push.',
 C10:'Store the originating route. Yes requests independent routing; No or Skip retains the existing routing choice. Not sure explains the independent option without accusing the venue. If no independent team is available, show verified alternatives before collecting more information.',
 C11:'Each phone number is its own tel link with a visible number and purpose. Do not turn Back into a call. Static resource content does not need a fake loading state.',
 C12:'This is static reviewed content with real operator details. No fake network request or auth gate. Never autoplay Listen.',
 M01:'A child is missing goes to M02. I saw a child who may need help opens the concern flow; it must not require a known alert ID. I am lost opens child help with optional safe location and no public posting.',
 M03:'Use my location once is not automatically last-seen location. Always ask the reporter to confirm/correct it. I do not know is valid.',
 M05:'Private receipt only. Do not offer public sharing of the report or a fabricated alert link.',
 A01:'No area chosen can read the actual available feed; enrollment requires a chosen area. Choose an area is a selection control, not a Send request. Cards contain real active alerts or an honest empty state.',
 A02:'Only ACTIVE unexpired current alerts show identifying details and I saw something. Terminal alerts instead show ended text; no share-identifying-details or sighting action. Unknown status hides stale details and offers Retry.',
 A04:'Require a real tip receipt; no Back leading to accidental resubmission. Share another observation creates a new operation only after the previous operation is confirmed.',
 A05:'Require area and explicit consent before OS permission. Unsupported/denied state offers reading and device-specific instructions. Never call notifications on until both browser and server registration succeed.',
 A06:'Stop stays available when enrollment is disabled. If only browser unsubscribe or only server revoke succeeds, explain partial completion and Retry. Change area commits on server before changing the confirmed display.',
 S01:'No Sign out control before sign-in. Use the real deployed auth method; do not render both unrelated login forms. Recovery must be supported by that method, not a dead link.',
 S02:'The Open request action lives on each actual authorized queue item, not an orphan footer button with no selected case. Empty queue has no invented request.',
 S06:'Private team note mode changes primary label to Save team note and uses note visibility. Reply mode requires child permission; voice reply requires a typed equivalent. A note can never be sent to the child by changing CSS only.',
 S10:'A pending/approved revision cannot be silently edited in place. Public photo/description needs actual verification, never generated child imagery.',
 S11:'Preparer cannot approve. If revision is already approved/rejected, show its actual state and allowed next action, not an always-enabled Approve button.',
 S12:'Publish only for the approved unpublished revision. ACTIVE shows Withdraw and Resolve, plus real delivery state. Terminal states remove activation controls. Each destructive state change has confirmation and server permission checks.',
 S13:'Only authorized operator sees these controls. Missing settings are unknown, not defaults that enable intake. Use expected versions to avoid overwriting other operator changes.',
 S14:'Only authorized access admin sees editable grants/areas. Role removal must invalidate dependent access. Do not fabricate an eligible responder when list is empty.',
 S15:'Use real revocation. On failure hide local private state but say remote sign-out could not be confirmed; do not continue showing case details.',
};
const screens = rows.map(([id,route,title,helper,content,primaryLabel,action,secondary,dependency])=>({id,route,title,helper,content:content.split('|'),primary:{label:primaryLabel,action,alreadyInContent:id==='C01'},secondary,dependency,specificRules:exceptions[id]??'Use the domain-specific validation and permission requirements in the parent UI specification.',layout:id.startsWith('S')?'staff_list_detail':'child_single_column',states:['loading','ready','empty_if_applicable','permission_denied','offline','submission_pending','submission_unconfirmed','server_rejected','expired_access'],successRule:'Never infer success from time elapsed, a click, or a local state toggle. Use the actual committed API outcome.'}));
write('screens.json', JSON.stringify({version:1,common,screens},null,2)+'\n');
write('copy.en.json',JSON.stringify({locale:'en',status:'design_copy_requires_operator_review',common,...Object.fromEntries(screens.map(s=>[s.id,{title:s.title,helper:s.helper,choices:s.content.map(c=>c.replace(/ from API/g,'')),primary:s.primary.label,secondaryLabels:s.secondary.split(';').map(c=>c.split('→')[0].trim())}]))},null,2)+'\n');
for (const s of screens) {
  // Append exact state-dependent exceptions after the common visual recipe below.
  write(`screens/${s.id}.md`, `# ${s.id} · ${s.title}\n\nTarget route: \`${s.route}\`. Layout: **${s.layout}**. Design only.\n\n## Build in this exact visual order\n\n1. Shared header: Bal Setu, Language, Leave this page. Staff shell uses role-appropriate navigation and Sign out instead of a private-help return flow.\n2. Back control to the actual previous step, preserving the in-memory draft.\n3. Heading: **${s.title}**.\n4. Supporting text: “${s.helper}”\n${s.content.map((c,i)=>`${i+5}. ${c}.`).join('\n')}\n${s.content.length+5}. Primary button: **${s.primary.label}**.\n${s.content.length+6}. Secondary actions: ${s.secondary}.\n${s.content.length+7}. Footer: Other ways to get help, Privacy, Staff sign in (public screens only).\n\n## Required behaviour\n\nPrimary action contract: \`${s.primary.action}\`. Required capability: \`${s.dependency}\`. Replace braced values only with authorized API data. If required real data is absent, show a truthful unavailable state; never display the braces or invent a value.\n\n${s.successRule}\n\nKeep input labels visible; do not turn headings into placeholder text. Buttons are semantic buttons, navigation is a link, mutually exclusive choices use a labelled radio group or equivalent accessible single-select pattern. Do not ship the strings “from API”, action-contract tokens or dependency names as user content. They are instructions to the implementer.\n\n## States to implement\n\n- Loading: bounded wait, then Retry.\n- Empty: specific explanation and next useful action; no invented records.\n- Offline: retain unsent data only in memory; say it has not been sent.\n- Submission unconfirmed: retry the same operation ID and payload.\n- Permission denied: explain the available alternative; do not retry OS prompts automatically.\n- Server rejection: preserve draft, translate stable error code, focus relevant field.\n- Expired access: hide private content, stop media, open return/sign-in flow.\n- Quick exit: stop capture/playback and discard private memory immediately.\n\n## Reference and limits\n\nUse tokens.css and the corresponding visual board. The board is composition guidance only; it is not a screenshot of a working service. Consult ../.. documents 02–06 for domain-specific validation and authorization. For ${s.id}, use the exact ID in the release checklist.\n`);
  fs.appendFileSync(path.join(root, `screens/${s.id}.md`), `\n## Screen-specific rules override the generic recipe\n\n${s.specificRules}\n`);
}
const wrap = (str, max=31) => { const result=[]; let line=''; for(const w of str.split(' ')){if((line+' '+w).trim().length>max && line){result.push(line);line=w;}else line=(line+' '+w).trim();} if(line)result.push(line);return result;};
const t=(x,y,str,size=18,color='#242238',weight=400)=>`<text x="${x}" y="${y}" font-family="Arial, sans-serif" font-size="${size}" fill="${color}" font-weight="${weight}">${escape(str)}</text>`;
const rect=(x,y,w,h,fill,r=14)=>`<rect x="${x}" y="${y}" width="${w}" height="${h}" rx="${r}" fill="${fill}"/>`;
for(let start=0;start<screens.length;start+=6){
 const batch=screens.slice(start,start+6); let svg=rect(0,0,1320,1880,'#eae7f0',0)+t(40,45,'BAL SETU · SCREEN COMPOSITION REFERENCE',26,'#603b91',700)+t(40,78,'Design only · Use real state and data · Never reproduce instruction text as UI',17);
 batch.forEach((s,i)=>{let x=40+(i%3)*430,y=110+Math.floor(i/3)*870;svg+=t(x,y,s.id+' · '+s.route,16,'#603b91',700);y+=20;svg+=rect(x,y,390,820,'#fafbff',24)+rect(x,y,390,70,'#ffffff',24)+t(x+20,y+42,'Bal Setu',22,'#603b91',700)+t(x+220,y+40,'Language',14)+t(x+310,y+40,s.id.startsWith('S')?'Sign out':'Leave',14,'#603b91',700);let yy=y+114;for(const line of wrap(s.title,25)){svg+=t(x+20,yy,line,27,'#242238',700);yy+=34;}yy+=6;for(const line of wrap(s.helper,36)){svg+=t(x+20,yy,line,17,'#595668');yy+=25;}yy+=20;for(const [j,c]of s.content.slice(0,5).entries()){const lines=wrap(c,31);const h=Math.max(64,lines.length*23+24);const primaryHere=s.primary.alreadyInContent&&j===0;svg+=rect(x+20,yy,350,h,primaryHere?'#603b91':['#f0eaf8','#eaf3fc','#fff0e7'][j%3]);lines.forEach((line,k)=>svg+=t(x+36,yy+29+k*23,line,17,primaryHere?'#ffffff':'#242238'));yy+=h+12;}if(!s.primary.alreadyInContent){svg+=rect(x+20,y+680,350,62,'#603b91',12);wrap(s.primary.label,32).forEach((line,k)=>svg+=t(x+36,y+716+k*20,line,17,'#ffffff',700));}svg+=t(x+20,y+780,'Other ways to get help',15,'#603b91');});
 write(`boards/board-${String(start/6+1).padStart(2,'0')}.svg`,`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1320 1880" width="1320" height="1880">${svg}</svg>`);
}
const gallery=`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Bal Setu asset reference</title><style>body{margin:0;padding:32px;background:#fafbff;color:#242238;font:18px/1.5 system-ui}main{max-width:1100px;margin:auto}h1{color:#603b91}.icons{display:grid;grid-template-columns:repeat(auto-fit,minmax(110px,1fr));gap:16px}.icon{background:#f0eaf8;padding:20px;border-radius:16px;text-align:center}.icon img{width:32px;height:32px}figure{margin:32px 0}img.board{width:100%;height:auto}small{display:block}a{color:#603b91}</style><main><h1>Bal Setu · implementation resources</h1><p>Design reference only. These are not working application screens.</p><figure><img src="assets/welcome-bridge.png" width="480" style="max-width:100%;height:auto" alt="Decorative purple bridge with teal leaves"><figcaption>Optional welcoming illustration; no essential information.</figcaption></figure><figure><img src="assets/bal-setu-app-icon-master.png" width="128" height="128" alt="Simplified Bal Setu icon"><figcaption>App-icon master derived from the supplied logo.</figcaption></figure><h2>Interface and word-picture icons</h2><div class="icons">${Object.keys(icons).map(n=>`<div class="icon"><img src="assets/icons/${n}.svg" alt=""><small>${n}</small></div>`).join('')}</div><h2>All screen compositions</h2>${Array.from({length:Math.ceil(screens.length/6)},(_,i)=>`<figure><img class="board" src="boards/board-${String(i+1).padStart(2,'0')}.svg" alt="Screen composition board ${i+1}"><figcaption>Board ${i+1}; see screens.json and screens/*.md for full behaviour.</figcaption></figure>`).join('')}</main></html>`;
write('gallery.html', gallery);
write('asset-manifest.json',JSON.stringify({version:1,assets:[...Object.keys(icons).map(name=>({id:name,path:`assets/icons/${name}.svg`,kind:'editable_svg_icon',dimensions:'24x24 viewBox',alt:'Empty when adjacent text labels the control',placement:'Use only with visible action text',origin:'Original vector UI geometry authored for this project'})),{id:'bal-setu-app-icon-master',path:'assets/bal-setu-app-icon-master.png',kind:'generated_raster_logo_derivative',alt:'Bal Setu when not accompanied by a text wordmark',placement:'Header 40px; app icon source',origin:'Built-in image edit of user supplied logo; raster master'}, {id:'welcome-bridge',path:'assets/welcome-bridge.png',kind:'generated_raster_illustration',alt:'',placement:'Optional home decoration, max 320 CSS px; hide if it displaces primary help action',origin:'Built-in image generation; see ASSETS.md for prompt'}],screenCount:screens.length,boardCount:Math.ceil(screens.length/6)},null,2)+'\n');
console.log(`Created ${Object.keys(icons).length} icons, ${screens.length} screen recipes, ${Math.ceil(screens.length/6)} boards, copy catalog and manifest.`);
