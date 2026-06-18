// WebRTC calls (1:1 and small-group mesh) over the SSE signaling relay.
// Media is peer-to-peer; the server only relays invite/accept/offer/answer/ice/
// hangup. STUN (and optional TURN) come from /api/calls/config. Group calls use a
// full mesh with a deterministic "lower user id offers" rule to avoid glare.
//
// Verified by syntax/logic; real media + cross-NAT require a browser and a TURN
// server (configured in Settings → Integrations).

const callState = {
  es: null,
  ice: null,           // cached iceServers
  active: false,
  callId: null,
  channelId: 0,
  video: false,
  localStream: null,
  muted: false,
  camOff: false,
  peers: {},           // userId -> { pc, stream, name }
  participants: {},    // userId -> name (everyone in the call incl. self)
  ring: null,          // pending incoming invite
};

const CALLS_I18N = {
  en: { callAudio: 'Audio call', callVideo: 'Video call', callIncoming: 'Incoming call', callFrom: 'from',
        callAccept: 'Accept', callDecline: 'Decline', callHangup: 'Hang up', callMute: 'Mute', callUnmute: 'Unmute',
        callCam: 'Camera', callAddPeople: 'Add participant', callCalling: 'Calling…', callConnecting: 'Connecting…',
        callEnded: 'Call ended', callNoMedia: 'Microphone/camera unavailable', callBusy: 'You are already in a call' },
  ru: { callAudio: 'Аудиозвонок', callVideo: 'Видеозвонок', callIncoming: 'Входящий звонок', callFrom: 'от',
        callAccept: 'Принять', callDecline: 'Отклонить', callHangup: 'Завершить', callMute: 'Выкл. микрофон', callUnmute: 'Вкл. микрофон',
        callCam: 'Камера', callAddPeople: 'Добавить участника', callCalling: 'Вызов…', callConnecting: 'Соединение…',
        callEnded: 'Звонок завершён', callNoMedia: 'Микрофон/камера недоступны', callBusy: 'Вы уже в звонке' },
};
let callsI18nMerged = false;
function callsEnsureI18n() {
  if (callsI18nMerged || typeof LANGS === 'undefined') return;
  for (const lang of ['ru', 'en', 'zh', 'es', 'fr', 'de', 'pt', 'ja', 'ko', 'ar']) {
    if (!LANGS[lang]) continue;
    const dict = Object.assign({}, CALLS_I18N.en, CALLS_I18N[lang] || {});
    for (const k in dict) if (!(k in LANGS[lang])) LANGS[lang][k] = dict[k];
  }
  callsI18nMerged = true;
}
function ct(k) { callsEnsureI18n(); return (typeof t === 'function') ? t(k) : k; }

// callId generator without Math.random dependency concerns (browser-side ok).
function callNewId() { return 'c' + Date.now() + '-' + Math.floor(Math.random() * 1e6); }

// === Signaling channel (global SSE) ===
function callInit() {
  callsEnsureI18n();
  if (callState.es || typeof EventSource === 'undefined') return;
  if (typeof isServiceEnabled === 'function' && !isServiceEnabled('chat')) return;
  try {
    const es = new EventSource('/api/calls/events');
    es.onmessage = e => { try { callHandleSignal(JSON.parse(e.data)); } catch (err) { /* ignore */ } };
    es.onerror = () => { /* browser auto-reconnects */ };
    callState.es = es;
  } catch (e) { /* SSE unavailable */ }
}

async function callIceConfig() {
  if (callState.ice) return callState.ice;
  try { const c = await api('/api/calls/config'); callState.ice = c.iceServers || []; }
  catch (e) { callState.ice = [{ urls: 'stun:stun.l.google.com:19302' }]; }
  return callState.ice;
}

async function callSignal(to, kind, payload) {
  try {
    await api('/api/calls/signal', 'POST', {
      to, kind, call_id: callState.callId, channel_id: callState.channelId, payload: payload || null,
    });
  } catch (e) { /* peer may be offline */ }
}

// === Outgoing call ===
async function callStart(channelId, members, withVideo) {
  if (callState.active) { if (typeof toast === 'function') toast(ct('callBusy'), 'error'); return; }
  const peers = (members || []).filter(id => id && id !== (currentUser && currentUser.id));
  if (!peers.length) return;
  if (!await callGetMedia(withVideo)) return;
  callState.active = true;
  callState.callId = callNewId();
  callState.channelId = channelId || 0;
  callState.video = !!withVideo;
  callState.participants[currentUser.id] = currentUser.username;
  callRenderUI();
  callSetStatus(ct('callCalling'));
  for (const pid of peers) {
    callSignal(pid, 'invite', { video: !!withVideo });
  }
}

// === Incoming signaling ===
function callHandleSignal(ev) {
  if (!ev || ev.type !== 'call') return;
  const from = ev.from || {};
  switch (ev.kind) {
    case 'invite':
      callOnInvite(ev, from);
      break;
    case 'accept':
      callOnAccept(ev, from);
      break;
    case 'decline':
    case 'cancel':
    case 'hangup':
      callOnPeerLeave(ev, from);
      break;
    case 'offer':
      callOnOffer(ev, from);
      break;
    case 'answer':
      callOnAnswer(ev, from);
      break;
    case 'ice':
      callOnIce(ev, from);
      break;
  }
}

function callOnInvite(ev, from) {
  // Already in this call → treat as a new participant joining; auto-accept.
  if (callState.active && callState.callId === ev.call_id) return;
  if (callState.active) { callSignal(from.id, 'decline'); return; }
  callState.ring = { callId: ev.call_id, channelId: ev.channel_id, from, video: !!(ev.payload && ev.payload.video) };
  callRenderRing();
}

async function callAccept() {
  const r = callState.ring;
  if (!r) return;
  callState.ring = null;
  if (!await callGetMedia(r.video)) return;
  callState.active = true;
  callState.callId = r.callId;
  callState.channelId = r.channelId || 0;
  callState.video = r.video;
  callState.participants[currentUser.id] = currentUser.username;
  callState.participants[r.from.id] = r.from.name;
  callRenderUI();
  callSetStatus(ct('callConnecting'));
  // Announce join to the caller; the lower-id side will create the offer.
  callSignal(r.from.id, 'accept');
  callMaybeOffer(r.from.id, r.from.name);
}

function callDecline() {
  const r = callState.ring;
  if (!r) return;
  callState.ring = null;
  callSignal(r.from.id, 'decline');
  callRenderRing();
}

function callOnAccept(ev, from) {
  if (!callState.active || ev.call_id !== callState.callId) return;
  callState.participants[from.id] = from.name;
  callMaybeOffer(from.id, from.name);
  callRenderUI();
}

// callMaybeOffer creates a peer connection; the lower user id initiates the offer.
async function callMaybeOffer(peerId, peerName) {
  if (callState.peers[peerId]) return;
  const pc = await callNewPeer(peerId, peerName);
  if (currentUser.id < peerId) {
    try {
      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
      callSignal(peerId, 'offer', pc.localDescription);
    } catch (e) { /* ignore */ }
  }
}

async function callNewPeer(peerId, peerName) {
  if (callState.peers[peerId]) return callState.peers[peerId].pc;
  const ice = await callIceConfig();
  const pc = new RTCPeerConnection({ iceServers: ice });
  const entry = { pc, stream: new MediaStream(), name: peerName };
  callState.peers[peerId] = entry;
  if (callState.localStream) {
    callState.localStream.getTracks().forEach(tr => pc.addTrack(tr, callState.localStream));
  }
  pc.onicecandidate = e => { if (e.candidate) callSignal(peerId, 'ice', e.candidate); };
  pc.ontrack = e => {
    (e.streams[0] || new MediaStream([e.track])).getTracks().forEach(tr => {
      if (!entry.stream.getTracks().includes(tr)) entry.stream.addTrack(tr);
    });
    callRenderUI();
  };
  pc.onconnectionstatechange = () => {
    if (['failed', 'closed', 'disconnected'].includes(pc.connectionState)) callRemovePeer(peerId);
  };
  return pc;
}

async function callOnOffer(ev, from) {
  if (!callState.active || ev.call_id !== callState.callId) return;
  callState.participants[from.id] = from.name;
  const pc = await callNewPeer(from.id, from.name);
  try {
    await pc.setRemoteDescription(new RTCSessionDescription(ev.payload));
    const answer = await pc.createAnswer();
    await pc.setLocalDescription(answer);
    callSignal(from.id, 'answer', pc.localDescription);
    callRenderUI();
  } catch (e) { /* ignore */ }
}

async function callOnAnswer(ev, from) {
  const entry = callState.peers[from.id];
  if (!entry) return;
  try { await entry.pc.setRemoteDescription(new RTCSessionDescription(ev.payload)); } catch (e) { /* ignore */ }
}

async function callOnIce(ev, from) {
  const entry = callState.peers[from.id];
  if (!entry || !ev.payload) return;
  try { await entry.pc.addIceCandidate(new RTCIceCandidate(ev.payload)); } catch (e) { /* ignore */ }
}

function callOnPeerLeave(ev, from) {
  if (callState.ring && callState.ring.from && callState.ring.from.id === from.id) {
    callState.ring = null; callRenderRing(); return;
  }
  if (!callState.active) return;
  callRemovePeer(from.id);
  // If everyone left, end the call.
  if (!Object.keys(callState.peers).length) callHangup(true);
}

function callRemovePeer(peerId) {
  const entry = callState.peers[peerId];
  if (entry) { try { entry.pc.close(); } catch (e) {} delete callState.peers[peerId]; }
  delete callState.participants[peerId];
  callRenderUI();
}

// === Media + controls ===
async function callGetMedia(withVideo) {
  if (callState.localStream) return true;
  try {
    callState.localStream = await navigator.mediaDevices.getUserMedia({ audio: true, video: !!withVideo });
    return true;
  } catch (e) {
    if (typeof toast === 'function') toast(ct('callNoMedia'), 'error');
    return false;
  }
}

function callToggleMute() {
  callState.muted = !callState.muted;
  if (callState.localStream) callState.localStream.getAudioTracks().forEach(tr => tr.enabled = !callState.muted);
  callRenderControls();
}
function callToggleCam() {
  callState.camOff = !callState.camOff;
  if (callState.localStream) callState.localStream.getVideoTracks().forEach(tr => tr.enabled = !callState.camOff);
  callRenderControls();
}

function callHangup(silent) {
  if (!silent) {
    for (const pid of Object.keys(callState.peers)) callSignal(Number(pid), 'hangup');
    // Also notify invited-but-not-yet-connected ring targets.
    for (const pid of Object.keys(callState.participants)) {
      if (Number(pid) !== currentUser.id && !callState.peers[pid]) callSignal(Number(pid), 'cancel');
    }
  }
  for (const pid of Object.keys(callState.peers)) { try { callState.peers[pid].pc.close(); } catch (e) {} }
  if (callState.localStream) callState.localStream.getTracks().forEach(tr => tr.stop());
  callState.peers = {}; callState.participants = {}; callState.localStream = null;
  callState.active = false; callState.callId = null; callState.channelId = 0; callState.muted = false; callState.camOff = false;
  const ui = document.getElementById('call-ui'); if (ui) ui.remove();
}

// callAddPeople invites more users mid-call (even outside the channel).
function callAddPeople() {
  if (!callState.active) return;
  const inCall = callState.participants;
  const users = (state.users || []).filter(u => u.id !== (currentUser && currentUser.id) && !inCall[u.id]);
  if (!users.length) return;
  const ov = document.createElement('div');
  ov.className = 'modal-overlay';
  ov.innerHTML = `<div class="modal"><div class="modal-header"><h2>${ct('callAddPeople')}</h2>
      <button class="btn-icon" onclick="this.closest('.modal-overlay').remove()">&#10005;</button></div>
    <div class="modal-body"><div class="chat-member-pick">${users.map(u =>
      `<label class="chat-pick-row"><input type="checkbox" value="${u.id}"> ${esc(u.username)}</label>`).join('')}</div></div>
    <div class="modal-footer"><button class="btn btn-primary" onclick="callDoAddPeople(this)">${ct('callAddPeople')}</button></div></div>`;
  document.body.appendChild(ov);
}
function callDoAddPeople(btn) {
  const ov = btn.closest('.modal-overlay');
  ov.querySelectorAll('input:checked').forEach(i => callSignal(Number(i.value), 'invite', { video: callState.video }));
  ov.remove();
}

// === UI ===
function callRenderRing() {
  let el = document.getElementById('call-ring');
  if (!callState.ring) { if (el) el.remove(); return; }
  if (!el) { el = document.createElement('div'); el.id = 'call-ring'; el.className = 'call-ring'; document.body.appendChild(el); }
  const r = callState.ring;
  el.innerHTML = `<div class="call-ring-box">
      <div class="call-ring-ico">${r.video ? '\u{1F4F9}' : '\u{1F4DE}'}</div>
      <div class="call-ring-txt"><b>${esc(r.from.name || '?')}</b><br><span>${ct('callIncoming')}</span></div>
      <div class="call-ring-actions">
        <button class="btn call-accept" onclick="callAccept()">${ct('callAccept')}</button>
        <button class="btn call-decline" onclick="callDecline()">${ct('callDecline')}</button>
      </div></div>`;
}

function callRenderUI() {
  if (!callState.active) return;
  let ui = document.getElementById('call-ui');
  if (!ui) {
    ui = document.createElement('div');
    ui.id = 'call-ui';
    ui.className = 'call-ui';
    ui.innerHTML = `<div class="call-grid" id="call-grid"></div>
      <div class="call-status" id="call-status"></div>
      <div class="call-controls" id="call-controls"></div>`;
    document.body.appendChild(ui);
  }
  const grid = ui.querySelector('#call-grid');
  // Build/refresh tiles: local + each peer.
  const tiles = [];
  tiles.push(callTileHtml('local', currentUser.username + ' (вы)'));
  for (const pid of Object.keys(callState.peers)) tiles.push(callTileHtml('peer-' + pid, callState.peers[pid].name || '?'));
  grid.innerHTML = tiles.join('');
  // Attach streams to video elements.
  const localV = grid.querySelector('[data-tile="local"] video');
  if (localV && callState.localStream) { localV.srcObject = callState.localStream; localV.muted = true; }
  for (const pid of Object.keys(callState.peers)) {
    const v = grid.querySelector('[data-tile="peer-' + pid + '"] video');
    if (v) v.srcObject = callState.peers[pid].stream;
  }
  callRenderControls();
}

function callTileHtml(key, label) {
  return `<div class="call-tile" data-tile="${key}">
      <video autoplay playsinline></video>
      <span class="call-tile-name">${esc(label)}</span></div>`;
}

function callSetStatus(text) {
  const el = document.getElementById('call-status');
  if (el) el.textContent = text || '';
}

function callRenderControls() {
  const el = document.getElementById('call-controls');
  if (!el) return;
  el.innerHTML = `
    <button class="call-ctl" title="${callState.muted ? ct('callUnmute') : ct('callMute')}" onclick="callToggleMute()">${callState.muted ? '\u{1F507}' : '\u{1F3A4}'}</button>
    ${callState.video ? `<button class="call-ctl" title="${ct('callCam')}" onclick="callToggleCam()">${callState.camOff ? '\u{1F6AB}' : '\u{1F4F9}'}</button>` : ''}
    <button class="call-ctl" title="${ct('callAddPeople')}" onclick="callAddPeople()">\u{2795}</button>
    <button class="call-ctl call-end" title="${ct('callHangup')}" onclick="callHangup()">\u{1F4DE}</button>`;
}
