// Admin page. Reached from the bottom app-rail gear (admins only) at /admin.
// Renders a tabbed page in a persistent top-level #admin-page element and relocates
// the existing admin-only sidebar sections into its panels (ids preserved, so all
// existing populate functions keep working). Phase 7 adds Mail server / Mailboxes.

const ADMIN_I18N = {
  en: { adminTitle: 'Administration', adminTabUsers: 'Users', adminTabBoard: 'Board', adminTabServices: 'Services', adminTabIntegrations: 'Integrations', adminTabData: 'Data', adminTabMailServer: 'Mail server', adminTabMailboxes: 'Mailboxes' },
  ru: { adminTitle: 'Администрирование', adminTabUsers: 'Пользователи', adminTabBoard: 'Доска', adminTabServices: 'Сервисы', adminTabIntegrations: 'Интеграции', adminTabData: 'Данные', adminTabMailServer: 'Почтовый сервер', adminTabMailboxes: 'Ящики' },
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
    const dict = ADMIN_I18N[lang] || ADMIN_I18N.en;
    for (const k in dict) if (!(k in LANGS[lang])) LANGS[lang][k] = dict[k];
  }
  adminI18nMerged = true;
}

// Tab definitions. `sections` are relocated sidebar section ids; `custom` panels
// are rendered by a function (used from Phase 7 onward).
const ADMIN_TABS = [
  { key: 'users', i: 'adminTabUsers', sections: ['users-section'] },
  { key: 'board', i: 'adminTabBoard', sections: ['columns-section'] },
  { key: 'services', i: 'adminTabServices', sections: ['services-section'] },
  { key: 'integrations', i: 'adminTabIntegrations', sections: ['telegram-section', 'sso-section'] },
  { key: 'data', i: 'adminTabData', sections: ['export-import-section', 'backup-section'] },
];

let adminBuilt = false;
let adminTab = 'users';

function ensureAdminPage() {
  if (adminBuilt) return;
  ensureAdminI18n();
  const page = document.createElement('div');
  page.id = 'admin-page';
  page.innerHTML = `<div class="admin-tabs" id="admin-tabs"></div><div class="admin-body" id="admin-body"></div>`;
  document.body.appendChild(page);
  const body = page.querySelector('#admin-body');
  for (const td of ADMIN_TABS) {
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
  bar.innerHTML = ADMIN_TABS.map(td => `<button class="admin-tab ${adminTab === td.key ? 'active' : ''}" onclick="adminSetTab('${td.key}')">${esc(t(td.i))}</button>`).join('');
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
  if (!ADMIN_TABS.some(td => td.key === adminTab)) adminTab = ADMIN_TABS[0].key;
  renderAdminTabs();
  const td = ADMIN_TABS.find(t => t.key === adminTab);
  if (td && typeof td.render === 'function') td.render(document.getElementById('admin-panel-' + adminTab));
}
