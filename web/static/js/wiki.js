// Wiki service frontend. Loaded before the main inline script; relies on
// shared helpers defined there (api, t, esc, escAttr, navigateTo, fmtDate).

let wikiState = {
  spaces: [],
  spaceId: null,
  tree: [],
  page: null,
  mode: 'view', // view | edit | history | space-settings
  revisions: [],
  viewingRev: null,
  dirty: false,
};

const WIKI_COLLAPSE_KEY = 'kanban-wiki-collapsed';

// Editor-mode i18n merged into LANGS (other wiki strings already live there).
const WIKI_I18N = {
  en: { wikiModeRich: 'Rich', wikiModeMarkdown: 'Markdown', wikiPreview: 'Preview', tbLinkText: 'Text', tbInsert: 'Insert', tbCancel: 'Cancel', tbCalloutInfo: 'Info', tbCalloutWarn: 'Warning', tbCalloutNote: 'Note', tbCalloutSuccess: 'Success' },
  ru: { wikiModeRich: 'Визуально', wikiModeMarkdown: 'Markdown', wikiPreview: 'Предпросмотр', tbLinkText: 'Текст', tbInsert: 'Вставить', tbCancel: 'Отмена', tbCalloutInfo: 'Инфо', tbCalloutWarn: 'Внимание', tbCalloutNote: 'Заметка', tbCalloutSuccess: 'Успех' },
  es: { wikiModeRich: 'Visual', wikiModeMarkdown: 'Markdown', wikiPreview: 'Vista previa', tbLinkText: 'Texto', tbInsert: 'Insertar', tbCancel: 'Cancelar', tbCalloutInfo: 'Info', tbCalloutWarn: 'Aviso', tbCalloutNote: 'Nota', tbCalloutSuccess: 'Éxito' },
  fr: { wikiModeRich: 'Visuel', wikiModeMarkdown: 'Markdown', wikiPreview: 'Aperçu', tbLinkText: 'Texte', tbInsert: 'Insérer', tbCancel: 'Annuler', tbCalloutInfo: 'Info', tbCalloutWarn: 'Attention', tbCalloutNote: 'Note', tbCalloutSuccess: 'Succès' },
  de: { wikiModeRich: 'Visuell', wikiModeMarkdown: 'Markdown', wikiPreview: 'Vorschau', tbLinkText: 'Text', tbInsert: 'Einfügen', tbCancel: 'Abbrechen', tbCalloutInfo: 'Info', tbCalloutWarn: 'Warnung', tbCalloutNote: 'Notiz', tbCalloutSuccess: 'Erfolg' },
  pt: { wikiModeRich: 'Visual', wikiModeMarkdown: 'Markdown', wikiPreview: 'Pré-visualização', tbLinkText: 'Texto', tbInsert: 'Inserir', tbCancel: 'Cancelar', tbCalloutInfo: 'Info', tbCalloutWarn: 'Aviso', tbCalloutNote: 'Nota', tbCalloutSuccess: 'Sucesso' },
};
let wikiI18nMerged = false;
function ensureWikiI18n() {
  if (wikiI18nMerged || typeof LANGS === 'undefined') return;
  for (const lang of ['ru', 'en', 'zh', 'es', 'fr', 'de', 'pt', 'ja', 'ko', 'ar']) {
    if (!LANGS[lang]) continue;
    const dict = Object.assign({}, WIKI_I18N.en, WIKI_I18N[lang] || {});
    for (const k in dict) if (!(k in LANGS[lang])) LANGS[lang][k] = dict[k];
  }
  wikiI18nMerged = true;
}

function wikiCollapsed() {
  try { return JSON.parse(localStorage.getItem(WIKI_COLLAPSE_KEY) || '{}'); } catch (e) { return {}; }
}
function wikiToggleNode(id) {
  const c = wikiCollapsed();
  c[id] = !c[id];
  localStorage.setItem(WIKI_COLLAPSE_KEY, JSON.stringify(c));
  wikiRenderTree();
}

// === Entry point (called by the SPA router) ===
async function renderWikiView() {
  ensureWikiI18n();
  const m = currentServicePath.match(/^\/page\/(\d+)(\/edit|\/history)?$/);
  const sm = currentServicePath.match(/^\/space\/(\d+)$/);
  try {
    wikiState.spaces = await api('/api/wiki/spaces');
  } catch (e) {
    document.getElementById('app').innerHTML = `<div class="wiki-empty">${esc(e.message)}</div>`;
    return;
  }
  if (m) {
    const pageId = Number(m[1]);
    const sub = m[2] || '';
    try {
      const page = await api('/api/wiki/pages/' + pageId);
      wikiState.page = page;
      wikiState.spaceId = page.space_id;
      wikiState.mode = sub === '/edit' ? 'edit' : sub === '/history' ? 'history' : 'view';
    } catch (e) {
      wikiState.page = null;
      wikiState.mode = 'view';
    }
  } else if (sm) {
    wikiState.spaceId = Number(sm[1]);
    wikiState.page = null;
    wikiState.mode = 'view';
  } else {
    // root: restore last visited space if it still exists
    const last = Number(localStorage.getItem('kanban-wiki-space') || 0);
    wikiState.spaceId = wikiState.spaces.some(s => s.id === last) ? last :
      (wikiState.spaces.length ? wikiState.spaces[0].id : null);
    wikiState.page = null;
    wikiState.mode = 'view';
  }
  if (wikiState.spaceId) {
    localStorage.setItem('kanban-wiki-space', wikiState.spaceId);
    try { wikiState.tree = await api('/api/wiki/spaces/' + wikiState.spaceId + '/tree'); }
    catch (e) { wikiState.tree = []; }
  } else {
    wikiState.tree = [];
  }
  wikiRenderLayout();
}

function wikiSpace() { return wikiState.spaces.find(s => s.id === wikiState.spaceId) || null; }
function wikiCanEdit() {
  const sp = wikiSpace();
  return sp && (sp.my_role === 'editor' || sp.my_role === 'admin');
}
function wikiIsSpaceAdmin() {
  const sp = wikiSpace();
  return sp && sp.my_role === 'admin';
}

// === Layout ===
function wikiRenderLayout() {
  const sp = wikiSpace();
  document.getElementById('app').innerHTML = `
  <div class="wiki-layout">
    <aside class="wiki-side">
      <div class="wiki-space-bar">
        <select class="wiki-space-select" id="wiki-space-select" onchange="wikiSwitchSpace(this.value)">
          ${wikiState.spaces.map(s => `<option value="${s.id}" ${s.id === wikiState.spaceId ? 'selected' : ''}>${esc(s.name)}</option>`).join('')}
        </select>
        ${currentUser && currentUser.role !== 'readonly' ? `<button class="btn-icon" onclick="wikiNewSpaceDialog()" title="${t('wikiNewSpace')}">+</button>` : ''}
        ${wikiIsSpaceAdmin() ? `<button class="btn-icon" onclick="wikiOpenSpaceSettings()" title="${t('wikiSpaceSettings')}">&#9881;</button>` : ''}
      </div>
      <input class="wiki-search" id="wiki-search" placeholder="${t('wikiSearchPh')}" autocomplete="off">
      <div class="wiki-search-results hidden" id="wiki-search-results"></div>
      <div class="wiki-tree" id="wiki-tree"></div>
      ${wikiCanEdit() ? `<button class="btn btn-ghost btn-sm wiki-add-root" onclick="wikiNewPage(null)">+ ${t('wikiNewPage')}</button>` : ''}
    </aside>
    <main class="wiki-main" id="wiki-main"></main>
  </div>`;
  wikiRenderTree();
  wikiRenderMain();
  const se = document.getElementById('wiki-search');
  if (se) {
    let tmo = null;
    se.addEventListener('input', function () {
      clearTimeout(tmo);
      tmo = setTimeout(() => wikiDoSearch(this.value), 300);
    });
    se.addEventListener('keydown', function (e) { if (e.key === 'Escape') { this.value = ''; wikiDoSearch(''); } });
  }
}

async function wikiSwitchSpace(id) {
  wikiState.spaceId = Number(id);
  localStorage.setItem('kanban-wiki-space', wikiState.spaceId);
  navigateTo('/wiki/space/' + id);
}

// === Tree ===
function wikiRenderTree() {
  const el = document.getElementById('wiki-tree');
  if (!el) return;
  if (!wikiState.spaceId) {
    el.innerHTML = `<div class="wiki-empty">${t('wikiNoSpaces')}</div>`;
    return;
  }
  if (!wikiState.tree.length) {
    el.innerHTML = `<div class="wiki-empty">${t('wikiNoPages')}</div>`;
    return;
  }
  const byParent = {};
  for (const p of wikiState.tree) {
    const key = p.parent_id || 0;
    (byParent[key] = byParent[key] || []).push(p);
  }
  const collapsed = wikiCollapsed();
  const activeId = wikiState.page ? wikiState.page.id : null;
  function renderNodes(parentKey, depth) {
    const nodes = byParent[parentKey] || [];
    return nodes.map(n => {
      const kids = byParent[n.id] || [];
      const isCollapsed = collapsed[n.id];
      return `<div class="wiki-node" style="--depth:${depth}">
        <div class="wiki-node-row ${n.id === activeId ? 'active' : ''}">
          ${kids.length ? `<span class="wiki-expander" onclick="wikiToggleNode(${n.id})">${isCollapsed ? '&#9656;' : '&#9662;'}</span>` : '<span class="wiki-expander-spacer"></span>'}
          <a class="wiki-node-title" href="/wiki/page/${n.id}" onclick="navigateTo('/wiki/page/${n.id}');return false;">${esc(n.title)}</a>
          ${wikiCanEdit() ? `<button class="wiki-node-add" onclick="wikiNewPage(${n.id})" title="${t('wikiAddChild')}">+</button>` : ''}
        </div>
        ${kids.length && !isCollapsed ? renderNodes(n.id, depth + 1) : ''}
      </div>`;
    }).join('');
  }
  el.innerHTML = renderNodes(0, 0);
}

// === Main panel ===
function wikiRenderMain() {
  const el = document.getElementById('wiki-main');
  if (!el) return;
  if (!wikiState.spaceId) {
    el.innerHTML = `<div class="wiki-empty wiki-empty-lg">
      <div style="font-size:2rem;margin-bottom:10px">&#128214;</div>
      <p>${t('wikiNoSpaces')}</p>
      ${currentUser && currentUser.role !== 'readonly' ? `<button class="btn btn-primary" onclick="wikiNewSpaceDialog()">${t('wikiNewSpace')}</button>` : ''}
    </div>`;
    return;
  }
  if (wikiState.mode === 'space-settings') { wikiRenderSpaceSettings(el); return; }
  if (!wikiState.page) {
    const sp = wikiSpace();
    el.innerHTML = `<div class="wiki-empty wiki-empty-lg">
      <h2 style="margin-bottom:6px">${esc(sp ? sp.name : '')}</h2>
      <p style="color:var(--fg2);margin-bottom:14px">${esc(sp ? sp.description : '')}</p>
      ${wikiState.tree.length ? `<p>${t('wikiPickPage')}</p>` :
        (wikiCanEdit() ? `<button class="btn btn-primary" onclick="wikiNewPage(null)">+ ${t('wikiNewPage')}</button>` : `<p>${t('wikiNoPages')}</p>`)}
    </div>`;
    return;
  }
  if (wikiState.mode === 'edit') { wikiRenderEditor(el); return; }
  if (wikiState.mode === 'history') { wikiRenderHistory(el); return; }
  wikiRenderPageView(el);
}

function wikiBreadcrumbs() {
  const byId = {};
  for (const p of wikiState.tree) byId[p.id] = p;
  const chain = [];
  let cur = wikiState.page ? byId[wikiState.page.id] : null;
  // walk up, guard against cycles
  let guard = 0;
  while (cur && guard++ < 50) {
    chain.unshift(cur);
    cur = cur.parent_id ? byId[cur.parent_id] : null;
  }
  const sp = wikiSpace();
  let html = `<a onclick="navigateTo('/wiki/space/${wikiState.spaceId}')">${esc(sp ? sp.key : '')}</a>`;
  for (let i = 0; i < chain.length; i++) {
    const c = chain[i];
    html += ` <span class="wiki-bc-sep">/</span> `;
    html += i === chain.length - 1 ? `<span>${esc(c.title)}</span>` : `<a onclick="navigateTo('/wiki/page/${c.id}')">${esc(c.title)}</a>`;
  }
  return `<div class="wiki-breadcrumbs">${html}</div>`;
}

function wikiRenderPageView(el) {
  const p = wikiState.page;
  el.innerHTML = `
    ${wikiBreadcrumbs()}
    <div class="wiki-page-header">
      <h1 class="wiki-title">${esc(p.title)}</h1>
      <div class="wiki-actions">
        ${wikiCanEdit() ? `<button class="btn btn-primary btn-sm" onclick="navigateTo('/wiki/page/${p.id}/edit')">${t('wikiEdit')}</button>` : ''}
        <button class="btn btn-ghost btn-sm" onclick="navigateTo('/wiki/page/${p.id}/history')">${t('wikiHistory')}</button>
        ${wikiCanEdit() ? `<div class="wiki-menu-wrap">
          <button class="btn btn-ghost btn-sm" onclick="wikiToggleMenu(event)">&#8943;</button>
          <div class="wiki-menu hidden">
            <a onclick="wikiNewPage(${p.id})">${t('wikiAddChild')}</a>
            <a onclick="wikiMovePage(${p.id},-1)">&#9650; ${t('wikiMoveUp')}</a>
            <a onclick="wikiMovePage(${p.id},1)">&#9660; ${t('wikiMoveDown')}</a>
            <a onclick="wikiIndentPage(${p.id})">&#8618; ${t('wikiIndent')}</a>
            <a onclick="wikiOutdentPage(${p.id})">&#8617; ${t('wikiOutdent')}</a>
            <a onclick="wikiDeletePage(${p.id})" style="color:var(--red)">${t('wikiDeletePage')}</a>
          </div>
        </div>` : ''}
      </div>
    </div>
    <div class="wiki-meta">v${p.rev_no} &middot; ${t('wikiEditedBy')} ${esc(p.updated_by_name || '?')} &middot; ${fmtDate(p.updated_at)}</div>
    <div class="wiki-content md-content">${p.content || `<p style="color:var(--fg3)">${t('wikiEmptyPage')}</p>`}</div>`;
  highlightCode();
}

// === WYSIWYG editor ===
const WIKI_TOOLBAR = [
  { cmd: 'formatBlock', arg: 'p', label: '&para;', key: 'tbParagraph' },
  { cmd: 'formatBlock', arg: 'h1', label: 'H1', key: 'tbH1' },
  { cmd: 'formatBlock', arg: 'h2', label: 'H2', key: 'tbH2' },
  { cmd: 'formatBlock', arg: 'h3', label: 'H3', key: 'tbH3' },
  { sep: true },
  { cmd: 'bold', label: '<b>B</b>', key: 'tbBold' },
  { cmd: 'italic', label: '<i>I</i>', key: 'tbItalic' },
  { cmd: 'underline', label: '<u>U</u>', key: 'tbUnderline' },
  { cmd: 'strikeThrough', label: '<s>S</s>', key: 'tbStrike' },
  { custom: 'inlineCode', label: '&lt;/&gt;', key: 'tbCode' },
  { sep: true },
  { cmd: 'insertUnorderedList', label: '&#8226;&#8801;', key: 'tbUl' },
  { cmd: 'insertOrderedList', label: '1.&#8801;', key: 'tbOl' },
  { cmd: 'formatBlock', arg: 'blockquote', label: '&#10078;', key: 'tbQuote' },
  { custom: 'codeBlock', label: '{ }', key: 'tbCodeBlock' },
  { sep: true },
  { custom: 'link', label: '&#128279;', key: 'tbLink' },
  { custom: 'image', label: '&#128247;', key: 'tbImage' },
  { custom: 'table', label: '&#9638;', key: 'tbTable' },
  { custom: 'callout', label: '&#128161;', key: 'tbCallout' },
  { sep: true },
  { cmd: 'undo', label: '&#8630;', key: 'tbUndo' },
  { cmd: 'redo', label: '&#8631;', key: 'tbRedo' },
];

// wikiMdToHtml converts markdown to HTML via the bundled marked (falls back to
// escaped text). The server sanitizes the result on save.
function wikiMdToHtml(md) {
  if (typeof marked !== 'undefined' && marked.parse) return marked.parse(md || '');
  return '<p>' + escHtmlText(md || '') + '</p>';
}

// wikiSanitizePreview strips active content from rendered HTML before it goes into
// the live preview (defence-in-depth against self-XSS; stored content is also
// sanitized server-side on save).
function wikiSanitizePreview(html) {
  const doc = new DOMParser().parseFromString(html, 'text/html');
  doc.querySelectorAll('script,style,iframe,object,embed,link,meta,base,form').forEach(e => e.remove());
  doc.querySelectorAll('*').forEach(el => {
    Array.from(el.attributes).forEach(a => {
      const n = a.name.toLowerCase(), v = a.value.replace(/\s/g, '').toLowerCase();
      if (n.startsWith('on')) el.removeAttribute(a.name);
      else if ((n === 'href' || n === 'src' || n === 'xlink:href') && (v.startsWith('javascript:') || v.startsWith('data:text/html'))) el.removeAttribute(a.name);
    });
  });
  return doc.body.innerHTML;
}

// wikiHtmlToMd converts the rich editor's HTML into Markdown so switching to the
// Markdown view never loses content. Covers what the toolbar can produce.
function wikiHtmlToMd(root) {
  const inline = (node) => {
    let s = '';
    node.childNodes.forEach(n => {
      if (n.nodeType === 3) { s += n.textContent; return; }
      if (n.nodeType !== 1) return;
      const t = n.tagName.toLowerCase();
      const inner = inline(n);
      if (t === 'b' || t === 'strong') s += '**' + inner + '**';
      else if (t === 'i' || t === 'em') s += '*' + inner + '*';
      else if (t === 's' || t === 'del') s += '~~' + inner + '~~';
      else if (t === 'code') s += '`' + n.textContent + '`';
      else if (t === 'a') s += '[' + inner + '](' + (n.getAttribute('href') || '') + ')';
      else if (t === 'img') s += '![' + (n.getAttribute('alt') || '') + '](' + (n.getAttribute('src') || '') + ')';
      else if (t === 'br') s += '\n';
      else s += inner;
    });
    return s;
  };
  let out = '';
  root.childNodes.forEach(node => {
    if (node.nodeType === 3) { const tx = node.textContent.trim(); if (tx) out += tx + '\n\n'; return; }
    if (node.nodeType !== 1) return;
    const t = node.tagName.toLowerCase();
    if (/^h[1-6]$/.test(t)) out += '#'.repeat(+t[1]) + ' ' + inline(node).trim() + '\n\n';
    else if (t === 'p' || t === 'div') { const s = inline(node).trim(); if (s) out += s + '\n\n'; }
    else if (t === 'ul') { node.querySelectorAll(':scope > li').forEach(li => out += '- ' + inline(li).trim() + '\n'); out += '\n'; }
    else if (t === 'ol') { let i = 1; node.querySelectorAll(':scope > li').forEach(li => out += (i++) + '. ' + inline(li).trim() + '\n'); out += '\n'; }
    else if (t === 'blockquote') out += '> ' + inline(node).trim().replace(/\n/g, '\n> ') + '\n\n';
    else if (t === 'pre') out += '```\n' + node.textContent.replace(/\n+$/, '') + '\n```\n\n';
    else if (t === 'hr') out += '---\n\n';
    else { const s = inline(node).trim(); if (s) out += s + '\n\n'; }
  });
  return out.trim();
}

function wikiSetEditorMode(m) {
  if (m === wikiState.editorMode) return;
  // Regenerate the target representation from the one we're leaving (lossless).
  if (wikiState.editorMode === 'rich') {
    const ed = document.getElementById('wiki-editor');
    if (ed) { wikiState.page.content = ed.innerHTML; wikiState.page.content_md = wikiHtmlToMd(ed); }
  } else {
    const ta = document.getElementById('wiki-md');
    if (ta) { wikiState.page.content_md = ta.value; wikiState.page.content = wikiMdToHtml(ta.value); }
  }
  wikiState.editorMode = m;
  wikiRenderMain();
}

function wikiRenderEditor(el) {
  const p = wikiState.page;
  // Choose default mode once per page-edit: markdown if the page has md source.
  if (wikiState.editorPageId !== p.id) {
    wikiState.editorMode = p.content_md ? 'markdown' : 'rich';
    wikiState.editorPageId = p.id;
  }
  const mode = wikiState.editorMode;
  const body = mode === 'markdown'
    ? `<div class="wiki-md-split">
        <textarea class="wiki-md-input" id="wiki-md" placeholder="# Markdown…">${esc(p.content_md || '')}</textarea>
        <div class="wiki-md-preview md-content" id="wiki-md-preview"></div>
      </div>`
    : `<div class="wiki-toolbar" id="wiki-toolbar">
        ${WIKI_TOOLBAR.map(b => b.sep ? '<span class="wiki-tb-sep"></span>' :
          `<button type="button" class="wiki-tb-btn" title="${t(b.key)}" data-cmd="${b.cmd || ''}" data-arg="${b.arg || ''}" data-custom="${b.custom || ''}">${b.label}</button>`).join('')}
      </div>
      <div class="wiki-editor md-content" id="wiki-editor" contenteditable="true">${p.content}</div>`;
  el.innerHTML = `
    ${wikiBreadcrumbs()}
    <input class="wiki-title-input" id="wiki-title-input" value="${escAttr(p.title)}" maxlength="300">
    <div class="wiki-mode-tabs">
      <button class="wiki-mode-tab ${mode === 'rich' ? 'active' : ''}" onclick="wikiSetEditorMode('rich')">${t('wikiModeRich')}</button>
      <button class="wiki-mode-tab ${mode === 'markdown' ? 'active' : ''}" onclick="wikiSetEditorMode('markdown')">${t('wikiModeMarkdown')}</button>
    </div>
    ${body}
    <div class="wiki-edit-footer">
      <input class="wiki-comment-input" id="wiki-rev-comment" placeholder="${t('wikiComment')}" maxlength="200">
      <div style="display:flex;gap:8px">
        <button class="btn btn-ghost btn-sm" onclick="wikiCancelEdit()">${t('wikiCancel')}</button>
        <button class="btn btn-primary btn-sm" id="wiki-save-btn" onclick="wikiSavePage()">${t('wikiSave')}</button>
      </div>
    </div>`;
  wikiState.dirty = false;
  document.getElementById('wiki-title-input').addEventListener('input', () => { wikiState.dirty = true; });
  if (mode === 'markdown') {
    const ta = document.getElementById('wiki-md');
    const prev = document.getElementById('wiki-md-preview');
    const render = () => { prev.innerHTML = wikiSanitizePreview(wikiMdToHtml(ta.value)); if (typeof highlightCode === 'function') highlightCode(); };
    ta.addEventListener('input', () => { wikiState.dirty = true; render(); });
    render();
    ta.focus();
    return;
  }
  const editor = document.getElementById('wiki-editor');
  editor.addEventListener('input', () => { wikiState.dirty = true; });
  document.getElementById('wiki-toolbar').addEventListener('mousedown', e => {
    const btn = e.target.closest('.wiki-tb-btn');
    if (!btn) return;
    e.preventDefault();
    if (btn.dataset.custom) { wikiCustomCommand(btn.dataset.custom, btn); return; }
    document.execCommand(btn.dataset.cmd, false, btn.dataset.arg || null);
    editor.focus();
  });
  editor.addEventListener('paste', wikiHandlePaste);
  editor.addEventListener('click', e => { if (e.target.tagName === 'IMG') wikiImageResizePopover(e.target); });
  editor.addEventListener('keydown', e => {
    if (e.key === 'Tab') {
      e.preventDefault();
      document.execCommand(e.shiftKey ? 'outdent' : 'indent');
    }
  });
  editor.focus();
}

// === Toolbar popovers (dependency-free, replace window.prompt) ===
function wikiSaveRange() {
  const sel = window.getSelection();
  return sel.rangeCount ? sel.getRangeAt(0).cloneRange() : null;
}
function wikiRestoreRange(r) {
  if (!r) return;
  const sel = window.getSelection();
  sel.removeAllRanges();
  sel.addRange(r);
}
function wikiClosePopover() {
  const p = document.getElementById('wiki-popover');
  if (p) p.remove();
  document.removeEventListener('mousedown', wikiPopoverOutside, true);
  document.removeEventListener('keydown', wikiPopoverEsc, true);
}
function wikiPopoverOutside(e) { if (!e.target.closest('#wiki-popover')) wikiClosePopover(); }
function wikiPopoverEsc(e) { if (e.key === 'Escape') wikiClosePopover(); }
function wikiPopover(anchor, html) {
  wikiClosePopover();
  const pop = document.createElement('div');
  pop.className = 'wiki-popover';
  pop.id = 'wiki-popover';
  pop.innerHTML = html;
  document.body.appendChild(pop);
  const r = anchor.getBoundingClientRect();
  pop.style.top = (r.bottom + 4) + 'px';
  pop.style.left = Math.max(8, Math.min(r.left, window.innerWidth - pop.offsetWidth - 8)) + 'px';
  setTimeout(() => {
    document.addEventListener('mousedown', wikiPopoverOutside, true);
    document.addEventListener('keydown', wikiPopoverEsc, true);
  }, 0);
  return pop;
}

function wikiInsertTable(rows, cols) {
  rows = Math.min(Math.max(rows, 1), 20);
  cols = Math.min(Math.max(cols, 1), 10);
  let html = '<table><thead><tr>';
  for (let c = 0; c < cols; c++) html += '<th><br></th>';
  html += '</tr></thead><tbody>';
  for (let r = 1; r < rows; r++) {
    html += '<tr>';
    for (let c = 0; c < cols; c++) html += '<td><br></td>';
    html += '</tr>';
  }
  html += '</tbody></table><p><br></p>';
  document.execCommand('insertHTML', false, html);
  wikiState.dirty = true;
}

function wikiImageResizePopover(img) {
  // Sanitizer keeps only a numeric width attribute, so resize uses pixel widths.
  const pop = wikiPopover(img, `
    <div class="wiki-pop-row">${[200, 400, 600].map(p => `<button class="btn btn-ghost btn-sm" data-px="${p}">${p}px</button>`).join('')}</div>
    <div class="wiki-pop-row"><input id="wiki-img-px" type="number" min="16" max="2000" placeholder="px"><button class="btn btn-primary btn-sm" id="wiki-img-set">OK</button></div>
    <div class="wiki-pop-row"><button class="btn btn-ghost btn-sm" id="wiki-img-reset">100%</button></div>`);
  pop.querySelectorAll('[data-px]').forEach(b => b.onclick = () => { img.setAttribute('width', b.dataset.px); wikiState.dirty = true; wikiClosePopover(); });
  pop.querySelector('#wiki-img-set').onclick = () => { const v = parseInt(pop.querySelector('#wiki-img-px').value, 10); if (v > 0) { img.setAttribute('width', String(v)); wikiState.dirty = true; } wikiClosePopover(); };
  pop.querySelector('#wiki-img-reset').onclick = () => { img.removeAttribute('width'); wikiState.dirty = true; wikiClosePopover(); };
}

function wikiCustomCommand(name, anchor) {
  const editor = document.getElementById('wiki-editor');
  switch (name) {
    case 'inlineCode': {
      const sel = window.getSelection();
      if (!sel.rangeCount) return;
      const text = sel.toString();
      if (!text) return;
      document.execCommand('insertHTML', false, '<code>' + escHtmlText(text) + '</code>');
      break;
    }
    case 'codeBlock':
      document.execCommand('insertHTML', false, '<pre><code>' + escHtmlText(window.getSelection().toString() || ' ') + '</code></pre><p><br></p>');
      break;
    case 'image': {
      const inp = document.createElement('input');
      inp.type = 'file';
      inp.accept = 'image/png,image/jpeg,image/gif,image/webp';
      inp.onchange = async () => {
        const f = inp.files[0];
        if (!f) return;
        await wikiUploadImage(f);
      };
      inp.click();
      break;
    }
    case 'link': {
      const saved = wikiSaveRange();
      const selText = window.getSelection().toString();
      const pop = wikiPopover(anchor, `
        <div class="wiki-pop-row"><input id="wiki-pop-url" placeholder="https://" value="https://"></div>
        <div class="wiki-pop-row"><input id="wiki-pop-text" placeholder="${t('tbLinkText')}" value="${escAttr(selText)}"></div>
        <div class="wiki-pop-btns"><button class="btn btn-ghost btn-sm" id="wiki-pop-cancel">${t('tbCancel')}</button><button class="btn btn-primary btn-sm" id="wiki-pop-ok">${t('tbInsert')}</button></div>`);
      pop.querySelector('#wiki-pop-url').focus();
      pop.querySelector('#wiki-pop-cancel').onclick = wikiClosePopover;
      pop.querySelector('#wiki-pop-ok').onclick = () => {
        const url = pop.querySelector('#wiki-pop-url').value.trim();
        const txt = pop.querySelector('#wiki-pop-text').value;
        wikiClosePopover();
        if (!url) return;
        editor.focus(); wikiRestoreRange(saved);
        if (selText && !txt) document.execCommand('createLink', false, url);
        else document.execCommand('insertHTML', false, `<a href="${escAttr(url)}">${esc(txt || url)}</a>`);
        wikiState.dirty = true;
      };
      return;
    }
    case 'table': {
      const saved = wikiSaveRange();
      const N = 8;
      let cells = '';
      for (let r = 1; r <= N; r++) for (let c = 1; c <= N; c++) cells += `<span class="wiki-grid-cell" data-r="${r}" data-c="${c}"></span>`;
      const pop = wikiPopover(anchor, `<div class="wiki-grid-label" id="wiki-grid-label">0×0</div><div class="wiki-grid">${cells}</div>`);
      const label = pop.querySelector('#wiki-grid-label');
      pop.querySelectorAll('.wiki-grid-cell').forEach(cell => {
        cell.addEventListener('mouseenter', () => {
          const R = +cell.dataset.r, C = +cell.dataset.c;
          label.textContent = R + '×' + C;
          pop.querySelectorAll('.wiki-grid-cell').forEach(x => x.classList.toggle('on', +x.dataset.r <= R && +x.dataset.c <= C));
        });
        cell.addEventListener('click', () => {
          const rows = +cell.dataset.r, cols = +cell.dataset.c;
          wikiClosePopover(); editor.focus(); wikiRestoreRange(saved); wikiInsertTable(rows, cols);
        });
      });
      return;
    }
    case 'callout': {
      const saved = wikiSaveRange();
      const selText = window.getSelection().toString();
      const kinds = [['info', 'tbCalloutInfo'], ['warn', 'tbCalloutWarn'], ['note', 'tbCalloutNote'], ['success', 'tbCalloutSuccess']];
      const pop = wikiPopover(anchor, `<div class="wiki-callout-list">${kinds.map(([k, lab]) => `<button class="wiki-callout-opt wiki-${k}" data-k="${k}">${t(lab)}</button>`).join('')}</div>`);
      pop.querySelectorAll('.wiki-callout-opt').forEach(b => b.onclick = () => {
        const k = b.dataset.k;
        wikiClosePopover(); editor.focus(); wikiRestoreRange(saved);
        document.execCommand('insertHTML', false, `<div class="wiki-${k}"><p>${escHtmlText(selText || '…')}</p></div><p><br></p>`);
        wikiState.dirty = true;
      });
      return;
    }
  }
  editor.focus();
  wikiState.dirty = true;
}

function escHtmlText(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

async function wikiUploadImage(file) {
  try {
    const upload = typeof compressImage === 'function' && file.type !== 'image/gif' ? await compressImage(file) : file;
    const reader = new FileReader();
    reader.onload = async () => {
      const base64 = reader.result.split(',')[1];
      const res = await api('/api/files', 'POST', { data: base64, filename: file.name, mime: upload.type || file.type });
      document.execCommand('insertHTML', false, `<img src="${escAttr(res.url)}" alt="${escAttr(file.name)}">`);
      wikiState.dirty = true;
    };
    reader.readAsDataURL(upload);
  } catch (e) { alert(t('uploadError') + ': ' + e.message); }
}

function wikiHandlePaste(e) {
  const items = (e.clipboardData || {}).items || [];
  for (const it of items) {
    if (it.kind === 'file' && it.type.startsWith('image/')) {
      e.preventDefault();
      wikiUploadImage(it.getAsFile());
      return;
    }
  }
}

async function wikiSavePage() {
  const p = wikiState.page;
  const btn = document.getElementById('wiki-save-btn');
  const title = document.getElementById('wiki-title-input').value.trim();
  let content, contentMd = '';
  if (wikiState.editorMode === 'markdown') {
    contentMd = document.getElementById('wiki-md').value;
    content = wikiMdToHtml(contentMd);
  } else {
    content = document.getElementById('wiki-editor').innerHTML;
  }
  const comment = document.getElementById('wiki-rev-comment').value.trim();
  if (!title) { alert(t('wikiTitleRequired')); return; }
  btn.disabled = true;
  try {
    await api('/api/wiki/pages/' + p.id, 'PUT', { title, content, content_md: contentMd, comment, base_rev: p.rev_no });
    wikiState.dirty = false;
    navigateTo('/wiki/page/' + p.id);
  } catch (e) {
    btn.disabled = false;
    if (String(e.message).includes('conflict')) {
      alert(t('wikiConflict'));
    } else {
      alert(e.message);
    }
  }
}

function wikiCancelEdit() {
  if (wikiState.dirty && !confirm(t('wikiUnsaved'))) return;
  wikiState.dirty = false;
  navigateTo('/wiki/page/' + wikiState.page.id);
}

// === Page operations ===
async function wikiNewPage(parentId) {
  const title = prompt(t('wikiNewPageTitle'));
  if (!title || !title.trim()) return;
  try {
    const res = await api('/api/wiki/pages', 'POST', {
      space_id: wikiState.spaceId, parent_id: parentId, title: title.trim(), content: '',
    });
    navigateTo('/wiki/page/' + res.id + '/edit');
  } catch (e) { alert(e.message); }
}

async function wikiDeletePage(id) {
  if (!confirm(t('wikiDeleteConfirm'))) return;
  try {
    await api('/api/wiki/pages/' + id, 'DELETE');
    navigateTo('/wiki/space/' + wikiState.spaceId);
  } catch (e) { alert(e.message); }
}

function wikiSiblings(page) {
  return wikiState.tree
    .filter(p => (p.parent_id || 0) === (page.parent_id || 0))
    .sort((a, b) => a.position - b.position || a.id - b.id);
}

async function wikiMovePage(id, dir) {
  const page = wikiState.tree.find(p => p.id === id);
  if (!page) return;
  const sib = wikiSiblings(page);
  const idx = sib.findIndex(p => p.id === id);
  const target = idx + dir;
  if (target < 0 || target >= sib.length) return;
  try {
    await api('/api/wiki/pages/' + id + '/move', 'POST', { parent_id: page.parent_id, position: sib[target].position + (dir > 0 ? 1 : 0) });
    renderWikiView();
  } catch (e) { alert(e.message); }
}

async function wikiIndentPage(id) {
  const page = wikiState.tree.find(p => p.id === id);
  if (!page) return;
  const sib = wikiSiblings(page);
  const idx = sib.findIndex(p => p.id === id);
  if (idx <= 0) { alert(t('wikiCantIndent')); return; }
  try {
    await api('/api/wiki/pages/' + id + '/move', 'POST', { parent_id: sib[idx - 1].id, position: 999999 });
    renderWikiView();
  } catch (e) { alert(e.message); }
}

async function wikiOutdentPage(id) {
  const page = wikiState.tree.find(p => p.id === id);
  if (!page || !page.parent_id) return;
  const parent = wikiState.tree.find(p => p.id === page.parent_id);
  try {
    await api('/api/wiki/pages/' + id + '/move', 'POST', { parent_id: parent ? parent.parent_id : null, position: (parent ? parent.position : 0) + 1 });
    renderWikiView();
  } catch (e) { alert(e.message); }
}

function wikiToggleMenu(e) {
  e.stopPropagation();
  const menu = e.target.closest('.wiki-menu-wrap').querySelector('.wiki-menu');
  menu.classList.toggle('hidden');
  const close = () => { menu.classList.add('hidden'); document.removeEventListener('click', close); };
  setTimeout(() => document.addEventListener('click', close), 0);
}

// === History ===
async function wikiRenderHistory(el) {
  const p = wikiState.page;
  let revs = [];
  try { revs = await api('/api/wiki/pages/' + p.id + '/revisions'); } catch (e) { /* ignore */ }
  wikiState.revisions = revs;
  el.innerHTML = `
    ${wikiBreadcrumbs()}
    <div class="wiki-page-header">
      <h1 class="wiki-title">${esc(p.title)} — ${t('wikiHistory')}</h1>
      <div class="wiki-actions"><button class="btn btn-ghost btn-sm" onclick="navigateTo('/wiki/page/${p.id}')">&larr; ${t('wikiBack')}</button></div>
    </div>
    <div class="wiki-history">
      <div class="wiki-rev-list">
        ${revs.map(r => `<div class="wiki-rev-item ${wikiState.viewingRev === r.id ? 'active' : ''}" onclick="wikiViewRevision(${r.id})">
          <div><b>v${r.rev_no}</b> ${r.rev_no === p.rev_no ? `<span class="wiki-rev-current">${t('wikiCurrent')}</span>` : ''}</div>
          <div class="wiki-rev-meta">${esc(r.author_name || '?')} &middot; ${fmtDate(r.created_at)}</div>
          ${r.comment ? `<div class="wiki-rev-comment">${esc(r.comment)}</div>` : ''}
        </div>`).join('')}
      </div>
      <div class="wiki-rev-view" id="wiki-rev-view"><div class="wiki-empty">${t('wikiPickRevision')}</div></div>
    </div>`;
}

async function wikiViewRevision(revId) {
  wikiState.viewingRev = revId;
  document.querySelectorAll('.wiki-rev-item').forEach(el => el.classList.remove('active'));
  try {
    const rev = await api('/api/wiki/revisions/' + revId);
    const isCurrent = rev.rev_no === wikiState.page.rev_no;
    document.getElementById('wiki-rev-view').innerHTML = `
      <div class="wiki-rev-view-head">
        <b>v${rev.rev_no}</b> &middot; ${esc(rev.author_name || '?')} &middot; ${fmtDate(rev.created_at)}
        ${!isCurrent && wikiCanEdit() ? `<button class="btn btn-primary btn-sm" onclick="wikiRestoreRevision(${rev.id})">${t('wikiRestore')}</button>` : ''}
      </div>
      <div class="wiki-content md-content">${rev.content}</div>`;
    highlightCode();
  } catch (e) { alert(e.message); }
}

async function wikiRestoreRevision(revId) {
  if (!confirm(t('wikiRestoreConfirm'))) return;
  try {
    await api('/api/wiki/pages/' + wikiState.page.id + '/restore', 'POST', { revision_id: revId });
    navigateTo('/wiki/page/' + wikiState.page.id);
  } catch (e) { alert(e.message); }
}

// === Search ===
async function wikiDoSearch(q) {
  const box = document.getElementById('wiki-search-results');
  if (!box) return;
  q = q.trim();
  if (!q) { box.classList.add('hidden'); box.innerHTML = ''; return; }
  try {
    const hits = await api('/api/wiki/search?q=' + encodeURIComponent(q));
    box.classList.remove('hidden');
    box.innerHTML = hits.length ? hits.map(h => `<div class="wiki-hit" onclick="navigateTo('/wiki/page/${h.page_id}')">
      <div class="wiki-hit-title">${esc(h.space_key)} / ${esc(h.title)}</div>
      <div class="wiki-hit-snippet">${esc(h.snippet)}</div>
    </div>`).join('') : `<div class="wiki-empty">${t('wikiNoResults')}</div>`;
  } catch (e) { box.classList.add('hidden'); }
}

// === Spaces: create & settings ===
async function wikiNewSpaceDialog() {
  const name = prompt(t('wikiSpaceNamePrompt'));
  if (!name || !name.trim()) return;
  let key = prompt(t('wikiSpaceKeyPrompt'), wikiSuggestKey(name));
  if (!key) return;
  key = key.trim().toUpperCase();
  try {
    const res = await api('/api/wiki/spaces', 'POST', { key, name: name.trim(), description: '', visibility: 'open' });
    wikiState.spaces = await api('/api/wiki/spaces');
    navigateTo('/wiki/space/' + res.id);
  } catch (e) { alert(e.message); }
}

function wikiSuggestKey(name) {
  const translit = { 'а':'A','б':'B','в':'V','г':'G','д':'D','е':'E','ж':'ZH','з':'Z','и':'I','й':'Y','к':'K','л':'L','м':'M','н':'N','о':'O','п':'P','р':'R','с':'S','т':'T','у':'U','ф':'F','х':'H','ц':'C','ч':'CH','ш':'SH','щ':'SCH','ъ':'','ы':'Y','ь':'','э':'E','ю':'YU','я':'YA' };
  let out = '';
  for (const ch of name.toLowerCase()) {
    if (/[a-z0-9]/.test(ch)) out += ch.toUpperCase();
    else if (translit[ch] !== undefined) out += translit[ch];
    if (out.length >= 8) break;
  }
  return out || 'SPACE';
}

function wikiOpenSpaceSettings() {
  wikiState.mode = 'space-settings';
  wikiRenderMain();
}

async function wikiRenderSpaceSettings(el) {
  const sp = wikiSpace();
  let members = [];
  try { members = await api('/api/wiki/spaces/' + sp.id + '/members'); } catch (e) { /* ignore */ }
  const allUsers = (state.users || []).filter(u => !members.some(m => m.user_id === u.id));
  el.innerHTML = `
    <div class="wiki-page-header">
      <h1 class="wiki-title">${esc(sp.name)} — ${t('wikiSpaceSettings')}</h1>
      <div class="wiki-actions"><button class="btn btn-ghost btn-sm" onclick="navigateTo('/wiki/space/${sp.id}')">&larr; ${t('wikiBack')}</button></div>
    </div>
    <div class="wiki-settings">
      <label>${t('wikiKey')}: <b>${esc(sp.key)}</b></label>
      <label>${t('wikiName')}<input id="wiki-sp-name" value="${escAttr(sp.name)}" maxlength="300"></label>
      <label>${t('wikiDescription')}<input id="wiki-sp-desc" value="${escAttr(sp.description)}" maxlength="500"></label>
      <label>${t('wikiVisibility')}
        <select id="wiki-sp-vis">
          <option value="open" ${sp.visibility === 'open' ? 'selected' : ''}>${t('wikiOpen')}</option>
          <option value="restricted" ${sp.visibility === 'restricted' ? 'selected' : ''}>${t('wikiRestricted')}</option>
        </select>
      </label>
      <div style="display:flex;gap:8px;margin:10px 0 20px">
        <button class="btn btn-primary btn-sm" onclick="wikiSaveSpaceSettings()">${t('wikiSave')}</button>
        <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="wikiDeleteSpace()">${t('wikiDeleteSpace')}</button>
      </div>
      <h3>${t('wikiMembers')}</h3>
      <div class="entity-list">
        ${members.map(m => `<div class="entity-item">
          <span>${esc(m.username)}</span>
          <select onchange="wikiSetMember(${m.user_id}, this.value)">
            ${['viewer', 'editor', 'admin'].map(r => `<option value="${r}" ${m.role === r ? 'selected' : ''}>${t('wikiRole_' + r)}</option>`).join('')}
          </select>
          <button class="btn-icon btn-sm" style="color:var(--red)" onclick="wikiRemoveMember(${m.user_id})">&#10005;</button>
        </div>`).join('') || `<div class="wiki-empty">${t('wikiNoMembers')}</div>`}
      </div>
      ${allUsers.length ? `<div class="entity-add" style="margin-top:8px">
        <select id="wiki-new-member">${allUsers.map(u => `<option value="${u.id}">${esc(u.username)}</option>`).join('')}</select>
        <select id="wiki-new-member-role">
          <option value="viewer">${t('wikiRole_viewer')}</option>
          <option value="editor" selected>${t('wikiRole_editor')}</option>
          <option value="admin">${t('wikiRole_admin')}</option>
        </select>
        <button class="btn btn-primary btn-sm" onclick="wikiAddMember()">+</button>
      </div>` : ''}
    </div>`;
}

async function wikiSaveSpaceSettings() {
  const sp = wikiSpace();
  try {
    await api('/api/wiki/spaces/' + sp.id, 'PUT', {
      name: document.getElementById('wiki-sp-name').value.trim(),
      description: document.getElementById('wiki-sp-desc').value.trim(),
      visibility: document.getElementById('wiki-sp-vis').value,
    });
    wikiState.spaces = await api('/api/wiki/spaces');
    navigateTo('/wiki/space/' + sp.id);
  } catch (e) { alert(e.message); }
}

async function wikiDeleteSpace() {
  const sp = wikiSpace();
  if (!confirm(t('wikiDeleteSpaceConfirm').replace('{n}', sp.page_count))) return;
  try {
    await api('/api/wiki/spaces/' + sp.id, 'DELETE');
    localStorage.removeItem('kanban-wiki-space');
    navigateTo('/wiki');
  } catch (e) { alert(e.message); }
}

async function wikiAddMember() {
  const sp = wikiSpace();
  const uid = Number(document.getElementById('wiki-new-member').value);
  const role = document.getElementById('wiki-new-member-role').value;
  try {
    await api('/api/wiki/spaces/' + sp.id + '/members', 'POST', { user_id: uid, role });
    wikiRenderMain();
  } catch (e) { alert(e.message); }
}

async function wikiSetMember(uid, role) {
  const sp = wikiSpace();
  try { await api('/api/wiki/spaces/' + sp.id + '/members', 'POST', { user_id: uid, role }); }
  catch (e) { alert(e.message); }
}

async function wikiRemoveMember(uid) {
  const sp = wikiSpace();
  try {
    await api('/api/wiki/spaces/' + sp.id + '/members?user_id=' + uid, 'DELETE');
    wikiRenderMain();
  } catch (e) { alert(e.message); }
}

// warn about unsaved edits when leaving the tab
window.addEventListener('beforeunload', e => {
  if (wikiState.dirty) { e.preventDefault(); e.returnValue = ''; }
});
