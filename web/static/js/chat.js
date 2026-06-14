// Chat service frontend. Loaded before the main inline script; relies on shared
// helpers defined there (api, t, esc, escAttr, navigateTo, getTimezone, langLocale,
// currentUser, currentServicePath, state.users, setRailBadge).

let chatState = {
  channels: [],
  channelId: null,
  channel: null,
  messages: [],
  oldestId: null,
  hasMore: false,
  loadingOlder: false,
  es: null, // EventSource for live updates
  pending: [], // attachments staged for the next message
};

const CHAT_CHANNEL_KEY = 'kanban-chat-channel';

// === i18n ===
// Chat strings are merged into the shared LANGS object on first render. ru/en are
// translated; the other 8 languages fall back to en until the Phase D i18n sweep.
const CHAT_I18N = {
  en: {
    chatChannels: 'Channels', chatGroups: 'Groups', chatDirect: 'Direct messages',
    chatNewChannel: 'New channel', chatNewGroup: 'New group', chatStartDM: 'New direct message',
    chatSearchPh: 'Search messages…', chatNoChannels: 'No channels yet',
    chatPickChannel: 'Pick a channel to start chatting', chatMessagePh: 'Write a message…',
    chatSend: 'Send', chatEdit: 'Edit', chatDelete: 'Delete', chatEdited: 'edited',
    chatDeleted: 'message deleted', chatReadonly: 'Read-only access', chatMembers: 'Members',
    chatJoin: 'Join', chatLeave: 'Leave', chatChannelSettings: 'Channel settings',
    chatChannelName: 'Name', chatTopic: 'Topic', chatSave: 'Save', chatCancel: 'Cancel',
    chatAddMember: 'Add', chatLoadMore: 'Load earlier messages', chatNoResults: 'Nothing found',
    chatNameRequired: 'Name is required', chatConfirmLeave: 'Leave this channel?',
    chatDeleteChannel: 'Delete channel', chatDeleteChannelConfirm: 'Delete this channel and all its messages?',
    chatNoUsers: 'No other users', chatDeleteMsgConfirm: 'Delete this message?',
    chatEditPrompt: 'Edit message:', chatAttach: 'Attach file',
  },
  ru: {
    chatChannels: 'Каналы', chatGroups: 'Группы', chatDirect: 'Личные сообщения',
    chatNewChannel: 'Новый канал', chatNewGroup: 'Новая группа', chatStartDM: 'Новое личное сообщение',
    chatSearchPh: 'Поиск сообщений…', chatNoChannels: 'Пока нет каналов',
    chatPickChannel: 'Выберите канал, чтобы начать общение', chatMessagePh: 'Написать сообщение…',
    chatSend: 'Отправить', chatEdit: 'Изменить', chatDelete: 'Удалить', chatEdited: 'изменено',
    chatDeleted: 'сообщение удалено', chatReadonly: 'Только чтение', chatMembers: 'Участники',
    chatJoin: 'Вступить', chatLeave: 'Покинуть', chatChannelSettings: 'Настройки канала',
    chatChannelName: 'Название', chatTopic: 'Тема', chatSave: 'Сохранить', chatCancel: 'Отмена',
    chatAddMember: 'Добавить', chatLoadMore: 'Загрузить ранние сообщения', chatNoResults: 'Ничего не найдено',
    chatNameRequired: 'Введите название', chatConfirmLeave: 'Покинуть канал?',
    chatDeleteChannel: 'Удалить канал', chatDeleteChannelConfirm: 'Удалить канал и все его сообщения?',
    chatNoUsers: 'Нет других пользователей', chatDeleteMsgConfirm: 'Удалить сообщение?',
    chatEditPrompt: 'Изменить сообщение:', chatAttach: 'Прикрепить файл',
  },
  es: {
    chatChannels: "Canales", chatGroups: "Grupos", chatDirect: "Mensajes directos",
    chatNewChannel: "Nuevo canal", chatNewGroup: "Nuevo grupo", chatStartDM: "Nuevo mensaje directo",
    chatSearchPh: "Buscar mensajes…", chatNoChannels: "Aún no hay canales",
    chatPickChannel: "Elige un canal para empezar a chatear", chatMessagePh: "Escribe un mensaje…",
    chatSend: "Enviar", chatEdit: "Editar", chatDelete: "Eliminar", chatEdited: "editado",
    chatDeleted: "mensaje eliminado", chatReadonly: "Acceso de solo lectura", chatMembers: "Miembros",
    chatJoin: "Unirse", chatLeave: "Salir", chatChannelSettings: "Ajustes del canal",
    chatChannelName: "Nombre", chatTopic: "Tema", chatSave: "Guardar", chatCancel: "Cancelar",
    chatAddMember: "Añadir", chatLoadMore: "Cargar mensajes anteriores", chatNoResults: "Nada encontrado",
    chatNameRequired: "El nombre es obligatorio", chatConfirmLeave: "¿Salir de este canal?",
    chatDeleteChannel: "Eliminar canal", chatDeleteChannelConfirm: "¿Eliminar este canal y todos sus mensajes?",
    chatNoUsers: "No hay otros usuarios", chatDeleteMsgConfirm: "¿Eliminar este mensaje?",
    chatEditPrompt: "Editar mensaje:", chatAttach: "Adjuntar archivo",
  },
  fr: {
    chatChannels: "Canaux", chatGroups: "Groupes", chatDirect: "Messages directs",
    chatNewChannel: "Nouveau canal", chatNewGroup: "Nouveau groupe", chatStartDM: "Nouveau message direct",
    chatSearchPh: "Rechercher des messages…", chatNoChannels: "Aucun canal pour le moment",
    chatPickChannel: "Choisissez un canal pour discuter", chatMessagePh: "Écrire un message…",
    chatSend: "Envoyer", chatEdit: "Modifier", chatDelete: "Supprimer", chatEdited: "modifié",
    chatDeleted: "message supprimé", chatReadonly: "Accès en lecture seule", chatMembers: "Membres",
    chatJoin: "Rejoindre", chatLeave: "Quitter", chatChannelSettings: "Paramètres du canal",
    chatChannelName: "Nom", chatTopic: "Sujet", chatSave: "Enregistrer", chatCancel: "Annuler",
    chatAddMember: "Ajouter", chatLoadMore: "Charger les messages précédents", chatNoResults: "Aucun résultat",
    chatNameRequired: "Le nom est obligatoire", chatConfirmLeave: "Quitter ce canal ?",
    chatDeleteChannel: "Supprimer le canal", chatDeleteChannelConfirm: "Supprimer ce canal et tous ses messages ?",
    chatNoUsers: "Aucun autre utilisateur", chatDeleteMsgConfirm: "Supprimer ce message ?",
    chatEditPrompt: "Modifier le message :", chatAttach: "Joindre un fichier",
  },
  de: {
    chatChannels: "Kanäle", chatGroups: "Gruppen", chatDirect: "Direktnachrichten",
    chatNewChannel: "Neuer Kanal", chatNewGroup: "Neue Gruppe", chatStartDM: "Neue Direktnachricht",
    chatSearchPh: "Nachrichten suchen…", chatNoChannels: "Noch keine Kanäle",
    chatPickChannel: "Wähle einen Kanal zum Chatten", chatMessagePh: "Nachricht schreiben…",
    chatSend: "Senden", chatEdit: "Bearbeiten", chatDelete: "Löschen", chatEdited: "bearbeitet",
    chatDeleted: "Nachricht gelöscht", chatReadonly: "Nur-Lese-Zugriff", chatMembers: "Mitglieder",
    chatJoin: "Beitreten", chatLeave: "Verlassen", chatChannelSettings: "Kanaleinstellungen",
    chatChannelName: "Name", chatTopic: "Thema", chatSave: "Speichern", chatCancel: "Abbrechen",
    chatAddMember: "Hinzufügen", chatLoadMore: "Ältere Nachrichten laden", chatNoResults: "Nichts gefunden",
    chatNameRequired: "Name ist erforderlich", chatConfirmLeave: "Diesen Kanal verlassen?",
    chatDeleteChannel: "Kanal löschen", chatDeleteChannelConfirm: "Diesen Kanal und alle Nachrichten löschen?",
    chatNoUsers: "Keine anderen Benutzer", chatDeleteMsgConfirm: "Diese Nachricht löschen?",
    chatEditPrompt: "Nachricht bearbeiten:", chatAttach: "Datei anhängen",
  },
  pt: {
    chatChannels: "Canais", chatGroups: "Grupos", chatDirect: "Mensagens diretas",
    chatNewChannel: "Novo canal", chatNewGroup: "Novo grupo", chatStartDM: "Nova mensagem direta",
    chatSearchPh: "Buscar mensagens…", chatNoChannels: "Ainda não há canais",
    chatPickChannel: "Escolha um canal para conversar", chatMessagePh: "Escreva uma mensagem…",
    chatSend: "Enviar", chatEdit: "Editar", chatDelete: "Excluir", chatEdited: "editado",
    chatDeleted: "mensagem excluída", chatReadonly: "Acesso somente leitura", chatMembers: "Membros",
    chatJoin: "Entrar", chatLeave: "Sair", chatChannelSettings: "Configurações do canal",
    chatChannelName: "Nome", chatTopic: "Tópico", chatSave: "Salvar", chatCancel: "Cancelar",
    chatAddMember: "Adicionar", chatLoadMore: "Carregar mensagens anteriores", chatNoResults: "Nada encontrado",
    chatNameRequired: "O nome é obrigatório", chatConfirmLeave: "Sair deste canal?",
    chatDeleteChannel: "Excluir canal", chatDeleteChannelConfirm: "Excluir este canal e todas as mensagens?",
    chatNoUsers: "Nenhum outro usuário", chatDeleteMsgConfirm: "Excluir esta mensagem?",
    chatEditPrompt: "Editar mensagem:", chatAttach: "Anexar arquivo",
  },
};

let chatI18nMerged = false;
function ensureChatI18n() {
  if (chatI18nMerged || typeof LANGS === 'undefined') return;
  for (const lang of ['ru', 'en', 'zh', 'es', 'fr', 'de', 'pt', 'ja', 'ko', 'ar']) {
    if (!LANGS[lang]) continue;
    const dict = CHAT_I18N[lang] || CHAT_I18N.en;
    for (const k in dict) if (!(k in LANGS[lang])) LANGS[lang][k] = dict[k];
  }
  chatI18nMerged = true;
}

// === Entry point (called by the SPA router) ===
async function renderChatView() {
  ensureChatI18n();
  try {
    chatState.channels = await api('/api/chat/channels');
  } catch (e) {
    document.getElementById('app').innerHTML = `<div class="chat-empty chat-empty-lg">${esc(e.message)}</div>`;
    return;
  }
  const m = currentServicePath.match(/^\/channel\/(\d+)/);
  let id = m ? Number(m[1]) : Number(localStorage.getItem(CHAT_CHANNEL_KEY) || 0);
  if (!chatState.channels.some(c => c.id === id)) id = 0;
  chatState.channelId = id || null;
  chatState.channel = chatState.channels.find(c => c.id === id) || null;
  chatRenderLayout();
  chatConnectSSE();
  if (chatState.channelId) {
    localStorage.setItem(CHAT_CHANNEL_KEY, chatState.channelId);
    await chatOpenChannel(chatState.channelId);
  }
}

// === Layout ===
function chatRenderLayout() {
  const canWrite = currentUser && currentUser.role !== 'readonly';
  document.getElementById('app').innerHTML = `
  <div class="chat-layout">
    <aside class="chat-side">
      <div class="chat-side-head">
        <input class="chat-search" id="chat-search" placeholder="${t('chatSearchPh')}" autocomplete="off">
        ${canWrite ? `<button class="btn-icon" title="${t('chatNewChannel')}" onclick="chatNewChannelDialog('channel')">+</button>` : ''}
      </div>
      <div class="chat-search-results hidden" id="chat-search-results"></div>
      <div class="chat-channels" id="chat-channels"></div>
    </aside>
    <main class="chat-main" id="chat-main"></main>
  </div>`;
  chatRenderChannelList();
  chatRenderMain();
  const se = document.getElementById('chat-search');
  if (se) {
    let tmo = null;
    se.addEventListener('input', function () { clearTimeout(tmo); tmo = setTimeout(() => chatDoSearch(this.value), 300); });
    se.addEventListener('keydown', function (e) { if (e.key === 'Escape') { this.value = ''; chatDoSearch(''); } });
  }
}

function chatChannelLabel(c) {
  if (c.type === 'dm') return c.peer_name || t('chatDirect');
  return c.name;
}
function chatChannelIcon(c) {
  return c.type === 'dm' ? '@' : (c.type === 'group' ? '&#128274;' : '#');
}
function chatCanManage(c) {
  return c && currentUser && (currentUser.role === 'admin' || c.created_by === currentUser.id);
}

function chatGrpAddBtn(type) {
  if (!currentUser || currentUser.role === 'readonly') return '';
  if (type === 'dm') return `<button class="chat-grp-add" title="${t('chatStartDM')}" onclick="chatStartDMDialog()">+</button>`;
  if (type === 'group') return `<button class="chat-grp-add" title="${t('chatNewGroup')}" onclick="chatNewChannelDialog('group')">+</button>`;
  return `<button class="chat-grp-add" title="${t('chatNewChannel')}" onclick="chatNewChannelDialog('channel')">+</button>`;
}

function chatRenderChannelList() {
  const el = document.getElementById('chat-channels');
  if (!el) return;
  const groups = [
    { key: 'channel', label: t('chatChannels') },
    { key: 'group', label: t('chatGroups') },
    { key: 'dm', label: t('chatDirect') },
  ];
  let html = '';
  for (const g of groups) {
    const items = chatState.channels.filter(c => c.type === g.key);
    if (!items.length && g.key === 'group') continue;
    html += `<div class="chat-grp">
      <div class="chat-grp-head"><span>${esc(g.label)}</span>${chatGrpAddBtn(g.key)}</div>`;
    for (const c of items) {
      html += `<div class="chat-ch-row ${c.id === chatState.channelId ? 'active' : ''}" onclick="chatOpen(${c.id})">
        <span class="chat-ch-name">${chatChannelIcon(c)} ${esc(chatChannelLabel(c))}</span>
        ${c.unread > 0 ? `<span class="chat-ch-badge">${c.unread > 99 ? '99+' : c.unread}</span>` : ''}
      </div>`;
    }
    html += `</div>`;
  }
  el.innerHTML = html || `<div class="chat-empty">${t('chatNoChannels')}</div>`;
}

function chatRenderMain() {
  const el = document.getElementById('chat-main');
  if (!el) return;
  const c = chatState.channel;
  if (!c) {
    el.innerHTML = `<div class="chat-empty chat-empty-lg">
      <div style="font-size:2rem;margin-bottom:10px">&#128172;</div>
      <p>${t('chatPickChannel')}</p></div>`;
    return;
  }
  el.innerHTML = `
    <div class="chat-head">
      <div class="chat-head-info">
        <h2 class="chat-head-title">${chatChannelIcon(c)} ${esc(chatChannelLabel(c))}</h2>
        ${c.topic ? `<div class="chat-head-topic">${esc(c.topic)}</div>` : ''}
      </div>
      <div class="chat-head-actions">${chatHeadActions(c)}</div>
    </div>
    <div class="chat-msgs" id="chat-msgs"></div>
    ${chatComposer()}`;
  chatRenderMessages();
  chatScrollBottom();
  const inp = document.getElementById('chat-input');
  if (inp) {
    inp.addEventListener('keydown', e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); chatSend(); } });
    inp.addEventListener('input', chatAutoGrow);
  }
}

function chatHeadActions(c) {
  if (c.type === 'dm') return '';
  let h = `<button class="btn btn-ghost btn-sm" onclick="chatMembersDialog()">${t('chatMembers')} (${c.member_count || 0})</button>`;
  if (currentUser && currentUser.role !== 'readonly') {
    if (c.type === 'channel' && !c.is_member) h += `<button class="btn btn-ghost btn-sm" onclick="chatJoin()">${t('chatJoin')}</button>`;
    else if (c.is_member) h += `<button class="btn btn-ghost btn-sm" onclick="chatLeave()">${t('chatLeave')}</button>`;
  }
  if (chatCanManage(c)) h += `<button class="btn btn-ghost btn-sm" title="${t('chatChannelSettings')}" onclick="chatChannelSettings()">&#9881;</button>`;
  return h;
}

function chatComposer() {
  if (currentUser && currentUser.role === 'readonly') return `<div class="chat-readonly">${t('chatReadonly')}</div>`;
  return `<div class="chat-pending" id="chat-pending"></div>
  <div class="chat-composer">
    <button class="btn btn-ghost btn-sm chat-attach-btn" title="${t('chatAttach')}" onclick="chatAttachFile()">&#128206;</button>
    <textarea id="chat-input" class="chat-input" rows="1" placeholder="${t('chatMessagePh')}"></textarea>
    <button class="btn btn-primary btn-sm" onclick="chatSend()">${t('chatSend')}</button>
  </div>`;
}

// chatAttachFile uploads a file to /api/files and stages it for the next message.
function chatAttachFile() {
  const inp = document.createElement('input');
  inp.type = 'file';
  inp.onchange = async () => {
    const file = inp.files[0];
    if (!file) return;
    if (typeof MAX_UPLOAD_SIZE !== 'undefined' && file.size > MAX_UPLOAD_SIZE) { alert(t('fileTooLarge')); return; }
    try {
      const upload = (typeof compressImage === 'function' && file.type.startsWith('image/') && file.type !== 'image/gif') ? await compressImage(file) : file;
      const reader = new FileReader();
      reader.onload = async () => {
        try {
          const res = await api('/api/files', 'POST', { data: reader.result.split(',')[1], filename: file.name, mime: upload.type || file.type || 'application/octet-stream' });
          chatState.pending.push({ file_id: res.id, filename: file.name });
          chatRenderPending();
        } catch (e) { alert(e.message); }
      };
      reader.readAsDataURL(upload);
    } catch (e) { alert(e.message); }
  };
  inp.click();
}

function chatRenderPending() {
  const el = document.getElementById('chat-pending');
  if (!el) return;
  el.innerHTML = chatState.pending.map((a, i) =>
    `<span class="chat-pending-chip">&#128206; ${esc(a.filename)} <a onclick="chatRemovePending(${i})">&#10005;</a></span>`).join('');
}
function chatRemovePending(i) { chatState.pending.splice(i, 1); chatRenderPending(); }

// chatRenderAttachments renders a message's attachments (inline images + file chips).
function chatRenderAttachments(msg) {
  if (!msg.attachments || !msg.attachments.length) return '';
  return `<div class="chat-atts">` + msg.attachments.map(a => {
    if (/\.(png|jpe?g|gif|webp|bmp)$/i.test(a.filename)) {
      return `<a href="${escAttr(a.url)}" target="_blank" rel="noopener"><img class="chat-att-img" src="${escAttr(a.url)}" alt="${escAttr(a.filename)}"></a>`;
    }
    return `<a class="chat-att" href="${escAttr(a.url)}" target="_blank" rel="noopener" download>&#128206; ${esc(a.filename)}</a>`;
  }).join('') + `</div>`;
}

// === Messages ===
function chatRenderMessages() {
  const box = document.getElementById('chat-msgs');
  if (!box) return;
  let html = chatState.hasMore ? `<div class="chat-loadmore" onclick="chatLoadOlder()">${t('chatLoadMore')}</div>` : '';
  let lastAuthor = null, lastDay = null;
  for (const msg of chatState.messages) {
    const day = (msg.created_at || '').slice(0, 10);
    if (day !== lastDay) { html += `<div class="chat-day">${esc(chatFmtDay(msg.created_at))}</div>`; lastDay = day; lastAuthor = null; }
    const grouped = !msg.deleted && msg.author_id === lastAuthor;
    html += chatRenderMsg(msg, grouped);
    lastAuthor = msg.deleted ? null : msg.author_id;
  }
  box.innerHTML = html;
}

function chatRenderMsg(msg, grouped) {
  if (msg.deleted) {
    return `<div class="chat-msg deleted"><div class="chat-msg-body"><i class="chat-del">${t('chatDeleted')}</i></div></div>`;
  }
  const mine = currentUser && msg.author_id === currentUser.id;
  const canManage = mine || (currentUser && currentUser.role === 'admin');
  return `<div class="chat-msg ${grouped ? 'grouped' : ''}">
    ${grouped ? '' : `<div class="chat-msg-head"><b class="chat-author">${esc(msg.author_name || '?')}</b> <span class="chat-time">${esc(chatFmtTime(msg.created_at))}</span></div>`}
    <div class="chat-msg-body">
      <span class="chat-text">${chatFormatText(msg.text)}</span>
      ${msg.edited_at ? `<span class="chat-edited">(${t('chatEdited')})</span>` : ''}
      ${chatRenderAttachments(msg)}
      ${canManage ? `<span class="chat-msg-actions">
        ${mine ? `<a onclick="chatEditMsg(${msg.id})">${t('chatEdit')}</a>` : ''}
        <a class="chat-danger" onclick="chatDeleteMsg(${msg.id})">${t('chatDelete')}</a>
      </span>` : ''}
    </div>
  </div>`;
}

// chatFormatText escapes the text, then highlights @mentions of known users and
// turns newlines into <br>. Runs on already-escaped text so it cannot inject HTML.
function chatFormatText(text) {
  let h = esc(text).replace(/\n/g, '<br>');
  h = h.replace(/@([\p{L}\p{N}_.\-]+)/gu, (full, name) => {
    const u = (state.users || []).find(x => x.username === name);
    return u ? `<span class="chat-mention">@${esc(name)}</span>` : full;
  });
  return h;
}

async function chatOpenChannel(id) {
  chatState.channelId = id;
  chatState.channel = chatState.channels.find(c => c.id === id) || chatState.channel;
  try {
    chatState.messages = await api(`/api/chat/channels/${id}/messages?limit=50`);
  } catch (e) { chatState.messages = []; }
  chatState.hasMore = chatState.messages.length >= 50;
  chatState.oldestId = chatState.messages.length ? chatState.messages[0].id : null;
  chatRenderMain();
  chatMarkRead();
}

function chatOpen(id) { navigateTo('/chat/channel/' + id); }

async function chatReloadChannel() {
  try { chatState.channels = await api('/api/chat/channels'); } catch (e) { /* ignore */ }
  chatState.channel = chatState.channels.find(c => c.id === chatState.channelId) || null;
  chatRenderChannelList();
  chatRenderMain();
}

async function chatSend() {
  const inp = document.getElementById('chat-input');
  if (!inp) return;
  const text = inp.value.trim();
  const atts = chatState.pending.slice();
  if (!text && !atts.length) return;
  inp.value = '';
  chatAutoGrow.call(inp);
  chatState.pending = [];
  chatRenderPending();
  try {
    const msg = await api(`/api/chat/channels/${chatState.channelId}/messages`, 'POST', { text, attachments: atts });
    chatAppendMsg(msg);
    chatMarkRead();
  } catch (e) { inp.value = text; chatState.pending = atts; chatRenderPending(); alert(e.message); }
}

function chatAppendMsg(msg) {
  if (msg.channel_id !== chatState.channelId) return;
  if (chatState.messages.some(m => m.id === msg.id)) return;
  chatState.messages.push(msg);
  chatRenderMessages();
  chatScrollBottom();
}

async function chatEditMsg(id) {
  const msg = chatState.messages.find(m => m.id === id);
  if (!msg) return;
  const text = prompt(t('chatEditPrompt'), msg.text);
  if (text === null) return;
  const v = text.trim();
  if (!v) return;
  try {
    const upd = await api('/api/chat/messages/' + id, 'PUT', { text: v });
    chatReplaceMsg(upd);
  } catch (e) { alert(e.message); }
}

async function chatDeleteMsg(id) {
  if (!confirm(t('chatDeleteMsgConfirm'))) return;
  try {
    await api('/api/chat/messages/' + id, 'DELETE');
    const i = chatState.messages.findIndex(m => m.id === id);
    if (i >= 0) { chatState.messages[i].deleted = true; chatState.messages[i].text = ''; chatRenderMessages(); }
  } catch (e) { alert(e.message); }
}

function chatReplaceMsg(msg) {
  const i = chatState.messages.findIndex(m => m.id === msg.id);
  if (i >= 0) { chatState.messages[i] = msg; chatRenderMessages(); }
}

async function chatLoadOlder() {
  if (chatState.loadingOlder || !chatState.oldestId) return;
  chatState.loadingOlder = true;
  try {
    const older = await api(`/api/chat/channels/${chatState.channelId}/messages?before=${chatState.oldestId}&limit=50`);
    const box = document.getElementById('chat-msgs');
    const prevH = box ? box.scrollHeight : 0;
    if (older.length) {
      chatState.messages = older.concat(chatState.messages);
      chatState.oldestId = chatState.messages[0].id;
      chatState.hasMore = older.length >= 50;
    } else {
      chatState.hasMore = false;
    }
    chatRenderMessages();
    if (box) box.scrollTop = box.scrollHeight - prevH;
  } catch (e) { /* ignore */ }
  chatState.loadingOlder = false;
}

async function chatMarkRead() {
  if (!chatState.channelId || !chatState.messages.length) return;
  const lastId = chatState.messages[chatState.messages.length - 1].id;
  try { await api(`/api/chat/channels/${chatState.channelId}/read`, 'POST', { last_id: lastId }); } catch (e) { /* ignore */ }
  if (chatState.channel) chatState.channel.unread = 0;
  chatRenderChannelList();
  chatUpdateRailBadge();
}

function chatScrollBottom() { const b = document.getElementById('chat-msgs'); if (b) b.scrollTop = b.scrollHeight; }
function chatAutoGrow() { this.style.height = 'auto'; this.style.height = Math.min(this.scrollHeight, 140) + 'px'; }

function chatUpdateRailBadge() {
  if (typeof setRailBadge !== 'function') return;
  const total = chatState.channels.reduce((s, c) => s + (c.unread || 0), 0);
  setRailBadge('chat', total);
}

// === Date helpers (SQLite stores UTC; render in the user's timezone) ===
function chatTimeFmt(s, opts) {
  if (!s) return '';
  const iso = s.includes('T') ? s : s.replace(' ', 'T') + 'Z';
  const d = new Date(iso);
  const loc = (typeof langLocale === 'function') ? langLocale() : undefined;
  try { return new Intl.DateTimeFormat(loc, Object.assign({ timeZone: getTimezone() }, opts)).format(d); }
  catch (e) { return new Intl.DateTimeFormat(loc, opts).format(d); }
}
function chatFmtTime(s) { return chatTimeFmt(s, { hour: '2-digit', minute: '2-digit', hour12: false }); }
function chatFmtDay(s) { return chatTimeFmt(s, { year: 'numeric', month: 'short', day: 'numeric' }); }

// chatInitGlobal is called once at app startup so the unread badge updates and
// live events flow even before the user first opens the chat view.
async function chatInitGlobal() {
  if (typeof isServiceEnabled === 'function' && !isServiceEnabled('chat')) return;
  chatConnectSSE();
  try {
    const r = await api('/api/chat/unread');
    if (typeof setRailBadge === 'function') setRailBadge('chat', r.unread || 0);
  } catch (e) { /* ignore */ }
}

function chatDisconnect() {
  if (chatState.es) { try { chatState.es.close(); } catch (e) { /* ignore */ } chatState.es = null; }
}

// === Live updates (SSE) ===
function chatConnectSSE() {
  if (chatState.es || typeof EventSource === 'undefined') return;
  try {
    const es = new EventSource('/api/chat/events');
    chatState.es = es;
    es.onmessage = ev => { try { chatHandleEvent(JSON.parse(ev.data)); } catch (e) { /* ignore */ } };
    es.onerror = () => { /* the browser reconnects automatically */ };
  } catch (e) { /* SSE unavailable */ }
}

function chatHandleEvent(ev) {
  if (!ev || !ev.type) return;
  switch (ev.type) {
    case 'chat_message':
      if (ev.message && ev.channel_id === chatState.channelId) {
        chatAppendMsg(ev.message);
        if (document.hasFocus()) chatMarkRead();
      }
      chatRefreshChannelsSoon();
      break;
    case 'chat_edit':
    case 'chat_delete':
      if (ev.message && ev.channel_id === chatState.channelId) chatReplaceMsg(ev.message);
      break;
    case 'chat_channels':
      chatRefreshChannelsSoon();
      break;
  }
}

let chatChanTmo = null;
function chatRefreshChannelsSoon() {
  clearTimeout(chatChanTmo);
  chatChanTmo = setTimeout(async () => {
    try {
      chatState.channels = await api('/api/chat/channels');
      chatState.channel = chatState.channels.find(c => c.id === chatState.channelId) || chatState.channel;
      chatRenderChannelList();
      chatUpdateRailBadge();
    } catch (e) { /* ignore */ }
  }, 300);
}

// === Search ===
async function chatDoSearch(q) {
  const box = document.getElementById('chat-search-results');
  if (!box) return;
  q = q.trim();
  if (!q) { box.classList.add('hidden'); box.innerHTML = ''; return; }
  try {
    const hits = await api('/api/chat/search?q=' + encodeURIComponent(q));
    box.classList.remove('hidden');
    box.innerHTML = hits.length ? hits.map(h => `<div class="chat-hit" onclick="chatOpen(${h.channel_id})">
      <div class="chat-hit-title">${esc(h.channel_name)} &middot; ${esc(h.author_name || '?')}</div>
      <div class="chat-hit-snippet">${esc(h.text)}</div>
    </div>`).join('') : `<div class="chat-empty">${t('chatNoResults')}</div>`;
  } catch (e) { box.classList.add('hidden'); }
}

// === Dialogs ===
function chatModal(title, bodyHtml, onSave) {
  const ov = document.createElement('div');
  ov.className = 'modal-overlay';
  ov.innerHTML = `<div class="modal chat-modal">
    <h3>${esc(title)}</h3>
    <div class="chat-modal-body">${bodyHtml}</div>
    <div class="chat-modal-foot">
      <button class="btn btn-ghost btn-sm" onclick="this.closest('.modal-overlay').remove()">${t('chatCancel')}</button>
      ${onSave ? `<button class="btn btn-primary btn-sm" id="chat-modal-save">${t('chatSave')}</button>` : ''}
    </div>
  </div>`;
  document.body.appendChild(ov);
  ov.addEventListener('click', e => { if (e.target === ov) ov.remove(); });
  if (onSave) ov.querySelector('#chat-modal-save').addEventListener('click', () => onSave(ov));
  return ov;
}

function chatNewChannelDialog(type) {
  type = type || 'channel';
  const body = `
    <label class="chat-fld">${t('chatChannelName')}<input id="chat-new-name" maxlength="100"></label>
    <label class="chat-fld">${t('chatTopic')}<input id="chat-new-topic" maxlength="300"></label>`;
  chatModal(type === 'group' ? t('chatNewGroup') : t('chatNewChannel'), body, async ov => {
    const name = ov.querySelector('#chat-new-name').value.trim();
    if (!name) { alert(t('chatNameRequired')); return; }
    const topic = ov.querySelector('#chat-new-topic').value.trim();
    try {
      const res = await api('/api/chat/channels', 'POST', { name, topic, type, members: [] });
      ov.remove();
      chatState.channels = await api('/api/chat/channels');
      navigateTo('/chat/channel/' + res.id);
    } catch (e) { alert(e.message); }
  });
}

function chatStartDMDialog() {
  const users = (state.users || []).filter(u => u.id !== (currentUser && currentUser.id));
  if (!users.length) { alert(t('chatNoUsers')); return; }
  const body = `<select id="chat-dm-user" class="chat-modal-select">${users.map(u => `<option value="${u.id}">${esc(u.username)}</option>`).join('')}</select>`;
  chatModal(t('chatStartDM'), body, async ov => {
    const uid = Number(ov.querySelector('#chat-dm-user').value);
    try {
      const res = await api('/api/chat/dm', 'POST', { user_id: uid });
      ov.remove();
      chatState.channels = await api('/api/chat/channels');
      navigateTo('/chat/channel/' + res.id);
    } catch (e) { alert(e.message); }
  });
}

async function chatMembersDialog() {
  const c = chatState.channel;
  if (!c) return;
  let members = [];
  try { members = await api(`/api/chat/channels/${c.id}/members`); } catch (e) { /* ignore */ }
  const canManage = chatCanManage(c) && c.type === 'group';
  const others = (state.users || []).filter(u => !members.some(m => m.user_id === u.id));
  const body = `
    <div class="chat-mem-list">${members.map(m => `<div class="chat-mem-row">
      <span>${esc(m.username)}</span>
      ${canManage && m.user_id !== c.created_by ? `<button class="btn-icon btn-sm chat-danger" onclick="chatRemoveMember(${m.user_id})">&#10005;</button>` : ''}
    </div>`).join('') || `<div class="chat-empty">—</div>`}</div>
    ${canManage && others.length ? `<div class="chat-mem-add">
      <select id="chat-add-member">${others.map(u => `<option value="${u.id}">${esc(u.username)}</option>`).join('')}</select>
      <button class="btn btn-primary btn-sm" onclick="chatAddMember()">${t('chatAddMember')}</button>
    </div>` : ''}`;
  chatModal(t('chatMembers'), body, null);
}

async function chatAddMember() {
  const sel = document.getElementById('chat-add-member');
  if (!sel) return;
  const uid = Number(sel.value);
  try {
    await api(`/api/chat/channels/${chatState.channelId}/members`, 'POST', { user_id: uid });
    document.querySelectorAll('.modal-overlay').forEach(m => m.remove());
    await chatReloadChannel();
    chatMembersDialog();
  } catch (e) { alert(e.message); }
}

async function chatRemoveMember(uid) {
  try {
    await api(`/api/chat/channels/${chatState.channelId}/members?user_id=${uid}`, 'DELETE');
    document.querySelectorAll('.modal-overlay').forEach(m => m.remove());
    await chatReloadChannel();
    chatMembersDialog();
  } catch (e) { alert(e.message); }
}

async function chatJoin() {
  try {
    await api(`/api/chat/channels/${chatState.channelId}/members`, 'POST', { user_id: currentUser.id });
    await chatReloadChannel();
  } catch (e) { alert(e.message); }
}

async function chatLeave() {
  if (!confirm(t('chatConfirmLeave'))) return;
  try {
    await api(`/api/chat/channels/${chatState.channelId}/members?user_id=${currentUser.id}`, 'DELETE');
    localStorage.removeItem(CHAT_CHANNEL_KEY);
    chatState.channelId = null;
    chatState.channel = null;
    navigateTo('/chat');
  } catch (e) { alert(e.message); }
}

function chatChannelSettings() {
  const c = chatState.channel;
  if (!c) return;
  const body = `
    <label class="chat-fld">${t('chatChannelName')}<input id="chat-set-name" maxlength="100" value="${escAttr(c.name)}"></label>
    <label class="chat-fld">${t('chatTopic')}<input id="chat-set-topic" maxlength="300" value="${escAttr(c.topic)}"></label>
    <button class="btn btn-ghost btn-sm chat-danger chat-set-del" onclick="chatDeleteChannel()">${t('chatDeleteChannel')}</button>`;
  chatModal(t('chatChannelSettings'), body, async ov => {
    const name = ov.querySelector('#chat-set-name').value.trim();
    const topic = ov.querySelector('#chat-set-topic').value.trim();
    if (!name) { alert(t('chatNameRequired')); return; }
    try {
      await api(`/api/chat/channels/${c.id}`, 'PUT', { name, topic });
      ov.remove();
      await chatReloadChannel();
    } catch (e) { alert(e.message); }
  });
}

async function chatDeleteChannel() {
  const c = chatState.channel;
  if (!c) return;
  if (!confirm(t('chatDeleteChannelConfirm'))) return;
  try {
    await api(`/api/chat/channels/${c.id}`, 'DELETE');
    document.querySelectorAll('.modal-overlay').forEach(m => m.remove());
    localStorage.removeItem(CHAT_CHANNEL_KEY);
    chatState.channelId = null;
    chatState.channel = null;
    navigateTo('/chat');
  } catch (e) { alert(e.message); }
}
