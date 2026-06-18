// Admin page. Reached from the bottom app-rail gear (admins only) at /admin.
// Renders a tabbed page in a persistent top-level #admin-page element and relocates
// the existing admin-only sidebar sections into its panels (ids preserved, so all
// existing populate functions keep working). Phase 7 adds Mail server / Mailboxes.

const ADMIN_I18N = {
  en: { adminTitle: 'Administration', settingsTitle: 'Settings', adminTabGeneral: 'General', adminTabAccount: 'Account', accProfile: 'Open profile', adminTabSite: 'Site', adminAppName: 'Application name', adminAppNameHint: 'Shown in the page title, header and backup names.', adminDefaultLang: 'Default language', adminDefaultLangHint: 'Interface language for new users (each user can change their own afterwards).', obReplayTitle: 'First-run wizard', obReplay: 'Show setup wizard again', obReplayHint: 'Re-open the one-time service-setup wizard (clears the completed flag).', adminTabTelegram: 'Telegram', adminTabMail: 'Mail', adminTabUsers: 'Users', adminTabBoard: 'Board', adminTabServices: 'Services', adminTabIntegrations: 'Integrations', adminTabData: 'Data', adminTabMailServer: 'Mail server', adminTabMailboxes: 'Mailboxes', adminMailDomain: 'Sending domain', adminDkimSelector: 'DKIM selector', adminDkimKey: 'DKIM private key (PEM)', adminDkimKeyHint: 'leave blank to keep current', adminDkimTxt: 'Publish this DNS TXT record', adminSave: 'Save', adminAudit: 'Security audit', adminAlias: 'Alias', adminPrimary: 'Primary', adminAdd: 'Add', adminUser: 'User', adminNoData: 'No data', adminMailDisabled: 'Outbound mail is off until a sending domain is set.', adminClear: 'Clear key', llmTitle: 'AI summaries (LLM)', dbTitle: 'Database', llmHint: 'Connect an OpenAI-compatible API (e.g. llama.cpp). When enabled: a "Summary" button on wiki pages and "✨ Generate" (description + TODO) in the task form.', llmEnabledOn: 'Enabled', llmBaseUrl: 'API base URL', llmModel: 'Model', llmApiKey: 'API key', llmApiKeyOptional: 'optional', llmEnable: 'Enable (tests connection)', llmTest: 'Test connection', llmTesting: 'Testing…', llmOk: 'Connection OK', dbHost: 'Host', dbPort: 'Port', dbName: 'Database', dbUser: 'User', dbPath: 'File', adminTabAudit: 'Audit log', auditTime: 'Time', auditUser: 'User', auditAction: 'Action', auditProject: 'Project', auditDetails: 'Details', auditAllProjects: 'All projects', adminTabSecurity: 'Security', twofaTitle: 'Two-factor authentication', twofaHint: 'Add a time-based one-time code (TOTP) from an authenticator app as a second login step.', twofaOn: 'Two-factor authentication is enabled.', twofaEnable: 'Enable 2FA', twofaDisable: 'Disable 2FA', twofaCode: 'Code', twofaScan: 'Add this secret to your authenticator app (or scan the otpauth URL), then enter the current code:', twofaConfirm: 'Confirm', twofaEnabled: '2FA enabled.', twofaRecovery: 'Save these one-time recovery codes somewhere safe:' },
  ru: { adminTitle: 'Администрирование', settingsTitle: 'Настройки', adminTabGeneral: 'Общие', adminTabAccount: 'Аккаунт', accProfile: 'Открыть профиль', adminTabSite: 'Сайт', adminAppName: 'Название приложения', adminAppNameHint: 'Показывается в заголовке вкладки, шапке и именах бэкапов.', adminDefaultLang: 'Язык по умолчанию', adminDefaultLangHint: 'Язык интерфейса для новых пользователей (каждый может потом сменить свой).', obReplayTitle: 'Визард первого запуска', obReplay: 'Показать визард заново', obReplayHint: 'Заново открыть одноразовый визард настройки сервисов (сбрасывает флаг завершения).', adminTabTelegram: 'Telegram', adminTabMail: 'Почта', adminTabUsers: 'Пользователи', adminTabBoard: 'Доска', adminTabServices: 'Сервисы', adminTabIntegrations: 'Интеграции', adminTabData: 'Данные', adminTabMailServer: 'Почтовый сервер', adminTabMailboxes: 'Ящики', adminMailDomain: 'Домен отправки', adminDkimSelector: 'DKIM селектор', adminDkimKey: 'Приватный ключ DKIM (PEM)', adminDkimKeyHint: 'пусто — оставить текущий', adminDkimTxt: 'Опубликуйте эту DNS TXT-запись', adminSave: 'Сохранить', adminAudit: 'Аудит безопасности', adminAlias: 'Алиас', adminPrimary: 'Основной', adminAdd: 'Добавить', adminUser: 'Пользователь', adminNoData: 'Нет данных', adminMailDisabled: 'Исходящая почта выключена, пока не задан домен отправки.', adminClear: 'Очистить ключ', llmTitle: 'ИИ-саммари (LLM)', dbTitle: 'База данных', llmHint: 'Подключите OpenAI-совместимый API (напр. llama.cpp). После включения появятся: кнопка «Саммари» на wiki-страницах и «✨ Сгенерировать» (описание + TODO) в форме задачи.', llmEnabledOn: 'Включено', llmBaseUrl: 'Базовый URL API', llmModel: 'Модель', llmApiKey: 'API-ключ', llmApiKeyOptional: 'необязательно', llmEnable: 'Включить (с проверкой подключения)', llmTest: 'Проверить подключение', llmTesting: 'Проверка…', llmOk: 'Подключение успешно', dbHost: 'Хост', dbPort: 'Порт', dbName: 'База', dbUser: 'Пользователь', dbPath: 'Файл', adminTabAudit: 'Журнал аудита', auditTime: 'Время', auditUser: 'Пользователь', auditAction: 'Действие', auditProject: 'Проект', auditDetails: 'Детали', auditAllProjects: 'Все проекты', adminTabSecurity: 'Безопасность', twofaTitle: 'Двухфакторная аутентификация', twofaHint: 'Добавьте одноразовый код (TOTP) из приложения-аутентификатора как второй шаг входа.', twofaOn: 'Двухфакторная аутентификация включена.', twofaEnable: 'Включить 2FA', twofaDisable: 'Отключить 2FA', twofaCode: 'Код', twofaScan: 'Добавьте этот секрет в приложение-аутентификатор (или отсканируйте otpauth-ссылку), затем введите текущий код:', twofaConfirm: 'Подтвердить', twofaEnabled: '2FA включена.', twofaRecovery: 'Сохраните эти одноразовые коды восстановления в надёжном месте:' },
  es: { adminTitle: 'Administración', adminTabUsers: 'Usuarios', adminTabBoard: 'Tablero', adminTabServices: 'Servicios', adminTabIntegrations: 'Integraciones', adminTabData: 'Datos', adminTabMailServer: 'Servidor de correo', adminTabMailboxes: 'Buzones' },
  fr: { adminTitle: 'Administration', adminTabUsers: 'Utilisateurs', adminTabBoard: 'Tableau', adminTabServices: 'Services', adminTabIntegrations: 'Intégrations', adminTabData: 'Données', adminTabMailServer: 'Serveur de messagerie', adminTabMailboxes: 'Boîtes' },
  de: { adminTitle: 'Verwaltung', adminTabUsers: 'Benutzer', adminTabBoard: 'Board', adminTabServices: 'Dienste', adminTabIntegrations: 'Integrationen', adminTabData: 'Daten', adminTabMailServer: 'Mailserver', adminTabMailboxes: 'Postfächer' },
  pt: { adminTitle: 'Administração', adminTabUsers: 'Usuários', adminTabBoard: 'Quadro', adminTabServices: 'Serviços', adminTabIntegrations: 'Integrações', adminTabData: 'Dados', adminTabMailServer: 'Servidor de e-mail', adminTabMailboxes: 'Caixas' },
};
let adminI18nMerged = false;
function ensureAdminI18n() {
  if (adminI18nMerged || typeof LANGS === 'undefined') return;
  for (const lang of ['ru', 'en', 'zh', 'es', 'fr', 'de', 'pt', 'ja', 'ko', 'ar']) {
    if (!LANGS[lang]) continue;
    const dict = Object.assign({}, ADMIN_I18N.en, ADMIN_I18N[lang] || {});
    for (const k in dict) if (!(k in LANGS[lang])) LANGS[lang][k] = dict[k];
  }
  adminI18nMerged = true;
}

// Tab definitions for the unified Settings page. `sections` are relocated sidebar
// section ids (ids preserved, so existing populate functions keep working); `render`
// panels are drawn by a function. `admin:true` = admins only; `show()` = conditional.
const ADMIN_TABS = [
  // User-facing first
  { key: 'account', i: 'adminTabAccount', render: renderAdminAccount },
  { key: 'general', i: 'adminTabGeneral', sections: ['theme-section', 'lang-section', 'fontsize-section', 'timezone-section'] },
  { key: 'mail', i: 'adminTabMail', render: renderAdminMail, show: () => typeof isServiceEnabled === 'function' && isServiceEnabled('mail') && currentUser && currentUser.role !== 'readonly' },
  // Admin-only service config (bot token, SSO, LLM, DB). Personal Telegram
  // linking lives in the Account tab instead.
  { key: 'integrations', i: 'adminTabIntegrations', sections: ['telegram-section', 'sso-section', 'llm-section', 'db-section'], admin: true },
  // Admin configuration
  { key: 'board', i: 'adminTabBoard', sections: ['columns-section', 'epics-section', 'sprints-section', 'tags-section'], admin: true },
  { key: 'users', i: 'adminTabUsers', sections: ['users-section'], admin: true },
  { key: 'services', i: 'adminTabServices', sections: ['services-section'], admin: true },
  { key: 'site', i: 'adminTabSite', render: renderAdminSite, admin: true },
  { key: 'mailserver', i: 'adminTabMailServer', render: renderAdminMailServer, admin: true },
  { key: 'auditlog', i: 'adminTabAudit', render: renderAdminAuditLog, admin: true },
  { key: 'data', i: 'adminTabData', sections: ['export-import-section', 'backup-section'], admin: true },
];

// renderAdminAccount renders per-user account tooling available to everyone:
// identity, change password, personal Telegram linking, and logout.
function renderAdminAccount(panel) {
  if (!panel || !currentUser) return;
  const tgConfigured = !!currentUser.telegram_configured || !!currentUser.telegram_bot_username;
  let tg = '';
  if (tgConfigured) {
    const bot = (currentUser.telegram_bot_username || '').replace(/^@/, '');
    tg = `<div class="admin-form" style="margin-top:20px">
      <h3 style="margin:0 0 8px">Telegram</h3>
      ${bot ? `<div class="admin-note">Bot: <a href="https://t.me/${esc(bot)}" target="_blank" rel="noopener noreferrer">@${esc(bot)}</a></div>` : ''}
      ${currentUser.telegram_id
        ? `<div style="color:var(--green);font-size:.85rem">&#10004; ${t('telegramLinked')}</div>
           <div class="admin-row"><button class="btn btn-ghost btn-sm" onclick="accUnlinkTelegram()">${t('unlinkTelegram')}</button></div>`
        : `<div class="admin-note">${t('telegramNotLinked')}</div>
           <div class="admin-row"><button class="btn btn-ghost btn-sm" onclick="generateTelegramHash()">${t('generateHash')}</button></div>
           <div id="tg-hash-display" style="margin-top:4px;font-size:.85rem"></div>`}
    </div>`;
  }
  panel.innerHTML = `<h2 class="admin-h">${t('adminTabAccount')}</h2>
    <div class="admin-form">
      <div class="admin-note">${esc(currentUser.username)} &middot; ${esc(t(currentUser.role) || currentUser.role)}</div>
      <div class="admin-row">
        <button class="btn btn-ghost btn-sm" onclick="navigateTo('/user/' + currentUser.id)">${t('accProfile')}</button>
        <button class="btn btn-ghost btn-sm" onclick="changeOwnPassword()">${t('changePassword')}</button>
      </div>
    </div>
    ${tg}
    <div id="acc-2fa" style="margin-top:20px"></div>
    <div class="admin-form" style="margin-top:20px">
      <div class="admin-row"><button class="btn btn-ghost btn-sm admin-danger" onclick="logout()">${t('logout')}</button></div>
    </div>`;
  renderAdmin2FA(panel.querySelector('#acc-2fa'));
}

async function accUnlinkTelegram() {
  try { await api('/api/user/telegram/unlink', 'POST'); currentUser.telegram_id = 0; }
  catch (e) { alert(t('error')); return; }
  renderAdminAccount(document.getElementById('admin-panel-account'));
}

// renderAdmin2FA renders the per-user two-factor (TOTP) settings.
async function renderAdmin2FA(panel) {
  if (!panel) return;
  panel.innerHTML = `<h3 style="margin:0 0 8px">${t('twofaTitle')}</h3><div id="twofa-body" class="admin-form">${t('adminNoData')}</div>`;
  let st = { enabled: false };
  try { st = await api('/api/2fa/status'); } catch (e) {}
  const body = panel.querySelector('#twofa-body');
  if (st.enabled) {
    body.innerHTML = `<p>${t('twofaOn')}</p>
      <label class="admin-fld">${t('twofaCode')}<input id="twofa-disable-code" autocomplete="one-time-code"></label>
      <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="twofaDisable()">${t('twofaDisable')}</button>`;
  } else {
    body.innerHTML = `<p>${t('twofaHint')}</p><button class="btn btn-primary btn-sm" onclick="twofaSetup()">${t('twofaEnable')}</button>`;
  }
}

async function twofaSetup() {
  const body = document.getElementById('twofa-body');
  let r;
  try { r = await api('/api/2fa/setup', 'POST', {}); } catch (e) { alert(e.message); return; }
  body.innerHTML = `<p>${t('twofaScan')}</p>
    <div class="twofa-secret">${esc(r.secret)}</div>
    <div style="font-size:.78rem;color:var(--fg3);word-break:break-all;margin:6px 0">${esc(r.uri)}</div>
    <label class="admin-fld">${t('twofaCode')}<input id="twofa-enable-code" autocomplete="one-time-code"></label>
    <button class="btn btn-primary btn-sm" onclick="twofaEnable()">${t('twofaConfirm')}</button>`;
}

async function twofaEnable() {
  const code = (document.getElementById('twofa-enable-code').value || '').trim();
  let r;
  try { r = await api('/api/2fa/enable', 'POST', { code }); } catch (e) { alert(e.message); return; }
  const body = document.getElementById('twofa-body');
  body.innerHTML = `<p style="color:var(--green)">&#10004; ${t('twofaEnabled')}</p>
    <p>${t('twofaRecovery')}</p>
    <pre class="twofa-codes">${(r.recovery_codes || []).map(esc).join('\n')}</pre>`;
}

async function twofaDisable() {
  const code = (document.getElementById('twofa-disable-code').value || '').trim();
  try { await api('/api/2fa/disable', 'POST', { code }); } catch (e) { alert(e.message); return; }
  renderAdmin2FA(document.getElementById('acc-2fa'));
}

// renderAdminAuditLog shows the activity log with user/project/action filters.
async function renderAdminAuditLog(panel) {
  const projs = (typeof projects !== 'undefined' && projects) ? projects : [];
  panel.innerHTML = `<h2 class="admin-h">${t('adminTabAudit')}</h2>
    <div class="audit-filters">
      <select id="audit-project"><option value="">${esc(t('auditAllProjects'))}</option>
        ${projs.map(p => `<option value="${p.id}">${esc(p.name)}</option>`).join('')}</select>
      <input id="audit-action" placeholder="${escAttr(t('auditAction'))}" style="width:140px">
      <button class="btn btn-ghost btn-sm" onclick="adminLoadAudit()">⟳</button>
    </div>
    <div id="audit-table" class="audit-table"></div>`;
  panel.querySelector('#audit-project').addEventListener('change', adminLoadAudit);
  panel.querySelector('#audit-action').addEventListener('keydown', e => { if (e.key === 'Enter') adminLoadAudit(); });
  adminLoadAudit();
}

async function adminLoadAudit() {
  const el = document.getElementById('audit-table');
  if (!el) return;
  const pj = document.getElementById('audit-project').value;
  const ac = document.getElementById('audit-action').value.trim();
  let q = '/api/admin/audit?limit=200';
  if (pj) q += '&project=' + encodeURIComponent(pj);
  if (ac) q += '&action=' + encodeURIComponent(ac);
  let rows = [];
  try { rows = await api(q); } catch (e) { el.innerHTML = `<div class="admin-empty">${esc(e.message)}</div>`; return; }
  if (!rows.length) { el.innerHTML = `<div class="admin-empty">${t('adminNoData')}</div>`; return; }
  el.innerHTML = `<table class="audit-tbl"><thead><tr>
      <th>${t('auditTime')}</th><th>${t('auditUser')}</th><th>${t('auditAction')}</th><th>${t('auditProject')}</th><th>${t('auditDetails')}</th>
    </tr></thead><tbody>${rows.map(r => `<tr>
      <td>${esc(fmtDate(r.created_at))}</td>
      <td>${esc(r.username || '#' + r.user_id)}</td>
      <td><code>${esc(r.action)}</code></td>
      <td>${esc(r.project_name || (r.project_id ? '#' + r.project_id : ''))}</td>
      <td>${esc((r.details || '').slice(0, 200))}</td>
    </tr>`).join('')}</tbody></table>`;
}

// adminTabVisible decides whether a tab is shown for the current user/role.
function adminTabVisible(td) {
  if (td.admin && !isAdmin()) return false;
  if (typeof td.show === 'function') return td.show();
  return true;
}
function adminVisibleTabs() { return ADMIN_TABS.filter(adminTabVisible); }

// === Mail server tab (settings + DKIM + security audit) ===
async function renderAdminMailServer(panel) {
  if (!panel) return;
  let s = {};
  try { s = await api('/api/mail/settings'); } catch (e) { /* ignore */ }
  panel.innerHTML = `
    <h2 class="admin-h">${t('adminTabMailServer')}</h2>
    <div class="admin-form">
      <label class="admin-fld">${t('adminMailDomain')}<input id="ms-domain" value="${escAttr(s.domain || '')}" placeholder="example.com"></label>
      <label class="admin-fld">${t('adminDkimSelector')}<input id="ms-selector" value="${escAttr(s.dkim_selector || 'default')}"></label>
      <label class="admin-fld">${t('adminDkimKey')} <span class="admin-hint">(${t('adminDkimKeyHint')})</span>
        <textarea id="ms-dkim" rows="4" placeholder="-----BEGIN RSA PRIVATE KEY-----"></textarea></label>
      <div class="admin-row">
        <button class="btn btn-primary btn-sm" onclick="adminSaveMailServer()">${t('adminSave')}</button>
        ${s.dkim_configured ? `<button class="btn btn-ghost btn-sm admin-danger" onclick="adminSaveMailServer('__clear__')">${t('adminClear')}</button>` : ''}
      </div>
      ${!s.outbound_enabled ? `<div class="admin-note">${t('adminMailDisabled')}</div>` : ''}
      ${s.dkim_txt ? `<div class="admin-fld">${t('adminDkimTxt')} <code class="admin-code">${esc(s.dkim_selector || 'default')}._domainkey TXT</code><textarea rows="3" readonly class="admin-code">${esc(s.dkim_txt)}</textarea></div>` : ''}
    </div>
    <h3 class="admin-h" style="font-size:1rem;margin-top:24px">${t('adminAudit')}</h3>
    <div id="ms-audit" class="admin-audit"></div>`;
  let audit = [];
  try { audit = await api('/api/mail/audit?limit=100'); } catch (e) { /* ignore */ }
  const ael = panel.querySelector('#ms-audit');
  if (ael) {
    ael.innerHTML = audit.length ? `<table class="admin-table"><thead><tr><th>time</th><th>event</th><th>source</th><th>from</th><th>subject</th><th>score</th></tr></thead><tbody>${audit.map(a => `<tr>
      <td>${esc(fmtDate(a.created_at))}</td><td><span class="admin-ev admin-ev-${esc(a.event)}">${esc(a.event)}</span></td>
      <td>${esc(a.source_ip || '')}</td><td>${esc(a.mail_from || '')}</td><td>${esc(a.subject || '')}</td><td>${a.score || ''}</td>
    </tr>`).join('')}</tbody></table>` : `<div class="admin-note">${t('adminNoData')}</div>`;
  }
}

async function adminSaveMailServer(clear) {
  const body = {
    domain: document.getElementById('ms-domain').value.trim(),
    dkim_selector: document.getElementById('ms-selector').value.trim(),
    dkim_key: clear === '__clear__' ? '__clear__' : document.getElementById('ms-dkim').value,
  };
  try { await api('/api/mail/settings', 'POST', body); renderAdminMailServer(document.getElementById('admin-panel-mailserver')); }
  catch (e) { alert(e.message); }
}

// === Mailboxes tab (aliases) ===
async function renderAdminMailboxes(panel) {
  if (!panel) return;
  let aliases = [];
  try { aliases = await api('/api/mail/aliases'); } catch (e) { /* ignore */ }
  const users = state.users || [];
  panel.innerHTML = `
    <h2 class="admin-h">${t('adminTabMailboxes')}</h2>
    <div class="admin-mb-add">
      <select id="mb-user">${users.map(u => `<option value="${u.id}">${esc(u.username)}</option>`).join('')}</select>
      <input id="mb-local" placeholder="${t('adminAlias')}">
      <label class="admin-inline"><input type="checkbox" id="mb-primary"> ${t('adminPrimary')}</label>
      <button class="btn btn-primary btn-sm" onclick="adminAddAlias()">${t('adminAdd')}</button>
    </div>
    <table class="admin-table"><thead><tr><th>${t('adminUser')}</th><th>${t('adminAlias')}</th><th>${t('adminPrimary')}</th><th></th></tr></thead>
    <tbody>${aliases.map(a => `<tr>
      <td>${esc(a.username)}</td><td>${esc(a.localpart)}</td>
      <td>${a.is_primary ? '★' : `<button class="btn btn-ghost btn-sm" onclick="adminSetPrimaryAlias(${a.id})">${t('adminPrimary')}</button>`}</td>
      <td><button class="btn-icon btn-sm admin-danger" onclick="adminDeleteAlias(${a.id})">&#10005;</button></td>
    </tr>`).join('') || `<tr><td colspan="4"><div class="admin-note">${t('adminNoData')}</div></td></tr>`}</tbody></table>`;
}

async function adminAddAlias() {
  const body = { user_id: Number(document.getElementById('mb-user').value), localpart: document.getElementById('mb-local').value.trim(), is_primary: document.getElementById('mb-primary').checked };
  if (!body.localpart) return;
  try { await api('/api/mail/aliases', 'POST', body); renderAdminMailboxes(document.getElementById('admin-panel-mailboxes')); }
  catch (e) { alert(e.message); }
}
async function adminDeleteAlias(id) {
  try { await api('/api/mail/aliases/' + id, 'DELETE'); renderAdminMailboxes(document.getElementById('admin-panel-mailboxes')); }
  catch (e) { alert(e.message); }
}
async function adminSetPrimaryAlias(id) {
  try { await api('/api/mail/aliases/' + id + '/primary', 'POST'); renderAdminMailboxes(document.getElementById('admin-panel-mailboxes')); }
  catch (e) { alert(e.message); }
}

let adminBuilt = false;
let adminTab = 'general';

function ensureAdminPage() {
  if (adminBuilt) return;
  ensureAdminI18n();
  const page = document.createElement('div');
  page.id = 'admin-page';
  page.innerHTML = `<div class="admin-tabs" id="admin-tabs"></div><div class="admin-body" id="admin-body"></div>`;
  document.body.appendChild(page);
  const body = page.querySelector('#admin-body');
  // Only build panels (and relocate their sections) for tabs the user can see.
  for (const td of adminVisibleTabs()) {
    const panel = document.createElement('div');
    panel.className = 'admin-panel';
    panel.id = 'admin-panel-' + td.key;
    for (const sid of (td.sections || [])) {
      const sec = document.getElementById(sid);
      if (sec) { sec.style.display = ''; panel.appendChild(sec); }
    }
    body.appendChild(panel);
  }
  adminBuilt = true;
}

function renderAdminTabs() {
  const bar = document.getElementById('admin-tabs');
  if (!bar) return;
  bar.innerHTML = adminVisibleTabs().map(td => `<button class="admin-tab ${adminTab === td.key ? 'active' : ''}" onclick="adminSetTab('${td.key}')">${esc(t(td.i))}</button>`).join('');
  document.querySelectorAll('#admin-page .admin-panel').forEach(p => p.classList.toggle('active', p.id === 'admin-panel-' + adminTab));
}

function adminSetTab(k) {
  adminTab = k;
  renderAdminTabs();
  const td = ADMIN_TABS.find(t => t.key === k);
  if (td && typeof td.render === 'function') td.render(document.getElementById('admin-panel-' + k));
}

function renderAdminView() {
  ensureAdminI18n();
  ensureAdminPage();
  const visible = adminVisibleTabs();
  if (!visible.some(td => td.key === adminTab)) adminTab = visible.length ? visible[0].key : 'general';
  renderAdminTabs();
  const td = ADMIN_TABS.find(t => t.key === adminTab);
  if (td && typeof td.render === 'function') td.render(document.getElementById('admin-panel-' + adminTab));
}

// Flag per supported interface language (keys mirror LANGS / server supportedLangs).
const LANG_FLAGS = { ru:'🇷🇺', en:'🇬🇧', zh:'🇨🇳', es:'🇪🇸', fr:'🇫🇷', de:'🇩🇪', pt:'🇵🇹', ja:'🇯🇵', ko:'🇰🇷', ar:'🇸🇦' };

// === Site tab (application name / branding) ===
async function renderAdminSite(panel) {
  if (!panel) return;
  let name = window.APP_NAME || '';
  let defLang = (typeof window !== 'undefined' && window.DEFAULT_LANG) || 'ru';
  try { const r = await api('/api/settings/app'); name = r.name || name; if (r.default_lang) defLang = r.default_lang; } catch (e) { /* ignore */ }
  const langOpts = Object.keys(LANG_FLAGS)
    .filter(k => typeof LANGS === 'undefined' || LANGS[k])
    .map(k => `<option value="${k}" ${k === defLang ? 'selected' : ''}>${LANG_FLAGS[k]} ${(typeof LANGS !== 'undefined' && LANGS[k] && LANGS[k].name) || k}</option>`)
    .join('');
  panel.innerHTML = `
    <h2 class="admin-h">${t('adminTabSite')}</h2>
    <div class="admin-form">
      <label class="admin-fld">${t('adminAppName')}
        <input id="site-appname" maxlength="60" value="${escAttr(name)}" placeholder="Madok.Works"></label>
      <label class="admin-fld">${t('adminDefaultLang')}
        <select id="site-deflang">${langOpts}</select></label>
      <div class="admin-row"><button class="btn btn-primary btn-sm" onclick="adminSaveAppName()">${t('adminSave')}</button></div>
      <div class="admin-note">${t('adminAppNameHint')}</div>
      <div class="admin-note">${t('adminDefaultLangHint')}</div>
    </div>
    <div class="admin-form" style="margin-top:24px">
      <h3 style="margin:0 0 8px">${t('obReplayTitle')}</h3>
      <div class="admin-row"><button class="btn btn-ghost btn-sm" onclick="adminReplayOnboarding()">${t('obReplay')}</button></div>
      <div class="admin-note">${t('obReplayHint')}</div>
    </div>`;
}

async function adminReplayOnboarding() {
  try { await api('/api/setup/onboarding', 'POST', { mode: 'reset' }); } catch (e) { alert(e.message); return; }
  location.reload();
}

async function adminSaveAppName() {
  const name = document.getElementById('site-appname').value.trim();
  const ls = document.getElementById('site-deflang');
  const lang = ls ? ls.value : '';
  try {
    const r = await api('/api/settings/app', 'POST', { name, lang });
    if (r.default_lang) window.DEFAULT_LANG = r.default_lang;
    if (typeof applyAppName === 'function') applyAppName(r.name || name || 'Madok.Works');
    if (typeof toast === 'function') toast(t('settingsSaved') || 'OK', 'success');
  } catch (e) { alert(e.message); }
}

// === Mail tab (per-user distribution rules + admin mailboxes/aliases) ===
function renderAdminMail(panel) {
  if (!panel) return;
  if (typeof ensureMailI18n === 'function') ensureMailI18n();
  let html = `<h2 class="admin-h">${t('adminTabMail')}</h2>
    <div class="admin-form">
      <div class="admin-row"><button class="btn btn-primary btn-sm" onclick="mailRulesDialog()">${t('mailRules')}</button></div>
    </div>`;
  if (isAdmin()) html += `<div id="admin-mb-block" style="margin-top:28px"></div>`;
  panel.innerHTML = html;
  if (isAdmin()) renderAdminMailboxes(panel.querySelector('#admin-mb-block'));
}
