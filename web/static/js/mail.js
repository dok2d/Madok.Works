// Mail service frontend (internal mail). Relies on shared helpers from the inline
// script (api, t, esc, escAttr, fmtDate, currentUser, state, setRailBadge).

let mailState = {
  folder: 'inbox',
  messages: [],
  counts: {}, // folder -> {unread,total}
  thread: [], // open thread messages
  openBoxId: null,
  view: 'list', // list | message
  search: '',
  compose: { attachments: [] },
  es: null,
};

const MAIL_FOLDERS = ['inbox', 'sent', 'drafts', 'trash'];

const MAIL_I18N = {
  en: {
    mailCompose: 'Compose', mailInbox: 'Inbox', mailSent: 'Sent', mailDrafts: 'Drafts', mailTrash: 'Trash',
    mailTo: 'To', mailCc: 'Cc', mailSubject: 'Subject', mailBody: 'Message', mailSend: 'Send',
    mailCancel: 'Cancel', mailReply: 'Reply', mailDelete: 'Delete', mailMove: 'To trash',
    mailSearchPh: 'Search mail…', mailNoMessages: 'No messages', mailNoSubject: '(no subject)',
    mailAttach: 'Attach', mailAttachments: 'Attachments', mailDeleteConfirm: 'Delete this message permanently?',
    mailFrom: 'From', mailReadonly: 'Read-only access', mailRecipientsRequired: 'Add at least one recipient',
    mailToPlaceholder: 'username, username…', mailFlag: 'Flag', mailUnflag: 'Unflag', mailBack: 'Back',
  },
  ru: {
    mailCompose: 'Написать', mailInbox: 'Входящие', mailSent: 'Отправленные', mailDrafts: 'Черновики', mailTrash: 'Корзина',
    mailTo: 'Кому', mailCc: 'Копия', mailSubject: 'Тема', mailBody: 'Сообщение', mailSend: 'Отправить',
    mailCancel: 'Отмена', mailReply: 'Ответить', mailDelete: 'Удалить', mailMove: 'В корзину',
    mailSearchPh: 'Поиск почты…', mailNoMessages: 'Нет писем', mailNoSubject: '(без темы)',
    mailAttach: 'Прикрепить', mailAttachments: 'Вложения', mailDeleteConfirm: 'Удалить письмо навсегда?',
    mailFrom: 'От', mailReadonly: 'Только чтение', mailRecipientsRequired: 'Укажите хотя бы одного получателя',
    mailToPlaceholder: 'логин, логин…', mailFlag: 'Пометить', mailUnflag: 'Снять метку', mailBack: 'Назад',
  },
};
let mailI18nMerged = false;
function ensureMailI18n() {
  if (mailI18nMerged || typeof LANGS === 'undefined') return;
  for (const lang of ['ru', 'en', 'zh', 'es', 'fr', 'de', 'pt', 'ja', 'ko', 'ar']) {
    if (!LANGS[lang]) continue;
    const dict = MAIL_I18N[lang] || MAIL_I18N.en;
    for (const k in dict) if (!(k in LANGS[lang])) LANGS[lang][k] = dict[k];
  }
  mailI18nMerged = true;
}

function mailFolderLabel(f) { return t('mail' + f[0].toUpperCase() + f.slice(1)); }

// === Entry point ===
async function renderMailView() {
  ensureMailI18n();
  mailState.view = 'list';
  await mailLoadCounts();
  mailRenderLayout();
  mailConnectSSE();
  await mailLoadFolder(mailState.folder);
}

async function mailLoadCounts() {
  try {
    const arr = await api('/api/mail/folders');
    mailState.counts = {};
    for (const c of arr) mailState.counts[c.folder] = c;
  } catch (e) { mailState.counts = {}; }
}

function mailRenderLayout() {
  const canWrite = currentUser && currentUser.role !== 'readonly';
  document.getElementById('app').innerHTML = `
  <div class="mail-layout">
    <aside class="mail-side">
      ${canWrite ? `<button class="btn btn-primary mail-compose" onclick="mailComposeDialog()">${t('mailCompose')}</button>` : ''}
      <input class="mail-search" id="mail-search" placeholder="${t('mailSearchPh')}" autocomplete="off">
      <div class="mail-folders" id="mail-folders"></div>
    </aside>
    <main class="mail-main" id="mail-main"></main>
  </div>`;
  mailRenderFolders();
  const se = document.getElementById('mail-search');
  if (se) {
    let tmo = null;
    se.addEventListener('input', function () { clearTimeout(tmo); tmo = setTimeout(() => mailDoSearch(this.value), 300); });
    se.addEventListener('keydown', function (e) { if (e.key === 'Escape') { this.value = ''; mailDoSearch(''); } });
  }
}

function mailRenderFolders() {
  const el = document.getElementById('mail-folders');
  if (!el) return;
  el.innerHTML = MAIL_FOLDERS.map(f => {
    const c = mailState.counts[f] || {};
    const unread = f === 'inbox' && c.unread > 0 ? `<span class="mail-fcount">${c.unread}</span>` : '';
    return `<div class="mail-frow ${mailState.folder === f && mailState.view === 'list' && !mailState.search ? 'active' : ''}" onclick="mailOpenFolder('${f}')">
      <span>${esc(mailFolderLabel(f))}</span>${unread}</div>`;
  }).join('');
}

function mailOpenFolder(f) {
  mailState.folder = f; mailState.search = '';
  const se = document.getElementById('mail-search'); if (se) se.value = '';
  mailLoadFolder(f);
}

async function mailLoadFolder(folder) {
  mailState.view = 'list';
  try { mailState.messages = await api('/api/mail/messages?folder=' + encodeURIComponent(folder)); }
  catch (e) { mailState.messages = []; }
  mailRenderFolders();
  mailRenderList();
}

function mailRenderList() {
  const el = document.getElementById('mail-main');
  if (!el) return;
  const msgs = mailState.messages;
  if (!msgs.length) { el.innerHTML = `<div class="mail-empty mail-empty-lg">${t('mailNoMessages')}</div>`; return; }
  const showFrom = mailState.folder !== 'sent';
  el.innerHTML = `<div class="mail-list">${msgs.map(m => `
    <div class="mail-row ${m.seen ? '' : 'unread'}" onclick="mailOpen(${m.box_id})">
      <span class="mail-row-flag ${m.flagged ? 'on' : ''}" onclick="event.stopPropagation();mailToggleFlag(${m.box_id}, ${!m.flagged})" title="${t('mailFlag')}">${m.flagged ? '★' : '☆'}</span>
      <span class="mail-row-from">${esc(showFrom ? (m.from_name || '?') : (m.to_addrs || ''))}</span>
      <span class="mail-row-main"><b class="mail-row-subj">${esc(m.subject || t('mailNoSubject'))}</b> <span class="mail-row-snip">${esc(m.snippet || '')}</span></span>
      <span class="mail-row-date">${esc(fmtDate(m.created_at))}</span>
    </div>`).join('')}</div>`;
}

async function mailOpen(boxId) {
  let box;
  try { box = await api('/api/mail/messages/' + boxId); }
  catch (e) { alert(e.message); return; }
  mailState.openBoxId = boxId;
  try { mailState.thread = await api('/api/mail/thread/' + box.thread_id); }
  catch (e) { mailState.thread = [box]; }
  mailState.view = 'message';
  mailLoadCounts().then(mailRenderFolders);
  mailRenderMessage(box);
}

function mailRenderMessage(box) {
  const el = document.getElementById('mail-main');
  if (!el) return;
  const canWrite = currentUser && currentUser.role !== 'readonly';
  const msgs = mailState.thread.length ? mailState.thread : [box];
  const subject = msgs[0].subject || t('mailNoSubject');
  el.innerHTML = `
    <div class="mail-msg-head">
      <button class="btn btn-ghost btn-sm" onclick="mailOpenFolder('${mailState.folder}')">&larr; ${t('mailBack')}</button>
      <h2 class="mail-msg-subject">${esc(subject)}</h2>
      <div class="mail-msg-actions">
        ${canWrite ? `<button class="btn btn-ghost btn-sm" onclick="mailReply()">${t('mailReply')}</button>` : ''}
        <button class="btn btn-ghost btn-sm" onclick="mailMove(${box.box_id}, 'trash')">${t('mailMove')}</button>
        <button class="btn btn-ghost btn-sm mail-danger" onclick="mailDelete(${box.box_id})">${t('mailDelete')}</button>
      </div>
    </div>
    <div class="mail-thread">${msgs.map(mailRenderOne).join('')}</div>`;
}

function mailRenderOne(m) {
  const body = m.body_html ? m.body_html : ('<p>' + esc(m.body_text || '').replace(/\n/g, '<br>') + '</p>');
  const atts = (m.attachments || []).map(a => `<a class="mail-att" href="${escAttr(a.url)}" target="_blank" rel="noopener">📎 ${esc(a.filename)}</a>`).join('');
  return `<div class="mail-one">
    <div class="mail-one-head">
      <span class="mail-one-from"><b>${esc(m.from_name || '?')}</b> <span class="mail-one-to">${t('mailTo')}: ${esc(m.to_addrs || '')}</span></span>
      <span class="mail-one-date">${esc(fmtDate(m.created_at))}</span>
    </div>
    <div class="mail-one-body md-content">${body}</div>
    ${atts ? `<div class="mail-one-atts">${atts}</div>` : ''}
  </div>`;
}

async function mailToggleFlag(boxId, flagged) {
  try { await api('/api/mail/messages/' + boxId + '/flag', 'POST', { flagged }); } catch (e) { return; }
  const m = mailState.messages.find(x => x.box_id === boxId); if (m) m.flagged = flagged;
  mailRenderList();
}

async function mailMove(boxId, folder) {
  try { await api('/api/mail/messages/' + boxId + '/move', 'POST', { folder }); } catch (e) { alert(e.message); return; }
  await mailLoadCounts(); await mailLoadFolder(mailState.folder);
}

async function mailDelete(boxId) {
  if (!confirm(t('mailDeleteConfirm'))) return;
  try { await api('/api/mail/messages/' + boxId, 'DELETE'); } catch (e) { alert(e.message); return; }
  await mailLoadCounts(); await mailLoadFolder(mailState.folder);
}

async function mailDoSearch(q) {
  q = q.trim();
  mailState.search = q;
  if (!q) { mailLoadFolder(mailState.folder); return; }
  try { mailState.messages = await api('/api/mail/messages?q=' + encodeURIComponent(q)); }
  catch (e) { mailState.messages = []; }
  mailState.view = 'list';
  mailRenderFolders();
  mailRenderList();
}

// === Compose ===
function mailComposeDialog(prefill) {
  prefill = prefill || {};
  mailState.compose = { attachments: [] };
  const ov = document.createElement('div');
  ov.className = 'modal-overlay';
  ov.innerHTML = `<div class="modal mail-modal">
    <h3>${esc(prefill.subject ? t('mailReply') : t('mailCompose'))}</h3>
    <div class="mail-form">
      <label class="mail-fld">${t('mailTo')}<input id="mail-to" placeholder="${t('mailToPlaceholder')}" value="${escAttr(prefill.to || '')}"></label>
      <label class="mail-fld">${t('mailSubject')}<input id="mail-subject" maxlength="300" value="${escAttr(prefill.subject || '')}"></label>
      <textarea id="mail-body" class="mail-body" rows="10" placeholder="${t('mailBody')}">${esc(prefill.body || '')}</textarea>
      <div class="mail-att-list" id="mail-att-list"></div>
    </div>
    <div class="mail-modal-foot">
      <button class="btn btn-ghost btn-sm" onclick="mailAttachFile()">📎 ${t('mailAttach')}</button>
      <span style="flex:1"></span>
      <button class="btn btn-ghost btn-sm" onclick="this.closest('.modal-overlay').remove()">${t('mailCancel')}</button>
      <button class="btn btn-primary btn-sm" id="mail-send-btn" onclick="mailSend(${prefill.thread_id || 0})">${t('mailSend')}</button>
    </div>
  </div>`;
  document.body.appendChild(ov);
  ov.addEventListener('click', e => { if (e.target === ov) ov.remove(); });
}

function mailRenderAttList() {
  const el = document.getElementById('mail-att-list');
  if (!el) return;
  el.innerHTML = mailState.compose.attachments.map((a, i) =>
    `<span class="mail-att-chip">📎 ${esc(a.filename)} <a onclick="mailRemoveAtt(${i})">✕</a></span>`).join('');
}
function mailRemoveAtt(i) { mailState.compose.attachments.splice(i, 1); mailRenderAttList(); }

function mailAttachFile() {
  const inp = document.createElement('input');
  inp.type = 'file';
  inp.onchange = async () => {
    const file = inp.files[0];
    if (!file) return;
    if (typeof MAX_UPLOAD_SIZE !== 'undefined' && file.size > MAX_UPLOAD_SIZE) { alert(t('fileTooLarge')); return; }
    const reader = new FileReader();
    reader.onload = async () => {
      try {
        const res = await api('/api/files', 'POST', { data: reader.result.split(',')[1], filename: file.name, mime: file.type || 'application/octet-stream' });
        mailState.compose.attachments.push({ file_id: res.id, filename: file.name });
        mailRenderAttList();
      } catch (e) { alert(e.message); }
    };
    reader.readAsDataURL(file);
  };
  inp.click();
}

async function mailSend(threadId) {
  const to = document.getElementById('mail-to').value.trim();
  if (!to) { alert(t('mailRecipientsRequired')); return; }
  const btn = document.getElementById('mail-send-btn');
  btn.disabled = true;
  const body = {
    to,
    subject: document.getElementById('mail-subject').value.trim(),
    body_text: document.getElementById('mail-body').value,
    thread_id: threadId || 0,
    attachments: mailState.compose.attachments,
  };
  try {
    await api('/api/mail/send', 'POST', body);
    document.querySelectorAll('.modal-overlay').forEach(m => m.remove());
    await mailLoadCounts();
    await mailLoadFolder(mailState.folder);
  } catch (e) { btn.disabled = false; alert(e.message); }
}

function mailReply() {
  const msgs = mailState.thread;
  const last = msgs[msgs.length - 1];
  if (!last) return;
  const quote = '\n\n— ' + (last.from_name || '') + ':\n' + (last.body_text || '').split('\n').map(l => '> ' + l).join('\n');
  mailComposeDialog({
    to: last.from_name || '',
    subject: (last.subject || '').replace(/^Re:\s*/i, 'Re: ') || 'Re: ',
    body: quote,
    thread_id: last.thread_id,
  });
}

// === Live updates + badge ===
function mailConnectSSE() {
  if (mailState.es || typeof EventSource === 'undefined') return;
  try {
    const es = new EventSource('/api/mail/stream');
    mailState.es = es;
    let tmo = null;
    es.onmessage = () => {
      clearTimeout(tmo);
      tmo = setTimeout(() => { mailLoadCounts().then(() => { mailRenderFolders(); if (mailState.view === 'list') mailLoadFolder(mailState.folder); }); mailUpdateBadge(); }, 250);
    };
    es.onerror = () => { };
  } catch (e) { }
}

async function mailInitGlobal() {
  if (typeof isServiceEnabled === 'function' && !isServiceEnabled('mail')) return;
  mailUpdateBadge();
}
async function mailUpdateBadge() {
  try {
    const r = await api('/api/mail/unread');
    if (typeof setRailBadge === 'function') setRailBadge('mail', r.unread || 0);
  } catch (e) { }
}
