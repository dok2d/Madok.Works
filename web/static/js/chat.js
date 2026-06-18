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
  replyTo: null, // message being replied to {id, author_name, text}
  pinned: [], // pinned messages for the current channel
  pinnedIdx: 0, // which pinned message the bar currently shows
};

const CHAT_CHANNEL_KEY = 'kanban-chat-channel';

// Quick-reaction emoji set (Telegram-style), positive and negative.
const CHAT_EMOJI = ['👍', '❤️', '😂', '🎉', '😮', '🔥', '👏', '🙏', '👎', '😢', '😡', '💩'];

// Categorized emoji set for the composer picker (a curated common subset).
const CHAT_EMOJI_GRID = {
  '😀': ['😀', '😁', '😂', '🤣', '😅', '😊', '😍', '😘', '😎', '🤩', '🤔', '😐', '😴', '😇', '🙃', '😉', '😋', '😜', '🤪', '😭', '😢', '😡', '😱', '😬', '🙄', '😏', '🥳', '🥺', '🤗', '🤐'],
  '👍': ['👍', '👎', '👏', '🙏', '👌', '🤝', '💪', '🙌', '👋', '✌️', '🤞', '👈', '👉', '👆', '👇', '✋', '🤚', '🖐️', '🫶', '🤙'],
  '❤️': ['❤️', '🧡', '💛', '💚', '💙', '💜', '🖤', '🤍', '💔', '❣️', '💕', '💞', '💓', '💗', '💖', '💘', '💝', '⭐', '🌟', '✨'],
  '🔥': ['🔥', '🎉', '🎊', '🎁', '🏆', '🥇', '✅', '❌', '⚠️', '❗', '❓', '💯', '🚀', '💡', '📌', '📎', '🔖', '🔔', '⏰', '📅'],
  '🐱': ['🐱', '🐶', '🦊', '🐻', '🐼', '🐨', '🦁', '🐸', '🐵', '🦄', '🐝', '🦋', '🌹', '🌸', '🌳', '🍀', '☀️', '🌙', '⚡', '🌈'],
  '🍕': ['🍕', '🍔', '🍟', '🌮', '🍣', '🍩', '🍰', '☕', '🍺', '🍷', '🥑', '🍎', '🍌', '🍇', '🍓', '🥕', '🧀', '🍿', '🍪', '🎂'],
};

// chatEmojiPicker shows the categorized grid; clicking an emoji inserts it into
// the message input at the caret.
function chatEmojiPicker(ev) {
  ev.stopPropagation();
  document.querySelectorAll('.chat-emoji-pop').forEach(p => p.remove());
  const cats = Object.keys(CHAT_EMOJI_GRID);
  const pop = document.createElement('div');
  pop.className = 'chat-emoji-pop';
  pop.innerHTML = `<div class="chat-emoji-tabs">${cats.map((c, i) => `<button data-cat="${i}" class="${i === 0 ? 'active' : ''}">${c}</button>`).join('')}</div>
    <div class="chat-emoji-grid" id="chat-emoji-grid"></div>`;
  document.body.appendChild(pop);
  const grid = pop.querySelector('#chat-emoji-grid');
  const fill = i => { grid.innerHTML = CHAT_EMOJI_GRID[cats[i]].map(e => `<button>${e}</button>`).join(''); };
  fill(0);
  pop.querySelectorAll('.chat-emoji-tabs button').forEach(b => b.addEventListener('click', e => {
    e.stopPropagation();
    pop.querySelectorAll('.chat-emoji-tabs button').forEach(x => x.classList.remove('active'));
    b.classList.add('active');
    fill(Number(b.dataset.cat));
  }));
  grid.addEventListener('click', e => {
    const b = e.target.closest('button');
    if (!b) return;
    chatInsertEmoji(b.textContent);
  });
  const rect = ev.target.getBoundingClientRect();
  pop.style.left = Math.max(8, rect.left) + 'px';
  pop.style.top = (rect.top - 232) + 'px';
  setTimeout(() => document.addEventListener('click', function close() { pop.remove(); document.removeEventListener('click', close); }), 0);
}

function chatInsertEmoji(emoji) {
  const inp = document.getElementById('chat-input');
  if (!inp) return;
  const s = inp.selectionStart || inp.value.length, e = inp.selectionEnd || inp.value.length;
  inp.value = inp.value.slice(0, s) + emoji + inp.value.slice(e);
  inp.focus();
  inp.selectionStart = inp.selectionEnd = s + emoji.length;
}

// === i18n ===
// Chat strings are merged into the shared LANGS object on first render. ru/en are
// translated; the other 8 languages fall back to en until the Phase D i18n sweep.
const CHAT_I18N = {
  en: {
    chatChannels: 'Channels', chatGroups: 'Groups', chatDirect: 'Direct messages',
    chatNewChannel: 'New channel', chatNewGroup: 'New group', chatStartDM: 'New direct message',
    chatSearchPh: 'Search messages…', chatNoChannels: 'No channels yet',
    chatPickChannel: 'Pick a channel to start chatting', chatMessagePh: 'Write a message…',
    chatTitle: 'Chat', chatNoGroups: 'No private groups yet',
    chatEmoji: 'Emoji', chatSend: 'Send', chatEdit: 'Edit', chatDelete: 'Delete', chatEdited: 'edited',
    chatDeleted: 'message deleted', chatReadonly: 'Read-only access', chatMembers: 'Members',
    chatJoin: 'Join', chatLeave: 'Leave', chatChannelSettings: 'Channel settings',
    chatChannelName: 'Name', chatTopic: 'Topic', chatSave: 'Save', chatCancel: 'Cancel',
    chatAddMember: 'Add', chatLoadMore: 'Load earlier messages', chatNoResults: 'Nothing found',
    chatNameRequired: 'Name is required', chatConfirmLeave: 'Leave this channel?',
    chatDeleteChannel: 'Delete channel', chatDeleteChannelConfirm: 'Delete this channel and all its messages?',
    chatNoUsers: 'No other users', chatDeleteMsgConfirm: 'Delete this message?',
    chatEditPrompt: 'Edit message:', chatAttach: 'Attach file',
    chatReply: 'Reply', chatReact: 'React', chatPin: 'Pin', chatUnpin: 'Unpin', chatPinChannel: 'Pin to top',
    chatPinned: 'Pinned message', chatReplyingTo: 'Replying to', chatCancelReply: 'Cancel',
    chatOptions: 'Options',
    chatTyping: '%s is typing…', chatSysCreate: '%s created the chat', chatSysJoin: '%s joined',
    chatSysLeave: '%s left', chatSysRename: 'Renamed to “%s”',
    chatReadonlyChannel: 'Read-only channel — only writers can post',
    chatReadonlyToggle: 'Read-only (only selected members can write)',
    chatMembersPick: 'Members', chatCanWrite: 'Can write',
    chatMute: 'Mute', chatUnmute: 'Unmute', chatMute1h: 'Mute 1 hour', chatMute8h: 'Mute 8 hours',
    chatMuteForever: 'Mute forever', chatMuted: 'Muted', chatSound: 'Notification sound',
  },
  ru: {
    chatChannels: 'Каналы', chatGroups: 'Группы', chatDirect: 'Личные сообщения',
    chatNewChannel: 'Новый канал', chatNewGroup: 'Новая группа', chatStartDM: 'Новое личное сообщение',
    chatSearchPh: 'Поиск сообщений…', chatNoChannels: 'Пока нет каналов',
    chatPickChannel: 'Выберите канал, чтобы начать общение', chatMessagePh: 'Написать сообщение…',
    chatTitle: 'Чат', chatNoGroups: 'Пока нет приватных групп',
    chatEmoji: 'Эмодзи', chatSend: 'Отправить', chatEdit: 'Изменить', chatDelete: 'Удалить', chatEdited: 'изменено',
    chatDeleted: 'сообщение удалено', chatReadonly: 'Только чтение', chatMembers: 'Участники',
    chatJoin: 'Вступить', chatLeave: 'Покинуть', chatChannelSettings: 'Настройки канала',
    chatChannelName: 'Название', chatTopic: 'Тема', chatSave: 'Сохранить', chatCancel: 'Отмена',
    chatAddMember: 'Добавить', chatLoadMore: 'Загрузить ранние сообщения', chatNoResults: 'Ничего не найдено',
    chatNameRequired: 'Введите название', chatConfirmLeave: 'Покинуть канал?',
    chatDeleteChannel: 'Удалить канал', chatDeleteChannelConfirm: 'Удалить канал и все его сообщения?',
    chatNoUsers: 'Нет других пользователей', chatDeleteMsgConfirm: 'Удалить сообщение?',
    chatEditPrompt: 'Изменить сообщение:', chatAttach: 'Прикрепить файл',
    chatReply: 'Ответить', chatReact: 'Реакция', chatPin: 'Закрепить', chatUnpin: 'Открепить', chatPinChannel: 'Закрепить наверху',
    chatPinned: 'Закреплённое', chatReplyingTo: 'Ответ', chatCancelReply: 'Отмена',
    chatOptions: 'Опции',
    chatTyping: '%s печатает…', chatSysCreate: '%s создал(а) чат', chatSysJoin: '%s присоединился(ась)',
    chatSysLeave: '%s покинул(а) чат', chatSysRename: 'Чат переименован в «%s»',
    chatReadonlyChannel: 'Канал только для чтения — пишут только избранные',
    chatReadonlyToggle: 'Только чтение (писать могут только выбранные участники)',
    chatMembersPick: 'Участники', chatCanWrite: 'Может писать',
    chatMute: 'Заглушить', chatUnmute: 'Включить уведомления', chatMute1h: 'Заглушить на 1 час', chatMute8h: 'Заглушить на 8 часов',
    chatMuteForever: 'Заглушить навсегда', chatMuted: 'Заглушено', chatSound: 'Звук уведомлений',
  },
  es: {
    chatChannels: "Canales", chatGroups: "Grupos", chatDirect: "Mensajes directos",
    chatNewChannel: "Nuevo canal", chatNewGroup: "Nuevo grupo", chatStartDM: "Nuevo mensaje directo",
    chatSearchPh: "Buscar mensajes…", chatNoChannels: "Aún no hay canales",
    chatPickChannel: "Elige un canal para empezar a chatear", chatMessagePh: "Escribe un mensaje…",
    chatEmoji: "Emoji", chatSend: "Enviar", chatEdit: "Editar", chatDelete: "Eliminar", chatEdited: "editado",
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
    chatEmoji: "Émoji", chatSend: "Envoyer", chatEdit: "Modifier", chatDelete: "Supprimer", chatEdited: "modifié",
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
    chatEmoji: "Emoji", chatSend: "Senden", chatEdit: "Bearbeiten", chatDelete: "Löschen", chatEdited: "bearbeitet",
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
    chatEmoji: "Emoji", chatSend: "Enviar", chatEdit: "Editar", chatDelete: "Excluir", chatEdited: "editado",
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
    // Base-en merge so keys missing from a locale fall back to English.
    const dict = Object.assign({}, CHAT_I18N.en, CHAT_I18N[lang] || {});
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
        <span class="chat-side-title">${t('chatTitle')}</span>
      </div>
      <div class="chat-channels" id="chat-channels"></div>
    </aside>
    <main class="chat-main" id="chat-main"></main>
  </div>`;
  chatRenderChannelList();
  chatRenderMain();
}

function chatChannelLabel(c) {
  if (c.type === 'dm') return c.peer_name || t('chatDirect');
  return c.name;
}
function chatChannelIcon(c) {
  return c.type === 'dm' ? '@' : (c.type === 'group' ? '&#128274;' : '#');
}

// === Avatars (initials on a color derived from the name) ===
const CHAT_AV_COLORS = ['#e17076', '#7bc862', '#65aadd', '#a695e7', '#ee7aae', '#6ec9cb', '#f3a85b', '#5ca3eb', '#df8ee5', '#b09b7a'];
function chatAvatarColor(name) {
  let h = 0;
  const s = name || '?';
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  return CHAT_AV_COLORS[h % CHAT_AV_COLORS.length];
}
function chatInitials(name) {
  const parts = (name || '?').trim().split(/\s+/).filter(Boolean);
  if (!parts.length) return '?';
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}
function chatAvatar(name, cls) {
  const bg = chatAvatarColor(name);
  const fg = (typeof contrastColor === 'function') ? contrastColor(bg) : '#fff';
  return `<span class="chat-av ${cls || ''}" style="background:${bg};color:${fg}">${esc(chatInitials(name))}</span>`;
}

// chatSysText localizes a system-message code ("kind|name"); used in the feed and
// in channel-list previews.
function chatSysCodes() { return ['create', 'join', 'leave', 'rename']; }
function chatSysText(code) {
  const i = code.indexOf('|');
  const k = i < 0 ? code : code.slice(0, i);
  const name = i < 0 ? '' : code.slice(i + 1);
  const key = { create: 'chatSysCreate', join: 'chatSysJoin', leave: 'chatSysLeave', rename: 'chatSysRename' }[k];
  return key ? t(key).replace('%s', name) : code;
}
function chatIsSysCode(text) {
  const i = text.indexOf('|');
  return i > 0 && chatSysCodes().includes(text.slice(0, i));
}
// chatListPreview renders the channel-list second line (localizes system codes).
function chatListPreview(c) {
  let txt = c.last_msg_text || '';
  if (!txt) return '';
  if (chatIsSysCode(txt)) return esc(chatSysText(txt));
  const from = (c.type !== 'dm' && c.last_msg_from) ? esc(c.last_msg_from) + ': ' : '';
  return from + esc(txt);
}
// chatListTime: today → HH:MM, otherwise short date.
function chatListTime(s) {
  if (!s) return '';
  const iso = s.includes('T') ? s : s.replace(' ', 'T') + 'Z';
  const d = new Date(iso), now = new Date();
  return d.toDateString() === now.toDateString() ? chatFmtTime(s) : chatTimeFmt(s, { day: '2-digit', month: '2-digit' });
}

// chatTogglePinChannel pins/unpins a CHANNEL in the sidebar. Distinct from
// chatTogglePin (defined later), which pins a *message* — a same-name function
// would shadow this one (last declaration wins) and break channel pinning.
async function chatTogglePinChannel(id) {
  try {
    await api('/api/chat/channels/' + id + '/pinchannel', 'POST', {});
    chatState.channels = await api('/api/chat/channels');
    chatRenderChannelList();
  } catch (e) { alert(e.message); }
}

// chatChannelRow renders one Telegram-style row: avatar + name/time + preview/badge.
function chatChannelRow(c, label, onclick, extraCls) {
  const badge = c.unread > 0 ? `<span class="chat-ch-badge">${c.unread > 99 ? '99+' : c.unread}</span>` : '';
  const preview = chatListPreview(c);
  const pin = `<button class="chat-ch-pin ${c.pinned ? 'pinned' : ''}" title="${escAttr(t('chatPinChannel'))}" onclick="event.stopPropagation();chatTogglePinChannel(${c.id})">&#128204;</button>`;
  return `<div class="chat-ch-row ${c.id === chatState.channelId ? 'active' : ''} ${extraCls || ''}" onclick="${onclick}">
    ${chatAvatar(label)}
    <div class="chat-ch-main">
      <div class="chat-ch-line1"><span class="chat-ch-name">${c.pinned ? '<span class="chat-ch-pinmark">&#128204;</span> ' : ''}${c.muted ? '<span class="chat-ch-mutemark" title="' + escAttr(t('chatMuted')) + '">&#128263;</span> ' : ''}${esc(label)}</span>${c.last_msg_at ? `<span class="chat-ch-time">${esc(chatListTime(c.last_msg_at))}</span>` : ''}</div>
      <div class="chat-ch-line2"><span class="chat-ch-preview">${preview}</span>${badge}</div>
    </div>
    ${pin}
  </div>`;
}
function chatCanManage(c) {
  return c && currentUser && (currentUser.role === 'admin' || c.created_by === currentUser.id);
}

function chatGrpAddBtn(type) {
  if (!currentUser || currentUser.role === 'readonly') return '';
  if (type === 'dm') return ''; // all contacts are listed directly now
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
    if (g.key === 'dm') { html += chatRenderDMGroup(g.label); continue; }
    const items = chatState.channels.filter(c => c.type === g.key);
    html += `<div class="chat-grp">
      <div class="chat-grp-head"><span>${esc(g.label)}</span>${chatGrpAddBtn(g.key)}</div>`;
    for (const c of items) {
      html += chatChannelRow(c, chatChannelLabel(c), `chatOpen(${c.id})`);
    }
    // Show a hint (instead of hiding the section) so the "+" stays reachable —
    // this is the only way to create the first private group.
    if (!items.length) html += `<div class="chat-empty">${esc(g.key === 'group' ? t('chatNoGroups') : t('chatNoChannels'))}</div>`;
    html += `</div>`;
  }
  el.innerHTML = html || `<div class="chat-empty">${t('chatNoChannels')}</div>`;
}

// chatRenderDMGroup lists every other user as a contact: an existing DM opens it,
// a contact without a DM yet creates one on click.
function chatRenderDMGroup(label) {
  const meId = currentUser && currentUser.id;
  const canWrite = currentUser && currentUser.role !== 'readonly';
  // Existing DMs first, ordered by last message (channels already come sorted desc);
  // then contacts without a DM yet, alphabetically.
  const havePeer = {};
  let rows = '';
  for (const c of chatState.channels) {
    if (c.type !== 'dm') continue;
    if (c.peer_id != null) havePeer[c.peer_id] = true;
    rows += chatChannelRow(c, chatChannelLabel(c), `chatOpen(${c.id})`);
  }
  if (canWrite) {
    const contacts = (state.users || []).filter(u => u.id !== meId && !havePeer[u.id])
      .sort((a, b) => (a.username || '').localeCompare(b.username || ''));
    for (const u of contacts) {
      rows += `<div class="chat-ch-row chat-contact" onclick="chatStartDM(${u.id})">
        ${chatAvatar(u.username)}
        <div class="chat-ch-main"><div class="chat-ch-line1"><span class="chat-ch-name">${esc(u.username)}</span></div></div>
      </div>`;
    }
  }
  let h = `<div class="chat-grp"><div class="chat-grp-head"><span>${esc(label)}</span></div>`;
  h += rows || `<div class="chat-empty">${t('chatNoUsers')}</div>`;
  h += `</div>`;
  return h;
}

// chatStartDM opens (or creates) the direct-message channel with a user.
async function chatStartDM(userId) {
  try {
    const res = await api('/api/chat/dm', 'POST', { user_id: userId });
    chatState.channels = await api('/api/chat/channels');
    navigateTo('/chat/channel/' + res.id);
  } catch (e) { alert(e.message); }
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
      ${chatAvatar(chatChannelLabel(c), 'chat-av-sm')}
      <div class="chat-head-info">
        <h2 class="chat-head-title">${esc(chatChannelLabel(c))}</h2>
        ${c.topic ? `<div class="chat-head-topic">${esc(c.topic)}</div>` : ''}
        <div class="chat-head-typing" id="chat-typing"></div>
      </div>
      <div class="chat-head-actions">${chatHeadActions(c)}</div>
    </div>
    <div class="chat-pinned-bar hidden" id="chat-pinned-bar"></div>
    <div class="chat-msgs" id="chat-msgs"></div>
    ${chatComposer()}`;
  chatRenderMessages();
  chatRenderPinnedBar();
  chatScrollBottom();
  const inp = document.getElementById('chat-input');
  if (inp) {
    inp.addEventListener('keydown', e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); chatSend(); } });
    inp.addEventListener('input', chatAutoGrow);
    inp.addEventListener('input', chatOnType);
    // Paste images from the clipboard → upload and insert a markdown image link.
    if (typeof setupPaste === 'function') setupPaste(inp);
  }
  chatRenderTyping();
}

async function chatCall(video) {
  const c = chatState.channel;
  if (!c) return;
  let members = [];
  if (c.type === 'dm') { if (c.peer_id) members = [c.peer_id]; }
  else { try { const ms = await api(`/api/chat/channels/${c.id}/members`); members = ms.map(m => m.user_id); } catch (e) { /* ignore */ } }
  if (typeof callStart === 'function') callStart(c.id, members, !!video);
}

function chatHeadActions(c) {
  // Per-conversation mute (the global notification-sound toggle lives in
  // Settings → Account, since it isn't channel-specific).
  let h = '';
  if (typeof callStart === 'function' && currentUser && currentUser.role !== 'readonly' && typeof RTCPeerConnection !== 'undefined') {
    h += `<button class="btn btn-ghost btn-sm" title="${t('callAudio')}" onclick="chatCall(false)">&#128222;</button>`;
    h += `<button class="btn btn-ghost btn-sm" title="${t('callVideo')}" onclick="chatCall(true)">&#127909;</button>`;
  }
  h += `<button class="btn btn-ghost btn-sm" title="${c.muted ? t('chatMuted') : t('chatMute')}" onclick="chatMuteMenu(event)">${c.muted ? '&#128263;' : '&#128266;'}</button>`;
  if (c.type === 'dm') return h;
  h += `<button class="btn btn-ghost btn-sm" onclick="chatMembersDialog()">${t('chatMembers')} (${c.member_count || 0})</button>`;
  if (currentUser && currentUser.role !== 'readonly') {
    if (c.type === 'channel' && !c.is_member) h += `<button class="btn btn-ghost btn-sm" onclick="chatJoin()">${t('chatJoin')}</button>`;
    else if (c.is_member) h += `<button class="btn btn-ghost btn-sm" onclick="chatLeave()">${t('chatLeave')}</button>`;
  }
  if (chatCanManage(c)) h += `<button class="btn btn-ghost btn-sm" title="${t('chatChannelSettings')}" onclick="chatChannelSettings()">&#9881;</button>`;
  return h;
}

// === Sound + mute ===
let chatAudioCtx = null;
function chatSoundOn() { return localStorage.getItem('chat-sound') !== 'off'; }
function chatToggleSound() {
  localStorage.setItem('chat-sound', chatSoundOn() ? 'off' : 'on');
  if (chatState.channel) chatRenderMain();
}
function chatBeep() {
  if (!chatSoundOn()) return;
  try {
    chatAudioCtx = chatAudioCtx || new (window.AudioContext || window.webkitAudioContext)();
    const ctx = chatAudioCtx;
    if (ctx.state === 'suspended') ctx.resume();
    const o = ctx.createOscillator(), g = ctx.createGain();
    o.connect(g); g.connect(ctx.destination);
    o.type = 'sine'; o.frequency.value = 680;
    g.gain.setValueAtTime(0.0001, ctx.currentTime);
    g.gain.exponentialRampToValueAtTime(0.12, ctx.currentTime + 0.02);
    g.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + 0.25);
    o.start(); o.stop(ctx.currentTime + 0.26);
  } catch (e) { /* audio unavailable */ }
}
function chatMaybeBeep(ev) {
  if (!ev || !ev.message) return;
  if (ev.message.author_id && currentUser && ev.message.author_id === currentUser.id) return;
  const ch = (chatState.channels || []).find(c => c.id === ev.channel_id);
  if (ch && ch.muted) return;
  chatBeep();
}
function chatMuteMenu(ev) {
  ev.stopPropagation();
  document.querySelectorAll('.chat-mute-menu').forEach(m => m.remove());
  const c = chatState.channel; if (!c) return;
  const m = document.createElement('div');
  m.className = 'chat-mute-menu chat-emoji-pop';
  const opt = (label, arg) => `<button class="chat-mute-opt" onclick="chatMuteSet(${arg})">${label}</button>`;
  m.innerHTML = c.muted ? opt(t('chatUnmute'), 'null')
    : opt(t('chatMute1h'), '3600') + opt(t('chatMute8h'), '28800') + opt(t('chatMuteForever'), '0');
  document.body.appendChild(m);
  const r = ev.currentTarget.getBoundingClientRect();
  m.style.top = (r.bottom + 4) + 'px';
  m.style.left = Math.max(8, r.right - m.offsetWidth) + 'px';
  setTimeout(() => document.addEventListener('click', function hide() { m.remove(); document.removeEventListener('click', hide); }), 0);
}
async function chatMuteSet(secs) {
  const c = chatState.channel; if (!c) return;
  try {
    if (secs === null) await api(`/api/chat/channels/${c.id}/mute`, 'DELETE');
    else { const until = secs === 0 ? 0 : Math.floor(Date.now() / 1000) + secs; await api(`/api/chat/channels/${c.id}/mute`, 'POST', { until }); }
    await chatReloadChannel();
    chatState.channels = await api('/api/chat/channels');
    chatRenderChannelList();
  } catch (e) { alert(e.message); }
}

function chatComposer() {
  if (currentUser && currentUser.role === 'readonly') return `<div class="chat-readonly">${t('chatReadonly')}</div>`;
  const c = chatState.channel;
  if (c && c.read_only && !c.can_write) return `<div class="chat-readonly">&#128274; ${t('chatReadonlyChannel')}</div>`;
  return `<div class="chat-reply-bar hidden" id="chat-reply-bar"></div>
  <div class="chat-pending" id="chat-pending"></div>
  <div class="chat-composer">
    <button class="btn btn-ghost btn-sm chat-attach-btn" title="${t('chatAttach')}" onclick="chatAttachFile()">&#128206;</button>
    <button class="btn btn-ghost btn-sm chat-emoji-btn" title="${t('chatEmoji')}" onclick="chatEmojiPicker(event)">&#128512;</button>
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
      // Open in the shared lightbox instead of downloading.
      return `<img class="chat-att-img" src="${escAttr(a.url)}" alt="${escAttr(a.filename)}" onclick="openLightbox('${escAttr(a.url)}')">`;
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
    if (msg.kind === 'system') { html += `<div class="chat-sys" id="chat-msg-${msg.id}"><span>${esc(chatSysText(msg.text || ''))}</span></div>`; lastAuthor = null; continue; }
    if (msg.deleted) { html += `<div class="chat-sys chat-del-row" id="chat-msg-${msg.id}"><span>${t('chatDeleted')}</span></div>`; lastAuthor = null; continue; }
    const grouped = msg.author_id === lastAuthor;
    html += chatRenderMsg(msg, grouped);
    lastAuthor = msg.author_id;
  }
  box.innerHTML = html;
}

// chatChecks renders sent/read ticks for own DM messages.
function chatChecks(msg) {
  const peer = (chatState.channel && chatState.channel.peer_last_read) || 0;
  const read = peer >= msg.id;
  return `<span class="chat-checks ${read ? 'read' : ''}">${read ? '&#10003;&#10003;' : '&#10003;'}</span>`;
}

function chatRenderMsg(msg, grouped) {
  const mine = currentUser && msg.author_id === currentUser.id;
  const c = chatState.channel;
  const isDM = c && c.type === 'dm';
  const showName = !mine && !grouped && c && c.type !== 'dm';
  const canManage = mine || (currentUser && currentUser.role === 'admin');
  const canWrite = currentUser && currentUser.role !== 'readonly';
  const avatar = mine ? '' : (grouped ? '<span class="chat-av-spacer"></span>' : chatAvatar(msg.author_name || '?', 'chat-av-sm'));
  const checks = (mine && isDM) ? chatChecks(msg) : '';
  const meta = `<span class="chat-meta">${msg.pinned ? `<span class="chat-pin-mark" title="${t('chatPinned')}">&#128204;</span>` : ''}${msg.edited_at ? `<span class="chat-edited">${t('chatEdited')}</span>` : ''}<span class="chat-time">${esc(chatFmtTime(msg.created_at))}</span>${checks}</span>`;
  return `<div class="chat-msg ${mine ? 'mine' : 'their'} ${grouped ? 'grouped' : ''} ${msg.pinned ? 'pinned' : ''}" id="chat-msg-${msg.id}">
    ${avatar}
    <div class="chat-bubble-wrap">
      <div class="chat-bubble" ondblclick="chatStartReply(${msg.id})" title="${escAttr(t('chatReply'))}">
        ${showName ? `<div class="chat-bubble-author" style="color:${chatAvatarColor(msg.author_name || '?')}">${esc(msg.author_name || '?')}</div>` : ''}
        ${msg.reply_to ? chatReplyQuote(msg) : ''}
        <div class="chat-text md-content">${chatFormatText(msg.text)}</div>
        ${chatRenderAttachments(msg)}
        ${meta}
      </div>
      ${chatReactionChips(msg)}
      ${(canWrite || canManage) ? `<button class="chat-msg-menu-btn" title="${t('chatOptions')}" onclick="chatMsgMenu(event, ${msg.id})">&#8943;</button>` : ''}
    </div>
  </div>`;
}

// chatMsgMenu opens a dropdown with the message actions (reply/react/pin/edit/delete).
function chatMsgMenu(ev, id) {
  ev.stopPropagation();
  document.querySelectorAll('.chat-msg-menu').forEach(m => m.remove());
  const msg = chatState.messages.find(m => m.id === id);
  if (!msg || msg.deleted) return;
  const mine = currentUser && msg.author_id === currentUser.id;
  const canManage = mine || (currentUser && currentUser.role === 'admin');
  const canWrite = currentUser && currentUser.role !== 'readonly';
  const items = [];
  if (canWrite) items.push(`<a onclick="chatStartReply(${id})">${t('chatReply')}</a>`);
  if (canWrite) items.push(`<a class="chat-menu-react" onclick="chatReactPick(event, ${id})">${t('chatReact')}</a>`);
  if (chatCanPin()) items.push(`<a onclick="chatTogglePin(${id}, ${msg.pinned ? 0 : 1})">${msg.pinned ? t('chatUnpin') : t('chatPin')}</a>`);
  if (mine) items.push(`<a onclick="chatEditMsg(${id})">${t('chatEdit')}</a>`);
  if (canManage) items.push(`<a class="chat-danger" onclick="chatDeleteMsg(${id})">${t('chatDelete')}</a>`);
  if (!items.length) return;
  const menu = document.createElement('div');
  menu.className = 'chat-msg-menu';
  menu.innerHTML = items.join('');
  document.body.appendChild(menu);
  const r = ev.target.getBoundingClientRect();
  // Open downward; flip above the button if it would overflow the viewport bottom.
  const mh = menu.offsetHeight;
  let top = r.bottom + 4;
  if (top + mh > window.innerHeight - 8) top = Math.max(8, r.top - mh - 4);
  menu.style.top = top + 'px';
  menu.style.left = Math.max(8, Math.min(r.left, window.innerWidth - menu.offsetWidth - 8)) + 'px';
  // Close the menu after an action (React keeps its own flow / opens the emoji pop).
  menu.addEventListener('click', e => { const a = e.target.closest('a'); if (a && !a.classList.contains('chat-menu-react')) menu.remove(); });
  const close = e => { if (!e.target.closest('.chat-msg-menu')) { menu.remove(); document.removeEventListener('mousedown', close, true); } };
  setTimeout(() => document.addEventListener('mousedown', close, true), 0);
}

// chatCanPin: managers can pin in public channels; any member in groups/DMs.
function chatCanPin() {
  const c = chatState.channel;
  if (!c || !currentUser || currentUser.role === 'readonly') return false;
  return c.type === 'channel' ? chatCanManage(c) : true;
}

function chatReplyQuote(msg) {
  return `<div class="chat-reply-quote" onclick="chatJumpTo(${msg.reply_to})">
    <b>${esc(msg.reply_to_author || '?')}</b> <span>${esc((msg.reply_to_text || '').slice(0, 120))}</span></div>`;
}

function chatReactionChips(msg) {
  if (!msg.reactions || !msg.reactions.length) return '';
  return `<div class="chat-reactions">` + msg.reactions.map(r =>
    `<button class="chat-react-chip ${r.mine ? 'mine' : ''}" onclick="chatReact(${msg.id}, '${escAttr(r.emoji)}')">${esc(r.emoji)} ${r.count}</button>`
  ).join('') + `</div>`;
}

// chatFormatText renders a message as Markdown (same renderer as task
// descriptions/comments): markdown, @mentions, links opening in a new tab, and
// inline images (pasted/linked). The result is wrapped in .md-content by the
// caller, so images get the shared lightbox on click.
function chatFormatText(text) {
  if (typeof renderMd === 'function') return renderMd(text || '');
  // Fallback: escaped text with linkify + mentions.
  let h = esc(text).replace(/\n/g, '<br>');
  h = h.replace(/(https?:\/\/[^\s<]+)/g, url => `<a href="${url}" target="_blank" rel="noopener noreferrer">${url}</a>`);
  h = h.replace(/(^|[\s>])@([\p{L}\p{N}_.\-]+)/gu, (full, pre, name) => {
    const u = (state.users || []).find(x => x.username === name);
    return u ? `${pre}<span class="chat-mention">@${esc(name)}</span>` : full;
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
  chatState.replyTo = null;
  chatState.pinned = [];
  chatState.pinnedIdx = 0;
  chatTypingUsers = {};
  chatRenderMain();
  chatLoadPinned();
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
  const replyTo = chatState.replyTo ? chatState.replyTo.id : 0;
  try {
    const msg = await api(`/api/chat/channels/${chatState.channelId}/messages`, 'POST', { text, attachments: atts, reply_to: replyTo });
    chatCancelReply();
    chatAppendMsg(msg);
    chatMarkRead();
  } catch (e) { inp.value = text; chatState.pending = atts; chatRenderPending(); alert(e.message); }
}

// === Typing indicator ===
let chatTypingSent = 0;       // throttle outgoing typing pings
let chatTypingUsers = {};     // id -> { name, until }
let chatTypingTimer = null;
function chatOnType() {
  if (!chatState.channelId) return;
  const now = Date.now();
  if (now - chatTypingSent < 2500) return;
  chatTypingSent = now;
  api(`/api/chat/channels/${chatState.channelId}/typing`, 'POST', {}).catch(() => {});
}
function chatShowTyping(uid, name) {
  if (!uid || uid === (currentUser && currentUser.id)) return;
  chatTypingUsers[uid] = { name: name || '?', until: Date.now() + 4500 };
  chatRenderTyping();
  clearTimeout(chatTypingTimer);
  chatTypingTimer = setTimeout(chatPruneTyping, 1200);
}
function chatPruneTyping() {
  const now = Date.now();
  for (const id in chatTypingUsers) if (chatTypingUsers[id].until < now) delete chatTypingUsers[id];
  chatRenderTyping();
  if (Object.keys(chatTypingUsers).length) chatTypingTimer = setTimeout(chatPruneTyping, 1200);
}
function chatClearTyping(uid) { if (chatTypingUsers[uid]) { delete chatTypingUsers[uid]; chatRenderTyping(); } }
function chatRenderTyping() {
  const el = document.getElementById('chat-typing');
  if (!el) return;
  const names = Object.values(chatTypingUsers).map(x => x.name);
  if (!names.length) { el.classList.remove('on'); el.textContent = ''; return; }
  el.classList.add('on');
  el.textContent = t('chatTyping').replace('%s', names.join(', '));
}

// === Reply ===
function chatStartReply(id) {
  const msg = chatState.messages.find(m => m.id === id);
  if (!msg) return;
  chatState.replyTo = { id: msg.id, author_name: msg.author_name, text: msg.text };
  chatRenderReplyBar();
  const inp = document.getElementById('chat-input');
  if (inp) inp.focus();
}
function chatCancelReply() { chatState.replyTo = null; chatRenderReplyBar(); }
function chatRenderReplyBar() {
  const el = document.getElementById('chat-reply-bar');
  if (!el) return;
  const r = chatState.replyTo;
  if (!r) { el.classList.add('hidden'); el.innerHTML = ''; return; }
  el.classList.remove('hidden');
  el.innerHTML = `<div class="chat-reply-bar-info">
      <b>${t('chatReplyingTo')} ${esc(r.author_name || '?')}</b>
      <span>${esc((r.text || '').slice(0, 120))}</span>
    </div>
    <button class="btn-icon btn-sm" title="${t('chatCancelReply')}" onclick="chatCancelReply()">&#10005;</button>`;
}

// === Reactions ===
function chatReactPick(ev, id) {
  ev.stopPropagation();
  document.querySelectorAll('.chat-react-pop, .chat-msg-menu').forEach(p => p.remove());
  const pop = document.createElement('div');
  pop.className = 'chat-react-pop';
  pop.innerHTML = CHAT_EMOJI.map(e => `<button class="chat-react-opt" onclick="chatReact(${id}, '${escAttr(e)}')">${esc(e)}</button>`).join('');
  document.body.appendChild(pop);
  const rect = ev.target.getBoundingClientRect();
  pop.style.left = Math.max(8, Math.min(rect.left, window.innerWidth - pop.offsetWidth - 8)) + 'px';
  pop.style.top = (rect.bottom + 4) + 'px';
  const close = e => { if (!e.target.closest('.chat-react-pop')) { pop.remove(); document.removeEventListener('mousedown', close, true); } };
  setTimeout(() => document.addEventListener('mousedown', close, true), 0);
}

async function chatReact(id, emoji) {
  document.querySelectorAll('.chat-react-pop').forEach(p => p.remove());
  try {
    const upd = await api('/api/chat/messages/' + id + '/react', 'POST', { emoji });
    chatMergeMsg(upd);
  } catch (e) { alert(e.message); }
}

// chatMergeMsg replaces a message in place, preserving scroll (used for reactions/pins).
function chatMergeMsg(msg) {
  const i = chatState.messages.findIndex(m => m.id === msg.id);
  if (i >= 0) { chatState.messages[i] = msg; chatRenderMessages(); }
}

// === Pins ===
async function chatTogglePin(id, pin) {
  try {
    const upd = await api('/api/chat/messages/' + id + '/pin', 'POST', { pinned: !!pin });
    chatMergeMsg(upd);
    await chatLoadPinned();
  } catch (e) { alert(e.message); }
}

async function chatLoadPinned() {
  if (!chatState.channelId) return;
  try { chatState.pinned = await api(`/api/chat/channels/${chatState.channelId}/pins`); }
  catch (e) { chatState.pinned = []; }
  if (chatState.pinnedIdx >= chatState.pinned.length) chatState.pinnedIdx = 0;
  chatRenderPinnedBar();
}

function chatRenderPinnedBar() {
  const el = document.getElementById('chat-pinned-bar');
  if (!el) return;
  const pins = chatState.pinned || [];
  if (!pins.length) { el.classList.add('hidden'); el.innerHTML = ''; return; }
  if (chatState.pinnedIdx >= pins.length) chatState.pinnedIdx = 0;
  const p = pins[chatState.pinnedIdx];
  el.classList.remove('hidden');
  el.innerHTML = `<div class="chat-pinned-inner" onclick="chatJumpTo(${p.id})">
      <span class="chat-pinned-icon">&#128204;</span>
      <div class="chat-pinned-text"><b>${t('chatPinned')}${pins.length > 1 ? ` (${chatState.pinnedIdx + 1}/${pins.length})` : ''}</b>
        <span>${esc((p.text || '').slice(0, 140)) || '&#128206;'}</span></div>
    </div>
    <div class="chat-pinned-actions">
      ${pins.length > 1 ? `<button class="btn-icon btn-sm" title="next" onclick="event.stopPropagation();chatCyclePinned()">&#9660;</button>` : ''}
      ${chatCanPin() ? `<button class="btn-icon btn-sm" title="${t('chatUnpin')}" onclick="event.stopPropagation();chatTogglePin(${p.id},0)">&#10005;</button>` : ''}
    </div>`;
}

function chatCyclePinned() {
  if (!chatState.pinned.length) return;
  chatState.pinnedIdx = (chatState.pinnedIdx + 1) % chatState.pinned.length;
  chatRenderPinnedBar();
}

// chatJumpTo scrolls to a message if it is currently loaded and flashes it.
function chatJumpTo(id) {
  const el = document.getElementById('chat-msg-' + id);
  if (!el) return;
  el.scrollIntoView({ block: 'center', behavior: 'smooth' });
  el.classList.add('chat-flash');
  setTimeout(() => el.classList.remove('chat-flash'), 1500);
}

function chatAppendMsg(msg) {
  if (msg.channel_id !== chatState.channelId) return;
  if (chatState.messages.some(m => m.id === msg.id)) return;
  chatState.messages.push(msg);
  chatRenderMessages();
  chatScrollBottom();
}

// chatEditMsg edits in place with a textarea matching the composer (multiline,
// markdown, paste images, Enter to save / Shift+Enter newline / Esc cancel) so
// the raw message text is never mangled the way a single-line prompt did.
function chatEditMsg(id) {
  const msg = chatState.messages.find(m => m.id === id);
  if (!msg) return;
  const row = document.getElementById('chat-msg-' + id);
  if (!row) return;
  const textEl = row.querySelector('.chat-text');
  if (!textEl || row.querySelector('.chat-edit-box')) return;
  const box = document.createElement('div');
  box.className = 'chat-edit-box';
  box.innerHTML = `<textarea class="chat-edit-input" rows="2"></textarea>
    <div class="chat-edit-actions">
      <button class="btn btn-ghost btn-sm" data-act="cancel">${t('chatCancel')}</button>
      <button class="btn btn-primary btn-sm" data-act="save">${t('chatSave')}</button>
    </div>`;
  textEl.style.display = 'none';
  textEl.parentNode.insertBefore(box, textEl.nextSibling);
  const ta = box.querySelector('textarea');
  ta.value = msg.text;
  ta.style.height = 'auto'; ta.style.height = Math.min(200, ta.scrollHeight) + 'px';
  ta.focus();
  if (typeof setupPaste === 'function') setupPaste(ta);
  const finish = () => { box.remove(); textEl.style.display = ''; };
  const save = async () => {
    const v = ta.value.trim();
    if (!v || v === msg.text) { finish(); return; }
    try { const upd = await api('/api/chat/messages/' + id, 'PUT', { text: v }); chatReplaceMsg(upd); }
    catch (e) { alert(e.message); }
  };
  box.querySelector('[data-act="cancel"]').onclick = finish;
  box.querySelector('[data-act="save"]').onclick = save;
  ta.addEventListener('keydown', e => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); save(); }
    else if (e.key === 'Escape') { e.preventDefault(); finish(); }
  });
}

async function chatDeleteMsg(id) {
  if (!await uiConfirm(t('chatDeleteMsgConfirm'))) return;
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
    case 'chat_message': {
      const active = ev.channel_id === chatState.channelId;
      if (ev.message && active) {
        if (ev.message.author_id) chatClearTyping(ev.message.author_id);
        chatAppendMsg(ev.message);
        if (document.hasFocus()) chatMarkRead();
      }
      // Beep for incoming messages unless you're actively looking at the channel.
      if (!(active && document.hasFocus())) chatMaybeBeep(ev);
      chatRefreshChannelsSoon();
      break;
    }
    case 'chat_typing':
      if (ev.channel_id === chatState.channelId) chatShowTyping(ev.user_id, ev.user_name);
      break;
    case 'chat_read':
      if (ev.channel_id === chatState.channelId && ev.user_id && ev.user_id !== (currentUser && currentUser.id) && chatState.channel) {
        if ((ev.message_id || 0) > (chatState.channel.peer_last_read || 0)) {
          chatState.channel.peer_last_read = ev.message_id;
          chatRenderMessages();
        }
      }
      break;
    case 'chat_edit':
      // Preserve locally-known reactions ("mine" is per-viewer, not in the broadcast).
      if (ev.message && ev.channel_id === chatState.channelId) {
        const i = chatState.messages.findIndex(m => m.id === ev.message.id);
        if (i >= 0) { ev.message.reactions = chatState.messages[i].reactions; chatState.messages[i] = ev.message; chatRenderMessages(); }
      }
      break;
    case 'chat_delete':
      if (ev.message && ev.channel_id === chatState.channelId) chatReplaceMsg(ev.message);
      break;
    case 'chat_pin':
      if (ev.channel_id === chatState.channelId) {
        if (ev.message) chatMergeMsg(ev.message);
        chatLoadPinned();
      }
      break;
    case 'chat_react':
      if (ev.channel_id === chatState.channelId && ev.message_id) chatRefetchMsg(ev.message_id);
      break;
    case 'chat_channels':
      chatRefreshChannelsSoon();
      break;
  }
}

// chatRefetchMsg reloads one message (viewer-aware) and updates it in place.
async function chatRefetchMsg(id) {
  if (!chatState.messages.some(m => m.id === id)) return;
  try { const m = await api('/api/chat/messages/' + id); chatMergeMsg(m); } catch (e) { /* ignore */ }
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
  const meId = currentUser && currentUser.id;
  // Private groups get a member picker; public channels get a read-only toggle.
  const memberPicker = (type === 'group') ? `
    <div class="chat-fld"><span>${t('chatMembersPick')}</span>
      <div class="chat-member-pick">${(state.users || []).filter(u => u.id !== meId)
        .map(u => `<label class="chat-pick-row"><input type="checkbox" value="${u.id}"> ${esc(u.username)}</label>`).join('') || `<div class="chat-empty">${t('chatNoUsers')}</div>`}</div>
    </div>` : '';
  const roToggle = (type === 'channel') ? `
    <label class="chat-pick-row"><input type="checkbox" id="chat-new-ro"> ${t('chatReadonlyToggle')}</label>` : '';
  const body = `
    <label class="chat-fld">${t('chatChannelName')}<input id="chat-new-name" maxlength="100"></label>
    <label class="chat-fld">${t('chatTopic')}<input id="chat-new-topic" maxlength="300"></label>
    ${memberPicker}${roToggle}`;
  chatModal(type === 'group' ? t('chatNewGroup') : t('chatNewChannel'), body, async ov => {
    const name = ov.querySelector('#chat-new-name').value.trim();
    if (!name) { alert(t('chatNameRequired')); return; }
    const topic = ov.querySelector('#chat-new-topic').value.trim();
    const members = Array.from(ov.querySelectorAll('.chat-member-pick input:checked')).map(i => Number(i.value));
    const roEl = ov.querySelector('#chat-new-ro');
    try {
      const res = await api('/api/chat/channels', 'POST', { name, topic, type, members });
      if (roEl && roEl.checked) {
        try { await api('/api/chat/channels/' + res.id, 'PUT', { name, topic, read_only: true }); } catch (e) { /* ignore */ }
      }
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
  const canManageWriters = chatCanManage(c) && c.read_only && c.type !== 'dm';
  const others = (state.users || []).filter(u => !members.some(m => m.user_id === u.id));
  const body = `
    <div class="chat-mem-list">${members.map(m => `<div class="chat-mem-row">
      <span>${esc(m.username)}</span>
      ${canManageWriters && m.user_id !== c.created_by ? `<label class="chat-writer-tog"><input type="checkbox" ${m.role === 'writer' ? 'checked' : ''} onchange="chatSetWriter(${m.user_id}, this.checked)"> ${t('chatCanWrite')}</label>` : ''}
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

async function chatSetWriter(uid, canWrite) {
  try {
    await api(`/api/chat/channels/${chatState.channelId}/members`, 'POST', { user_id: uid, can_write: !!canWrite });
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
  if (!await uiConfirm(t('chatConfirmLeave'))) return;
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
  const roToggle = c.type !== 'dm' ? `
    <label class="chat-pick-row"><input type="checkbox" id="chat-set-ro" ${c.read_only ? 'checked' : ''}> ${t('chatReadonlyToggle')}</label>` : '';
  const body = `
    <label class="chat-fld">${t('chatChannelName')}<input id="chat-set-name" maxlength="100" value="${escAttr(c.name)}"></label>
    <label class="chat-fld">${t('chatTopic')}<input id="chat-set-topic" maxlength="300" value="${escAttr(c.topic)}"></label>
    ${roToggle}
    <button class="btn btn-ghost btn-sm chat-danger chat-set-del" onclick="chatDeleteChannel()">${t('chatDeleteChannel')}</button>`;
  chatModal(t('chatChannelSettings'), body, async ov => {
    const name = ov.querySelector('#chat-set-name').value.trim();
    const topic = ov.querySelector('#chat-set-topic').value.trim();
    if (!name) { alert(t('chatNameRequired')); return; }
    const roEl = ov.querySelector('#chat-set-ro');
    const payload = { name, topic };
    if (roEl) payload.read_only = roEl.checked;
    try {
      await api(`/api/chat/channels/${c.id}`, 'PUT', payload);
      ov.remove();
      await chatReloadChannel();
    } catch (e) { alert(e.message); }
  });
}

async function chatDeleteChannel() {
  const c = chatState.channel;
  if (!c) return;
  if (!await uiConfirm(t('chatDeleteChannelConfirm'))) return;
  try {
    await api(`/api/chat/channels/${c.id}`, 'DELETE');
    document.querySelectorAll('.modal-overlay').forEach(m => m.remove());
    localStorage.removeItem(CHAT_CHANNEL_KEY);
    chatState.channelId = null;
    chatState.channel = null;
    navigateTo('/chat');
  } catch (e) { alert(e.message); }
}
