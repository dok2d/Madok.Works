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
  customFolders: [], // [{id,name}]
  es: null,
};

const MAIL_FOLDERS = ['inbox', 'sent', 'drafts', 'junk', 'trash'];

const MAIL_I18N = {
  en: {
    mailCompose: 'Compose', mailInbox: 'Inbox', mailSent: 'Sent', mailDrafts: 'Drafts', mailTrash: 'Trash',
    mailTo: 'To', mailCc: 'Cc', mailSubject: 'Subject', mailBody: 'Message', mailSend: 'Send',
    mailCancel: 'Cancel', mailReply: 'Reply', mailDelete: 'Delete', mailMove: 'To trash',
    mailSearchPh: 'Search mail…', mailNoMessages: 'No messages', mailNoSubject: '(no subject)',
    mailAttach: 'Attach', mailAttachments: 'Attachments', mailDeleteConfirm: 'Delete this message permanently?',
    mailFrom: 'From', mailReadonly: 'Read-only access', mailRecipientsRequired: 'Add at least one recipient',
    mailToPlaceholder: 'username, username…', mailFlag: 'Flag', mailUnflag: 'Unflag', mailBack: 'Back', mailJunk: 'Junk', mailNewFolder: 'New folder', mailDeleteFolder: 'Delete folder', mailMoveTo: 'Move to', mailSave: 'Save', mailEdit: 'Edit', mailRules: 'Rules', mailRuleNew: 'New rule', mailRuleName: 'Name', mailRuleEnabled: 'Enabled', mailRuleMatch: 'Match', mailRuleAll: 'all conditions', mailRuleAny: 'any condition', mailRuleIf: 'If', mailRuleThen: 'Then',
  },
  ru: {
    mailCompose: 'Написать', mailInbox: 'Входящие', mailSent: 'Отправленные', mailDrafts: 'Черновики', mailTrash: 'Корзина',
    mailTo: 'Кому', mailCc: 'Копия', mailSubject: 'Тема', mailBody: 'Сообщение', mailSend: 'Отправить',
    mailCancel: 'Отмена', mailReply: 'Ответить', mailDelete: 'Удалить', mailMove: 'В корзину',
    mailSearchPh: 'Поиск почты…', mailNoMessages: 'Нет писем', mailNoSubject: '(без темы)',
    mailAttach: 'Прикрепить', mailAttachments: 'Вложения', mailDeleteConfirm: 'Удалить письмо навсегда?',
    mailFrom: 'От', mailReadonly: 'Только чтение', mailRecipientsRequired: 'Укажите хотя бы одного получателя',
    mailToPlaceholder: 'логин, логин…', mailFlag: 'Пометить', mailUnflag: 'Снять метку', mailBack: 'Назад', mailJunk: 'Спам', mailNewFolder: 'Новая папка', mailDeleteFolder: 'Удалить папку', mailMoveTo: 'Переместить в', mailSave: 'Сохранить', mailEdit: 'Изменить', mailRules: 'Правила', mailRuleNew: 'Новое правило', mailRuleName: 'Название', mailRuleEnabled: 'Включено', mailRuleMatch: 'Совпадение', mailRuleAll: 'все условия', mailRuleAny: 'любое условие', mailRuleIf: 'Если', mailRuleThen: 'То',
  },
  es: {
    mailCompose: "Redactar", mailInbox: "Entrada", mailSent: "Enviados", mailDrafts: "Borradores", mailTrash: "Papelera",
    mailTo: "Para", mailCc: "Cc", mailSubject: "Asunto", mailBody: "Mensaje", mailSend: "Enviar",
    mailCancel: "Cancelar", mailReply: "Responder", mailDelete: "Eliminar", mailMove: "A la papelera",
    mailSearchPh: "Buscar correo…", mailNoMessages: "Sin mensajes", mailNoSubject: "(sin asunto)",
    mailAttach: "Adjuntar", mailAttachments: "Adjuntos", mailDeleteConfirm: "¿Eliminar este mensaje permanentemente?",
    mailFrom: "De", mailReadonly: "Acceso de solo lectura", mailRecipientsRequired: "Añade al menos un destinatario",
    mailToPlaceholder: "usuario, usuario…", mailFlag: "Destacar", mailUnflag: "Quitar marca", mailBack: "Atrás", mailJunk: "Spam", mailNewFolder: "Nueva carpeta", mailDeleteFolder: "Eliminar carpeta", mailMoveTo: "Mover a", mailSave: "Guardar", mailEdit: "Editar", mailRules: "Reglas", mailRuleNew: "Nueva regla", mailRuleName: "Nombre", mailRuleEnabled: "Activado", mailRuleMatch: "Coincidir", mailRuleAll: "todas las condiciones", mailRuleAny: "cualquier condición", mailRuleIf: "Si", mailRuleThen: "Entonces",
  },
  fr: {
    mailCompose: "Nouveau", mailInbox: "Boîte de réception", mailSent: "Envoyés", mailDrafts: "Brouillons", mailTrash: "Corbeille",
    mailTo: "À", mailCc: "Cc", mailSubject: "Objet", mailBody: "Message", mailSend: "Envoyer",
    mailCancel: "Annuler", mailReply: "Répondre", mailDelete: "Supprimer", mailMove: "Vers la corbeille",
    mailSearchPh: "Rechercher du courrier…", mailNoMessages: "Aucun message", mailNoSubject: "(sans objet)",
    mailAttach: "Joindre", mailAttachments: "Pièces jointes", mailDeleteConfirm: "Supprimer définitivement ce message ?",
    mailFrom: "De", mailReadonly: "Accès en lecture seule", mailRecipientsRequired: "Ajoutez au moins un destinataire",
    mailToPlaceholder: "utilisateur, utilisateur…", mailFlag: "Marquer", mailUnflag: "Retirer la marque", mailBack: "Retour", mailJunk: "Indésirables", mailNewFolder: "Nouveau dossier", mailDeleteFolder: "Supprimer le dossier", mailMoveTo: "Déplacer vers", mailSave: "Enregistrer", mailEdit: "Modifier", mailRules: "Règles", mailRuleNew: "Nouvelle règle", mailRuleName: "Nom", mailRuleEnabled: "Activée", mailRuleMatch: "Correspondance", mailRuleAll: "toutes les conditions", mailRuleAny: "une condition", mailRuleIf: "Si", mailRuleThen: "Alors",
  },
  de: {
    mailCompose: "Verfassen", mailInbox: "Posteingang", mailSent: "Gesendet", mailDrafts: "Entwürfe", mailTrash: "Papierkorb",
    mailTo: "An", mailCc: "Cc", mailSubject: "Betreff", mailBody: "Nachricht", mailSend: "Senden",
    mailCancel: "Abbrechen", mailReply: "Antworten", mailDelete: "Löschen", mailMove: "In den Papierkorb",
    mailSearchPh: "E-Mail suchen…", mailNoMessages: "Keine Nachrichten", mailNoSubject: "(kein Betreff)",
    mailAttach: "Anhängen", mailAttachments: "Anhänge", mailDeleteConfirm: "Diese Nachricht endgültig löschen?",
    mailFrom: "Von", mailReadonly: "Nur-Lese-Zugriff", mailRecipientsRequired: "Mindestens einen Empfänger hinzufügen",
    mailToPlaceholder: "Benutzer, Benutzer…", mailFlag: "Markieren", mailUnflag: "Markierung entfernen", mailBack: "Zurück", mailJunk: "Spam", mailNewFolder: "Neuer Ordner", mailDeleteFolder: "Ordner löschen", mailMoveTo: "Verschieben nach", mailSave: "Speichern", mailEdit: "Bearbeiten", mailRules: "Regeln", mailRuleNew: "Neue Regel", mailRuleName: "Name", mailRuleEnabled: "Aktiv", mailRuleMatch: "Treffer", mailRuleAll: "alle Bedingungen", mailRuleAny: "eine Bedingung", mailRuleIf: "Wenn", mailRuleThen: "Dann",
  },
  pt: {
    mailCompose: "Escrever", mailInbox: "Caixa de entrada", mailSent: "Enviados", mailDrafts: "Rascunhos", mailTrash: "Lixeira",
    mailTo: "Para", mailCc: "Cc", mailSubject: "Assunto", mailBody: "Mensagem", mailSend: "Enviar",
    mailCancel: "Cancelar", mailReply: "Responder", mailDelete: "Excluir", mailMove: "Para a lixeira",
    mailSearchPh: "Buscar e-mail…", mailNoMessages: "Sem mensagens", mailNoSubject: "(sem assunto)",
    mailAttach: "Anexar", mailAttachments: "Anexos", mailDeleteConfirm: "Excluir esta mensagem permanentemente?",
    mailFrom: "De", mailReadonly: "Acesso somente leitura", mailRecipientsRequired: "Adicione pelo menos um destinatário",
    mailToPlaceholder: "usuário, usuário…", mailFlag: "Marcar", mailUnflag: "Remover marca", mailBack: "Voltar", mailJunk: "Spam", mailNewFolder: "Nova pasta", mailDeleteFolder: "Excluir pasta", mailMoveTo: "Mover para", mailSave: "Salvar", mailEdit: "Editar", mailRules: "Regras", mailRuleNew: "Nova regra", mailRuleName: "Nome", mailRuleEnabled: "Ativada", mailRuleMatch: "Correspondência", mailRuleAll: "todas as condições", mailRuleAny: "qualquer condição", mailRuleIf: "Se", mailRuleThen: "Então",
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
  try { mailState.customFolders = await api('/api/mail/folders/custom'); }
  catch (e) { mailState.customFolders = []; }
}

function mailRenderLayout() {
  const canWrite = currentUser && currentUser.role !== 'readonly';
  document.getElementById('app').innerHTML = `
  <div class="mail-layout">
    <aside class="mail-side">
      ${canWrite ? `<button class="btn btn-primary mail-compose" onclick="mailComposeDialog()">${t('mailCompose')}</button>` : ''}
      <input class="mail-search" id="mail-search" placeholder="${t('mailSearchPh')}" autocomplete="off">
      <div class="mail-folders" id="mail-folders"></div>
      ${canWrite ? `<button class="btn btn-ghost btn-sm mail-rules-btn" onclick="mailRulesDialog()">&#9881; ${t('mailRules')}</button>` : ''}
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

function mailFolderRow(name, label, deletable, fid) {
  const c = mailState.counts[name] || {};
  const unread = c.unread > 0 ? `<span class="mail-fcount">${c.unread}</span>` : '';
  const active = mailState.folder === name && mailState.view === 'list' && !mailState.search;
  return `<div class="mail-frow ${active ? 'active' : ''}" onclick="mailOpenFolder('${escAttr(name)}')">
    <span class="mail-frow-name">${esc(label)}</span>
    ${deletable ? `<button class="mail-fdel" title="${t('mailDeleteFolder')}" onclick="event.stopPropagation();mailDeleteFolder(${fid})">&#10005;</button>` : ''}
    ${unread}</div>`;
}

function mailRenderFolders() {
  const el = document.getElementById('mail-folders');
  if (!el) return;
  let html = MAIL_FOLDERS.map(f => mailFolderRow(f, mailFolderLabel(f), false, 0)).join('');
  for (const cf of mailState.customFolders) html += mailFolderRow(cf.name, cf.name, true, cf.id);
  const canWrite = currentUser && currentUser.role !== 'readonly';
  if (canWrite) html += `<div class="mail-frow mail-newfolder" onclick="mailNewFolder()">+ ${t('mailNewFolder')}</div>`;
  el.innerHTML = html;
}

async function mailNewFolder() {
  const name = prompt(t('mailNewFolder'));
  if (!name || !name.trim()) return;
  try { await api('/api/mail/folders/custom', 'POST', { name: name.trim() }); await mailLoadCounts(); mailRenderFolders(); }
  catch (e) { alert(e.message); }
}

async function mailDeleteFolder(id) {
  if (!confirm(t('mailDeleteFolder') + '?')) return;
  try {
    await api('/api/mail/folders/custom/' + id, 'DELETE');
    if (!MAIL_FOLDERS.includes(mailState.folder)) mailState.folder = 'inbox';
    await mailLoadCounts();
    await mailLoadFolder(mailState.folder);
  } catch (e) { alert(e.message); }
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
        <select class="mail-move-sel" onchange="if(this.value){mailMove(${box.box_id}, this.value);this.value=''}">
          <option value="">${t('mailMoveTo')}…</option>
          ${mailMoveOptions(box.folder)}
        </select>
        <button class="btn btn-ghost btn-sm mail-danger" onclick="mailDelete(${box.box_id})">${t('mailDelete')}</button>
      </div>
    </div>
    <div class="mail-thread">${msgs.map(mailRenderOne).join('')}</div>`;
}

// mailMoveOptions builds the "move to" option list (system + custom, excluding
// the current folder and the read-only Sent folder).
function mailMoveOptions(current) {
  const opts = [];
  for (const f of ['inbox', 'junk', 'trash']) {
    if (f !== current) opts.push(`<option value="${f}">${esc(mailFolderLabel(f))}</option>`);
  }
  for (const cf of mailState.customFolders) {
    if (cf.name !== current) opts.push(`<option value="${escAttr(cf.name)}">${esc(cf.name)}</option>`);
  }
  return opts.join('');
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

// === Rules / filters ===
const MAIL_RULE_FIELDS = ['from', 'to', 'subject', 'body'];
const MAIL_RULE_OPS = ['contains', 'equals', 'starts_with', 'regex'];
const MAIL_RULE_ACTIONS = ['move_to_folder', 'flag', 'mark_read', 'delete'];

async function mailRulesDialog() {
  let rules = [];
  try { rules = await api('/api/mail/rules'); } catch (e) { /* ignore */ }
  const body = `
    <div class="mail-rule-list">${rules.map(r => `<div class="mail-rule-row">
      <span>${r.enabled ? '' : '⊘ '}${esc(r.name || '—')}</span>
      <span>
        <button class="btn btn-ghost btn-sm" onclick="mailRuleEditor(${r.id})">${t('mailEdit') || 'Edit'}</button>
        <button class="btn btn-ghost btn-sm mail-danger" onclick="mailRuleDelete(${r.id})">${t('mailDelete')}</button>
      </span></div>`).join('') || `<div class="mail-empty">—</div>`}</div>
    <button class="btn btn-primary btn-sm" style="margin-top:10px" onclick="mailRuleEditor(0)">+ ${t('mailRuleNew')}</button>`;
  mailModal(t('mailRules'), body, null);
  mailRulesCache = rules;
}

let mailRulesCache = [];

function mailModal(title, bodyHtml, onSave) {
  const ov = document.createElement('div');
  ov.className = 'modal-overlay';
  ov.innerHTML = `<div class="modal mail-modal">
    <h3>${esc(title)}</h3>
    <div class="mail-modal-inner">${bodyHtml}</div>
    <div class="mail-modal-foot">
      <button class="btn btn-ghost btn-sm" onclick="this.closest('.modal-overlay').remove()">${t('mailCancel')}</button>
      ${onSave ? `<button class="btn btn-primary btn-sm" id="mail-modal-save">${t('mailSave')}</button>` : ''}
    </div>
  </div>`;
  document.body.appendChild(ov);
  ov.addEventListener('click', e => { if (e.target === ov) ov.remove(); });
  if (onSave) ov.querySelector('#mail-modal-save').addEventListener('click', () => onSave(ov));
  return ov;
}

function mailRuleEditor(id) {
  document.querySelectorAll('.modal-overlay').forEach(m => m.remove());
  const r = mailRulesCache.find(x => x.id === id) || { name: '', enabled: true, match_type: 'all', conditions: [{ field: 'from', op: 'contains', value: '' }], actions: [{ type: 'move_to_folder', arg: '' }] };
  mailRuleDraft = JSON.parse(JSON.stringify(r));
  mailRuleDraft.id = id;
  mailRenderRuleEditor();
}

let mailRuleDraft = null;

function mailRenderRuleEditor() {
  const r = mailRuleDraft;
  const folderOpts = ['inbox', 'junk', 'trash'].concat(mailState.customFolders.map(f => f.name));
  const lbl = (k, v) => `<option value="${v}">${esc(mailRuleLabel(k, v))}</option>`;
  const body = `
    <label class="mail-fld">${t('mailRuleName')}<input id="mr-name" value="${escAttr(r.name)}"></label>
    <label class="mail-fld mail-fld-row"><input type="checkbox" id="mr-enabled" ${r.enabled ? 'checked' : ''}> ${t('mailRuleEnabled')}</label>
    <label class="mail-fld">${t('mailRuleMatch')}
      <select id="mr-match"><option value="all" ${r.match_type === 'all' ? 'selected' : ''}>${t('mailRuleAll')}</option><option value="any" ${r.match_type === 'any' ? 'selected' : ''}>${t('mailRuleAny')}</option></select>
    </label>
    <div class="mail-rule-h">${t('mailRuleIf')}</div>
    <div id="mr-conds">${r.conditions.map((c, i) => `<div class="mail-rule-cond">
      <select onchange="mailRuleDraft.conditions[${i}].field=this.value">${MAIL_RULE_FIELDS.map(f => `<option value="${f}" ${c.field === f ? 'selected' : ''}>${esc(mailRuleLabel('field', f))}</option>`).join('')}</select>
      <select onchange="mailRuleDraft.conditions[${i}].op=this.value">${MAIL_RULE_OPS.map(o => `<option value="${o}" ${c.op === o ? 'selected' : ''}>${esc(mailRuleLabel('op', o))}</option>`).join('')}</select>
      <input value="${escAttr(c.value)}" oninput="mailRuleDraft.conditions[${i}].value=this.value">
      <button class="btn-icon btn-sm mail-danger" onclick="mailRuleDraft.conditions.splice(${i},1);mailRenderRuleEditor()">&#10005;</button>
    </div>`).join('')}</div>
    <button class="btn btn-ghost btn-sm" onclick="mailRuleDraft.conditions.push({field:'from',op:'contains',value:''});mailRenderRuleEditor()">+ ${t('mailRuleIf')}</button>
    <div class="mail-rule-h">${t('mailRuleThen')}</div>
    <div id="mr-acts">${r.actions.map((a, i) => `<div class="mail-rule-act">
      <select onchange="mailRuleDraft.actions[${i}].type=this.value;mailRenderRuleEditor()">${MAIL_RULE_ACTIONS.map(t2 => `<option value="${t2}" ${a.type === t2 ? 'selected' : ''}>${esc(mailRuleLabel('action', t2))}</option>`).join('')}</select>
      ${a.type === 'move_to_folder' ? `<select onchange="mailRuleDraft.actions[${i}].arg=this.value">${folderOpts.map(f => `<option value="${escAttr(f)}" ${a.arg === f ? 'selected' : ''}>${esc(f)}</option>`).join('')}</select>` : ''}
      <button class="btn-icon btn-sm mail-danger" onclick="mailRuleDraft.actions.splice(${i},1);mailRenderRuleEditor()">&#10005;</button>
    </div>`).join('')}</div>
    <button class="btn btn-ghost btn-sm" onclick="mailRuleDraft.actions.push({type:'flag',arg:''});mailRenderRuleEditor()">+ ${t('mailRuleThen')}</button>`;
  document.querySelectorAll('.modal-overlay').forEach(m => m.remove());
  mailModal(t('mailRules'), body, async ov => {
    r.name = ov.querySelector('#mr-name').value.trim();
    r.enabled = ov.querySelector('#mr-enabled').checked;
    r.match_type = ov.querySelector('#mr-match').value;
    // ensure default args for move actions
    for (const a of r.actions) if (a.type === 'move_to_folder' && !a.arg) a.arg = folderOpts[0];
    try {
      if (r.id) await api('/api/mail/rules/' + r.id, 'PUT', r);
      else await api('/api/mail/rules', 'POST', r);
      ov.remove();
    } catch (e) { alert(e.message); }
  });
}

async function mailRuleDelete(id) {
  if (!confirm(t('mailDelete') + '?')) return;
  try { await api('/api/mail/rules/' + id, 'DELETE'); mailRulesDialog(); } catch (e) { alert(e.message); }
}

function mailRuleLabel(kind, v) {
  const en = { from: 'From', to: 'To', subject: 'Subject', body: 'Body', contains: 'contains', equals: 'equals', starts_with: 'starts with', regex: 'matches regex', move_to_folder: 'move to folder', flag: 'flag', mark_read: 'mark read', delete: 'delete' };
  const ru = { from: 'От', to: 'Кому', subject: 'Тема', body: 'Текст', contains: 'содержит', equals: 'равно', starts_with: 'начинается с', regex: 'регэксп', move_to_folder: 'в папку', flag: 'пометить', mark_read: 'прочитано', delete: 'удалить' };
  const m = (currentLang === 'ru') ? ru : en;
  return m[v] || v;
}
