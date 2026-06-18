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
  wikiRenderHeader();
  document.getElementById('app').innerHTML = `
  <div class="wiki-layout">
    <aside class="wiki-side">
      <div class="wiki-tree" id="wiki-tree"></div>
      ${wikiCanEdit() ? `<button class="btn btn-ghost btn-sm wiki-add-root" onclick="wikiNewPage(null)">+ ${t('wikiNewPage')}</button>` : ''}
    </aside>
    <main class="wiki-main" id="wiki-main"></main>
  </div>`;
  wikiRenderTree();
  wikiRenderMain();
}

// wikiRenderHeader injects the space selector (kanban-style header dropdown) plus
// new-space / space-settings actions into #wiki-hdr (shown only in wiki view).
function wikiRenderHeader() {
  const el = document.getElementById('wiki-hdr');
  if (!el) return;
  const sp = wikiSpace();
  const canWrite = currentUser && currentUser.role !== 'readonly';
  const items = wikiState.spaces.map(s =>
    `<div class="sfd-item ${s.id === wikiState.spaceId ? 'active' : ''}" onclick="wikiSwitchSpace(${s.id})">${esc(s.name)}</div>`).join('')
    || `<div class="sfd-item" style="cursor:default">${esc(t('wikiNoSpaces'))}</div>`;
  const newSpace = canWrite ? `<button class="btn-icon wiki-hdr-btn" onclick="wikiNewSpaceDialog()" title="${escAttr(t('wikiNewSpace'))}">+</button>` : '';
  const settings = wikiIsSpaceAdmin() ? `<button class="btn-icon wiki-hdr-btn" onclick="wikiOpenSpaceSettings()" title="${escAttr(t('wikiSpaceSettings'))}">&#9881;</button>` : '';
  el.innerHTML = `<span class="hdr-sep">/</span>
    <span class="hdr-filter">
      <span class="hdr-filter-name" id="wiki-space-label" onclick="hdrDropToggle(event,'wiki-space-dd')">${esc(sp ? sp.name : t('wikiNoSpaces'))}</span>
      <div class="hdr-filter-dropdown hidden" id="wiki-space-dd">${items}</div>
    </span>${newSpace}${settings}`;
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
        ${wikiState.llmEnabled ? `<button class="btn btn-ghost btn-sm" onclick="wikiSummarize(${p.id})">&#10024; ${t('wikiSummary')}</button>` : ''}
        <button class="btn btn-ghost btn-sm" onclick="navigateTo('/wiki/page/${p.id}/history')">${t('wikiHistory')}</button>
        <button class="btn btn-ghost btn-sm" onclick="window.open('/api/wiki/pages/${p.id}/export','_blank')" title="${escAttr(t('wikiExportMd'))}">&#11015;</button>
        <button class="btn btn-ghost btn-sm" onclick="window.print()" title="${escAttr(t('wikiPrint'))}">&#128424;</button>
        ${wikiCanEdit() ? `<div class="wiki-menu-wrap">
          <button class="btn btn-ghost btn-sm" onclick="wikiToggleMenu(event)">&#8943;</button>
          <div class="wiki-menu hidden">
            <a onclick="wikiNewPage(${p.id})">${t('wikiAddChild')}</a>
            <a onclick="wikiMovePage(${p.id},-1)">&#9650; ${t('wikiMoveUp')}</a>
            <a onclick="wikiMovePage(${p.id},1)">&#9660; ${t('wikiMoveDown')}</a>
            <a onclick="wikiIndentPage(${p.id})">&#8618; ${t('wikiIndent')}</a>
            <a onclick="wikiOutdentPage(${p.id})">&#8617; ${t('wikiOutdent')}</a>
            <a onclick="wikiToggleTemplate(${p.id})">&#128209; ${p.is_template ? t('wikiUnmarkTemplate') : t('wikiMarkTemplate')}</a>
            <a onclick="wikiDeletePage(${p.id})" style="color:var(--red)">${t('wikiDeletePage')}</a>
          </div>
        </div>` : ''}
      </div>
    </div>
    <div class="wiki-meta">v${p.rev_no} &middot; ${t('wikiEditedBy')} ${esc(p.updated_by_name || '?')} &middot; ${fmtDate(p.updated_at)}</div>
    <div id="wiki-summary-panel"></div>
    <div class="wiki-content md-content">${p.content || `<p style="color:var(--fg3)">${t('wikiEmptyPage')}</p>`}</div>
    ${(p.backlinks && p.backlinks.length) ? `<div class="wiki-backlinks">
      <h3>${t('wikiBacklinks')}</h3>
      <ul>${p.backlinks.map(b => `<li><a href="/wiki/page/${b.id}" onclick="navigateTo('/wiki/page/${b.id}');return false;">${b.space_key ? esc(b.space_key) + ' / ' : ''}${esc(b.title)}</a></li>`).join('')}</ul>
    </div>` : ''}`;
  const contentEl = el.querySelector('.wiki-content');
  wikiFillSubpageTocs(contentEl, p.id);
  wikiFillToc(contentEl);
  wikiFillWikiLinks(contentEl, p.links, p.task_links);
  highlightCode();
  wikiEnsureLLM();
  wikiRenderCommentsSection(el, p);
}

// === Page comments + reactions ===
let wikiComments = [];

// wikiReactBarHtml renders the reaction chips + an add button for a target.
function wikiReactBarHtml(targetType, id, reactions) {
  const chips = (reactions || []).map(r =>
    `<button class="wiki-react ${r.mine ? 'mine' : ''}" onclick="wikiToggleReaction('${targetType}',${id},'${escAttr(r.emoji)}')">${esc(r.emoji)} ${r.count}</button>`).join('');
  return `<span class="wiki-react-bar">${chips}<button class="wiki-react-add" onclick="wikiReactPicker(event,'${targetType}',${id})">+</button></span>`;
}

function wikiReactPicker(ev, targetType, id) {
  ev.stopPropagation();
  document.querySelectorAll('.wiki-react-pop').forEach(p => p.remove());
  const pop = document.createElement('div');
  pop.className = 'wiki-react-pop';
  pop.innerHTML = (typeof CHAT_EMOJI !== 'undefined' ? CHAT_EMOJI : ['👍', '❤️', '🎉', '🔥', '👏'])
    .map(e => `<button onclick="wikiToggleReaction('${targetType}',${id},'${escAttr(e)}')">${esc(e)}</button>`).join('');
  document.body.appendChild(pop);
  const rect = ev.target.getBoundingClientRect();
  pop.style.left = rect.left + 'px';
  pop.style.top = (rect.bottom + 4) + 'px';
  setTimeout(() => document.addEventListener('click', () => pop.remove(), { once: true }), 0);
}

async function wikiToggleReaction(targetType, id, emoji) {
  document.querySelectorAll('.wiki-react-pop').forEach(p => p.remove());
  try {
    const url = targetType === 'page' ? `/api/wiki/pages/${id}/react` : `/api/wiki/comments/${id}/react`;
    const reacts = await api(url, 'POST', { emoji });
    if (targetType === 'page') {
      if (wikiState.page) wikiState.page.reactions = reacts;
      const bar = document.getElementById('wiki-page-reactions');
      if (bar) bar.innerHTML = wikiReactBarHtml('page', id, reacts);
    } else {
      const c = wikiComments.find(x => x.id === id);
      if (c) c.reactions = reacts;
      wikiRenderCommentList();
    }
  } catch (e) { alert(e.message); }
}

function wikiRenderCommentsSection(el, p) {
  const sec = document.createElement('div');
  sec.className = 'wiki-comments-section';
  sec.innerHTML = `
    <div id="wiki-page-reactions" class="wiki-page-reactions">${wikiReactBarHtml('page', p.id, p.reactions)}</div>
    <h3 class="wiki-comments-title">${t('comments')}</h3>
    <div id="wiki-comment-list"></div>
    ${currentUser && currentUser.role !== 'readonly' ? `<div class="wiki-comment-form">
      <textarea id="wiki-comment-input" rows="2" placeholder="${escAttr(t('writeComment'))}"></textarea>
      <button class="btn btn-primary btn-sm" onclick="wikiPostComment()">${t('send')}</button>
    </div>` : ''}`;
  el.appendChild(sec);
  wikiLoadComments(p.id);
}

async function wikiLoadComments(pageId) {
  try { wikiComments = await api(`/api/wiki/pages/${pageId}/comments`); }
  catch (e) { wikiComments = []; }
  wikiRenderCommentList();
}

// wikiCommentSortPref mirrors the task comment preference (Plan 2.12).
function wikiCommentSortPref() { return localStorage.getItem('kanban-comment-sort') === 'newest' ? 'newest' : 'oldest'; }

function wikiRenderCommentList() {
  const el = document.getElementById('wiki-comment-list');
  if (!el) return;
  if (!wikiComments.length) { el.innerHTML = `<div class="wiki-no-comments">${t('noComments')}</div>`; return; }
  // Build threads: top-level ordered by preference; replies always oldest-first.
  const byParent = {};
  for (const c of wikiComments) { const k = c.parent_id || 0; (byParent[k] = byParent[k] || []).push(c); }
  const tops = (byParent[0] || []).slice();
  if (wikiCommentSortPref() === 'newest') tops.reverse();
  const canModerate = wikiState.space && wikiRoleIsAdmin();
  const render = (c, depth) => {
    const own = currentUser && c.author_id === currentUser.id;
    const acts = [];
    if (currentUser && currentUser.role !== 'readonly') acts.push(`<a onclick="wikiReplyComment(${c.id})">${t('reply')}</a>`);
    if (own || canModerate) {
      acts.push(`<a onclick="wikiEditComment(${c.id})">${t('edit')}</a>`);
      acts.push(`<a onclick="wikiDeleteComment(${c.id})" style="color:var(--red)">${t('delete')}</a>`);
    }
    let h = `<div class="wiki-comment" style="margin-left:${depth * 22}px" id="wiki-comment-${c.id}">
      <div class="wiki-comment-head"><b>${esc(c.author_name || '?')}</b> <span class="wiki-comment-time">${fmtDate(c.updated_at || c.created_at)}</span></div>
      <div class="wiki-comment-body md-content">${renderMd(c.text)}</div>
      <div class="wiki-comment-foot">${wikiReactBarHtml('comment', c.id, c.reactions)} <span class="wiki-comment-acts">${acts.join('')}</span></div>
    </div>`;
    for (const child of (byParent[c.id] || [])) h += render(child, depth + 1);
    return h;
  };
  el.innerHTML = tops.map(c => render(c, 0)).join('');
}

function wikiRoleIsAdmin() {
  const sp = wikiState.space;
  return sp && sp.my_role === 'admin';
}

async function wikiPostComment(parentId) {
  const inp = document.getElementById('wiki-comment-input');
  if (!inp) return;
  const text = inp.value.trim();
  if (!text) return;
  try {
    await api(`/api/wiki/pages/${wikiState.page.id}/comments`, 'POST', { text, parent_id: parentId || null });
    inp.value = '';
    await wikiLoadComments(wikiState.page.id);
  } catch (e) { alert(e.message); }
}

async function wikiReplyComment(id) {
  const text = await uiPrompt(t('reply'));
  if (!text || !text.trim()) return;
  try {
    await api(`/api/wiki/pages/${wikiState.page.id}/comments`, 'POST', { text: text.trim(), parent_id: id });
    await wikiLoadComments(wikiState.page.id);
  } catch (e) { alert(e.message); }
}

async function wikiEditComment(id) {
  const c = wikiComments.find(x => x.id === id);
  const text = await uiPrompt(t('edit'), c ? c.text : '');
  if (text === null) return;
  try {
    await api('/api/wiki/comments/' + id, 'PUT', { text: text.trim() });
    await wikiLoadComments(wikiState.page.id);
  } catch (e) { alert(e.message); }
}

async function wikiDeleteComment(id) {
  if (!await uiConfirm(t('deleteConfirm'))) return;
  try {
    await api('/api/wiki/comments/' + id, 'DELETE');
    await wikiLoadComments(wikiState.page.id);
  } catch (e) { alert(e.message); }
}

// wikiFillWikiLinks turns literal [[Title]] / [[KEY:Title]] tokens in the rendered
// page into links, using the server-resolved token->pageId map (`links`). Resolved
// tokens become navigable anchors; unresolved ones render as a muted broken-link
// span. Layout markers ([[subpages]]/[[toc]]) and text inside code are skipped.
function wikiFillWikiLinks(container, links, taskLinks) {
  if (!container) return;
  links = links || {};
  taskLinks = taskLinks || {};
  const re = /\[\[\s*([^\]\n|]+?)\s*\]\]/g;
  const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT, null);
  const nodes = [];
  for (let n = walker.nextNode(); n; n = walker.nextNode()) nodes.push(n);
  for (const node of nodes) {
    const text = node.nodeValue;
    if (text.indexOf('[[') < 0) continue;
    if (node.parentElement && node.parentElement.closest('pre,code,a')) continue;
    re.lastIndex = 0;
    let m, last = 0, frag = null;
    while ((m = re.exec(text))) {
      const tok = m[1].trim();
      const low = tok.toLowerCase();
      if (low === 'subpages' || low === 'subpages:tree' || low === 'toc') continue;
      if (!frag) frag = document.createDocumentFragment();
      if (m.index > last) frag.appendChild(document.createTextNode(text.slice(last, m.index)));
      const isTask = Object.prototype.hasOwnProperty.call(taskLinks, tok);
      if (isTask) {
        // Task reference [[task:123]] / [[KEY-123]].
        const tid = taskLinks[tok] || 0;
        const label = tok.startsWith('task:') ? '#' + tok.slice(5) : tok;
        if (tid) {
          const a = document.createElement('a');
          a.className = 'wiki-link wiki-task-link';
          a.href = '/task/' + tid;
          a.textContent = label;
          a.addEventListener('click', e => { e.preventDefault(); navigateTo('/task/' + tid); });
          frag.appendChild(a);
        } else {
          const span = document.createElement('span');
          span.className = 'wiki-link-broken';
          span.title = t('wikiLinkBroken');
          span.textContent = label;
          frag.appendChild(span);
        }
      } else {
        const title = tok.includes(':') ? tok.slice(tok.indexOf(':') + 1).trim() : tok;
        const pid = links[tok] || 0;
        if (pid) {
          const a = document.createElement('a');
          a.className = 'wiki-link';
          a.href = '/wiki/page/' + pid;
          a.textContent = title;
          a.addEventListener('click', e => { e.preventDefault(); navigateTo('/wiki/page/' + pid); });
          frag.appendChild(a);
        } else {
          const span = document.createElement('span');
          span.className = 'wiki-link-broken';
          span.title = t('wikiLinkBroken');
          span.textContent = title;
          frag.appendChild(span);
        }
      }
      last = re.lastIndex;
    }
    if (frag) {
      if (last < text.length) frag.appendChild(document.createTextNode(text.slice(last)));
      node.parentNode.replaceChild(frag, node);
    }
  }
}

// wikiFillToc replaces a literal [[toc]] token with a generated table of contents
// built from the page's h1-h3 headings (each gets an id for anchor links).
function wikiFillToc(container) {
  if (!container) return;
  // Find a text node holding exactly the [[toc]] marker.
  const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT, null);
  let tocNode = null;
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    if (/^\s*\[\[\s*toc\s*\]\]\s*$/i.test(n.nodeValue)) { tocNode = n; break; }
  }
  if (!tocNode) return;
  const heads = container.querySelectorAll('h1, h2, h3');
  if (!heads.length) { tocNode.parentNode.removeChild(tocNode); return; }
  const nav = document.createElement('nav');
  nav.className = 'wiki-toc';
  const ul = document.createElement('ul');
  let i = 0;
  heads.forEach(h => {
    if (!h.id) h.id = 'h-' + (i++) + '-' + (h.textContent || '').toLowerCase().replace(/[^a-z0-9]+/g, '-').slice(0, 40);
    const li = document.createElement('li');
    li.className = 'wiki-toc-' + h.tagName.toLowerCase();
    const a = document.createElement('a');
    a.href = '#' + h.id;
    a.textContent = h.textContent;
    a.addEventListener('click', e => { e.preventDefault(); const el = document.getElementById(h.id); if (el) el.scrollIntoView({ behavior: 'smooth' }); });
    li.appendChild(a);
    ul.appendChild(li);
  });
  nav.appendChild(ul);
  // Replace the marker's containing <p> if it only held the marker, else the node.
  const p = tocNode.parentElement;
  if (p && p.tagName === 'P' && p.textContent.trim().match(/^\[\[\s*toc\s*\]\]$/i)) p.replaceWith(nav);
  else tocNode.parentNode.replaceChild(nav, tocNode);
}

// wikiEnsureLLM refreshes the LLM-integration status on each page view. If the
// value changed (e.g. an admin just enabled it), it re-renders so the Summary
// button appears/disappears. Re-rendering calls this again, but with the value
// now unchanged it won't loop.
async function wikiEnsureLLM() {
  try {
    const r = await api('/api/integrations/llm/status');
    const en = !!r.enabled;
    if (en !== wikiState.llmEnabled) {
      wikiState.llmEnabled = en;
      if (wikiState.page && wikiState.mode !== 'edit') wikiRenderMain();
    }
  } catch { /* leave previous state */ }
}

// wikiSummaryPanelOpen renders into the summary panel if the user is still on
// the same page (so background polling never clobbers another view).
function wikiSummaryPanelOpen(pageId) {
  if (!wikiState.page || wikiState.page.id !== pageId || wikiState.mode !== 'view') return null;
  return document.getElementById('wiki-summary-panel');
}
function wikiSummaryShow(pageId, bodyHtml, cached, isErr) {
  const panel = wikiSummaryPanelOpen(pageId);
  if (!panel) return;
  panel.innerHTML = `<div class="wiki-summary${isErr ? ' wiki-summary-err' : ''}">
    <div class="wiki-summary-head">&#10024; ${t('wikiSummary')}${cached ? ` <span class="wiki-summary-cached">${t('wikiSummaryCached')}</span>` : ''}</div>
    <div class="wiki-summary-body md-content">${bodyHtml}</div></div>`;
  if (typeof highlightCode === 'function') highlightCode();
}

// wikiSummarize starts (or fetches the cached) LLM summary. Generation runs in
// the background on the server, so leaving the page doesn't cancel it — the
// result is cached and shown on return. The client just polls for display.
async function wikiSummarize(pageId) {
  const panel = document.getElementById('wiki-summary-panel');
  if (!panel) return;
  wikiSummaryShow(pageId, t('wikiSummarizing'), false, false);
  try {
    const r = await api('/api/wiki/pages/' + pageId + '/summary', 'POST');
    if (r.status === 'ready') {
      wikiSummaryShow(pageId, wikiSanitizePreview(wikiMdToHtml(r.summary || '')), r.cached, false);
    } else if (r.status === 'error') {
      wikiSummaryShow(pageId, esc(r.error || ''), false, true);
    } else {
      wikiPollSummary(pageId);
    }
  } catch (e) {
    wikiSummaryShow(pageId, esc(e.message), false, true);
  }
}

// wikiPollSummary checks the server every few seconds until the background
// summary is ready. It stops if the user navigates away; server-side generation
// continues regardless and the result stays cached.
async function wikiPollSummary(pageId) {
  await new Promise(r => setTimeout(r, 3000));
  if (!wikiSummaryPanelOpen(pageId)) return;
  try {
    const r = await api('/api/wiki/pages/' + pageId + '/summary');
    if (r.status === 'ready') {
      wikiSummaryShow(pageId, wikiSanitizePreview(wikiMdToHtml(r.summary || '')), r.cached, false);
    } else if (r.status === 'error') {
      wikiSummaryShow(pageId, esc(r.error || ''), false, true);
    } else if (r.status === 'pending') {
      wikiPollSummary(pageId);
    }
    // status "none": nothing running and no cache — stop quietly.
  } catch (e) {
    wikiSummaryShow(pageId, esc(e.message), false, true);
  }
}

// === WYSIWYG editor ===
// Markdown toolbar: each button inserts markdown into the #wiki-md textarea.
const WIKI_TOOLBAR = [
  { act: 'h1', label: 'H1', key: 'tbH1' },
  { act: 'h2', label: 'H2', key: 'tbH2' },
  { act: 'h3', label: 'H3', key: 'tbH3' },
  { sep: true },
  { act: 'bold', label: '<b>B</b>', key: 'tbBold' },
  { act: 'italic', label: '<i>I</i>', key: 'tbItalic' },
  { act: 'strike', label: '<s>S</s>', key: 'tbStrike' },
  { act: 'inlineCode', label: '&lt;/&gt;', key: 'tbCode' },
  { sep: true },
  { act: 'ul', label: '&#8226;&#8801;', key: 'tbUl' },
  { act: 'ol', label: '1.&#8801;', key: 'tbOl' },
  { act: 'quote', label: '&#10078;', key: 'tbQuote' },
  { act: 'codeBlock', label: '{ }', key: 'tbCodeBlock' },
  { sep: true },
  { act: 'link', label: '&#128279;', key: 'tbLink' },
  { act: 'image', label: '&#128247;', key: 'tbImage' },
  { act: 'table', label: '&#9638;', key: 'tbTable' },
  { act: 'callout', label: '&#128161;', key: 'tbCallout' },
  { act: 'spoiler', label: '&#9656;', key: 'tbSpoiler' },
  { act: 'subpages', label: '&#9776;', key: 'tbSubpages' },
];

// wikiMdToHtml converts markdown to HTML via the bundled marked (falls back to
// escaped text). The server sanitizes the result on save.
function wikiMdToHtml(md) {
  const html = (typeof marked !== 'undefined' && marked.parse) ? marked.parse(md || '') : '<p>' + escHtmlText(md || '') + '</p>';
  return wikiInjectSubpageMarkers(html);
}

// wikiInjectSubpageMarkers turns the [[subpages]] / [[subpages:tree]] tokens into
// an empty placeholder div. The list is intentionally NOT baked in here: it is
// filled live on every render (wikiFillSubpageTocs) so it never goes stale when
// child pages are added, renamed, or removed.
function wikiInjectSubpageMarkers(html) {
  const repl = (_, mode) => `<div class="wiki-subpages${mode ? ' wiki-subpages-tree' : ''}"></div>`;
  return html
    .replace(/<p>\s*\[\[\s*subpages(\s*:\s*tree)?\s*\]\]\s*<\/p>/gi, repl)
    .replace(/\[\[\s*subpages(\s*:\s*tree)?\s*\]\]/gi, repl);
}

// wikiFillSubpageTocs replaces every .wiki-subpages placeholder inside `container`
// with a live list of `pageId`'s child pages (nested for the :tree variant).
function wikiFillSubpageTocs(container, pageId) {
  if (!container) return;
  const placeholders = container.querySelectorAll('.wiki-subpages');
  if (!placeholders.length) return;
  const byParent = {};
  for (const p of wikiState.tree) {
    const k = p.parent_id || 0;
    (byParent[k] = byParent[k] || []).push(p);
  }
  const renderList = (parent, deep) => {
    const kids = byParent[parent] || [];
    if (!kids.length) return '';
    return '<ul class="wiki-toc-list">' + kids.map(k =>
      `<li><a class="wiki-toc-link" href="/wiki/page/${k.id}" onclick="navigateTo('/wiki/page/${k.id}');return false;">${esc(k.title)}</a>${deep ? renderList(k.id, true) : ''}</li>`
    ).join('') + '</ul>';
  };
  placeholders.forEach(ph => {
    const deep = ph.classList.contains('wiki-subpages-tree');
    const html = pageId ? renderList(pageId, deep) : '';
    ph.innerHTML = html || `<span class="wiki-toc-empty">${t('wikiNoSubpages')}</span>`;
  });
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

function wikiRenderEditor(el) {
  const p = wikiState.page;
  // Markdown is the only editing mode. Seed from content_md, or derive markdown
  // from legacy HTML pages that have no stored markdown source.
  const md = p.content_md || (p.content ? wikiHtmlToMd(new DOMParser().parseFromString(p.content, 'text/html').body) : '');
  const toolbar = `<div class="wiki-toolbar" id="wiki-toolbar">${WIKI_TOOLBAR.map(b =>
    b.sep ? '<span class="wiki-tb-sep"></span>' :
      `<button type="button" class="wiki-tb-btn" title="${t(b.key)}" data-act="${b.act}">${b.label}</button>`).join('')}</div>`;
  el.innerHTML = `
    ${wikiBreadcrumbs()}
    <input class="wiki-title-input" id="wiki-title-input" value="${escAttr(p.title)}" maxlength="300">
    ${toolbar}
    <label class="wiki-sync-toggle"><input type="checkbox" id="wiki-sync-cb"> ${t('wikiSyncScroll')}</label>
    <div class="wiki-md-split">
      <textarea class="wiki-md-input" id="wiki-md" placeholder="# Markdown…">${esc(md)}</textarea>
      <div class="wiki-md-preview md-content" id="wiki-md-preview"></div>
    </div>
    <div class="wiki-edit-footer">
      <input class="wiki-comment-input" id="wiki-rev-comment" placeholder="${t('wikiComment')}" maxlength="200">
      <div style="display:flex;gap:8px">
        <button class="btn btn-ghost btn-sm" onclick="wikiCancelEdit()">${t('wikiCancel')}</button>
        <button class="btn btn-primary btn-sm" id="wiki-save-btn" onclick="wikiSavePage()">${t('wikiSave')}</button>
      </div>
    </div>`;
  wikiState.dirty = false;
  document.getElementById('wiki-title-input').addEventListener('input', () => { wikiState.dirty = true; });
  const ta = document.getElementById('wiki-md');
  ta.addEventListener('input', () => { wikiState.dirty = true; wikiMdPreview(); });
  ta.addEventListener('paste', wikiHandlePaste);
  const prev = document.getElementById('wiki-md-preview');
  wikiSetupScrollSync(ta, prev);
  const syncCb = document.getElementById('wiki-sync-cb');
  if (syncCb) {
    syncCb.checked = wikiScrollSyncOn();
    syncCb.addEventListener('change', () => {
      localStorage.setItem('wiki-scroll-sync', syncCb.checked ? '1' : '0');
      if (syncCb.checked) wikiSyncScroll(ta, prev); // align immediately on enable
    });
  }
  document.getElementById('wiki-toolbar').addEventListener('mousedown', e => {
    const btn = e.target.closest('.wiki-tb-btn');
    if (!btn) return;
    e.preventDefault(); // keep the textarea selection
    wikiTbAction(btn.dataset.act, btn);
  });
  wikiMdPreview();
  ta.focus();
}

// === Toolbar popovers (dependency-free) ===
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

// === Markdown insertion helpers (operate on the #wiki-md textarea) ===
function wikiMdTa() { return document.getElementById('wiki-md'); }
function wikiMdPreview() {
  const ta = wikiMdTa(), prev = document.getElementById('wiki-md-preview');
  if (!ta || !prev) return;
  prev.innerHTML = wikiSanitizePreview(wikiMdToHtml(ta.value));
  wikiFillSubpageTocs(prev, wikiState.page ? wikiState.page.id : null);
  if (typeof highlightCode === 'function') highlightCode();
  // Re-align the preview to the editor's current viewport after re-render so the
  // line being edited stays in view.
  wikiSyncScroll(ta, prev);
}

// === Editor ⇄ preview scroll sync ===
// Proportional, bidirectional, with a one-frame lock so the programmatic scroll
// of the target doesn't echo back and cause jitter.
let wikiScrollLock = false;
function wikiScrollSyncOn() { return localStorage.getItem('wiki-scroll-sync') === '1'; }
function wikiSyncScroll(src, dst) {
  if (!wikiScrollSyncOn() || wikiScrollLock || !src || !dst) return;
  const sMax = src.scrollHeight - src.clientHeight;
  const dMax = dst.scrollHeight - dst.clientHeight;
  if (sMax <= 0 || dMax <= 0) return;
  wikiScrollLock = true;
  dst.scrollTop = (src.scrollTop / sMax) * dMax;
  requestAnimationFrame(() => { wikiScrollLock = false; });
}
function wikiSetupScrollSync(ta, prev) {
  if (!ta || !prev) return;
  ta.addEventListener('scroll', () => wikiSyncScroll(ta, prev));
  prev.addEventListener('scroll', () => wikiSyncScroll(prev, ta));
  // After a keystroke or caret move the textarea keeps the caret visible itself;
  // mirror its viewport into the preview so the edited spot shows on both sides.
  const follow = () => wikiSyncScroll(ta, prev);
  ta.addEventListener('keyup', follow);
  ta.addEventListener('click', follow);
}
function wikiMdChanged() { wikiState.dirty = true; wikiMdPreview(); }
function wikiMdWrap(before, after) {
  const ta = wikiMdTa(); if (!ta) return;
  const s = ta.selectionStart, e = ta.selectionEnd, sel = ta.value.slice(s, e);
  ta.setRangeText(before + sel + after, s, e, 'end');
  ta.selectionStart = s + before.length;
  ta.selectionEnd = s + before.length + sel.length;
  ta.focus(); wikiMdChanged();
}
function wikiMdLinePrefix(prefix) {
  const ta = wikiMdTa(); if (!ta) return;
  const v = ta.value, s = ta.selectionStart, e = ta.selectionEnd;
  const ls = v.lastIndexOf('\n', s - 1) + 1;
  let le = v.indexOf('\n', e); if (le < 0) le = v.length;
  const ordered = prefix === '1. ';
  const block = v.slice(ls, le).split('\n').map((ln, i) => (ordered ? (i + 1) + '. ' : prefix) + ln).join('\n');
  ta.setRangeText(block, ls, le, 'end');
  ta.selectionStart = ls; ta.selectionEnd = ls + block.length;
  ta.focus(); wikiMdChanged();
}
function wikiMdInsert(text) {
  const ta = wikiMdTa(); if (!ta) return;
  const s = ta.selectionStart, e = ta.selectionEnd;
  ta.setRangeText(text, s, e, 'end');
  ta.focus(); wikiMdChanged();
}

function wikiTbAction(act, anchor) {
  switch (act) {
    case 'h1': return wikiMdLinePrefix('# ');
    case 'h2': return wikiMdLinePrefix('## ');
    case 'h3': return wikiMdLinePrefix('### ');
    case 'bold': return wikiMdWrap('**', '**');
    case 'italic': return wikiMdWrap('*', '*');
    case 'strike': return wikiMdWrap('~~', '~~');
    case 'inlineCode': return wikiMdWrap('`', '`');
    case 'ul': return wikiMdLinePrefix('- ');
    case 'ol': return wikiMdLinePrefix('1. ');
    case 'quote': return wikiMdLinePrefix('> ');
    case 'codeBlock': return wikiMdCodeBlock();
    case 'link': return wikiMdLink(anchor);
    case 'image': return wikiMdImagePick();
    case 'table': return wikiMdTablePick(anchor);
    case 'callout': return wikiMdCalloutPick(anchor);
    case 'spoiler': return wikiMdSpoiler();
    case 'subpages': return wikiMdSubpagesPick(anchor);
  }
}

function wikiMdSubpagesPick(anchor) {
  const opts = [['', 'tbSubpagesDirect'], [':tree', 'tbSubpagesTree']];
  const pop = wikiPopover(anchor, `<div class="wiki-callout-list">${opts.map(([m, lab]) => `<button class="wiki-callout-opt" data-m="${m}">${t(lab)}</button>`).join('')}</div>`);
  pop.querySelectorAll('.wiki-callout-opt').forEach(b => b.onclick = () => {
    const m = b.dataset.m;
    wikiClosePopover();
    wikiMdInsert('\n\n[[subpages' + m + ']]\n\n');
  });
}

function wikiMdCodeBlock() {
  const ta = wikiMdTa(); if (!ta) return;
  const s = ta.selectionStart, e = ta.selectionEnd, sel = ta.value.slice(s, e);
  ta.setRangeText('```\n' + sel + '\n```\n', s, e, 'end');
  ta.focus(); wikiMdChanged();
}

function wikiMdLink(anchor) {
  const ta = wikiMdTa(); if (!ta) return;
  const s = ta.selectionStart, e = ta.selectionEnd, selText = ta.value.slice(s, e);
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
    ta.setRangeText('[' + (txt || url) + '](' + url + ')', s, e, 'end');
    ta.focus(); wikiMdChanged();
  };
}

function wikiMdImagePick() {
  const inp = document.createElement('input');
  inp.type = 'file';
  inp.accept = 'image/png,image/jpeg,image/gif,image/webp';
  inp.onchange = () => { const f = inp.files[0]; if (f) wikiUploadImage(f); };
  inp.click();
}

function wikiMdTablePick(anchor) {
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
      wikiClosePopover();
      wikiMdInsert('\n\n' + wikiBuildMdTable(rows, cols) + '\n');
    });
  });
}

function wikiBuildMdTable(rows, cols) {
  const row = () => '| ' + Array(cols).fill('   ').join(' | ') + ' |';
  const sep = '| ' + Array(cols).fill('---').join(' | ') + ' |';
  const lines = [row(), sep];
  for (let r = 1; r < rows; r++) lines.push(row());
  return lines.join('\n') + '\n';
}

function wikiMdCalloutPick(anchor) {
  const ta = wikiMdTa(); if (!ta) return;
  const s = ta.selectionStart, e = ta.selectionEnd, selText = ta.value.slice(s, e);
  const kinds = [['info', 'tbCalloutInfo'], ['warn', 'tbCalloutWarn'], ['note', 'tbCalloutNote'], ['success', 'tbCalloutSuccess']];
  const pop = wikiPopover(anchor, `<div class="wiki-callout-list">${kinds.map(([k, lab]) => `<button class="wiki-callout-opt wiki-${k}" data-k="${k}">${t(lab)}</button>`).join('')}</div>`);
  pop.querySelectorAll('.wiki-callout-opt').forEach(b => b.onclick = () => {
    const k = b.dataset.k;
    wikiClosePopover();
    ta.setRangeText('\n<div class="wiki-' + k + '">\n\n' + (selText || '…') + '\n\n</div>\n', s, e, 'end');
    ta.focus(); wikiMdChanged();
  });
}

// wikiMdSpoiler inserts a collapsible <details> block. marked passes it through
// as an HTML block (like callouts) and the inner blank lines let its body be
// parsed as markdown; the sanitizer already whitelists details/summary.
function wikiMdSpoiler() {
  const ta = wikiMdTa(); if (!ta) return;
  const s = ta.selectionStart, e = ta.selectionEnd, sel = ta.value.slice(s, e);
  const block = '\n<details>\n<summary>' + t('tbSpoilerTitle') + '</summary>\n\n' + (sel || '…') + '\n\n</details>\n';
  ta.setRangeText(block, s, e, 'end');
  ta.focus(); wikiMdChanged();
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
      const alt = (file.name || 'image').replace(/[\[\]]/g, '');
      wikiMdInsert('![' + alt + '](' + res.url + ')');
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
  const contentMd = document.getElementById('wiki-md').value;
  const content = wikiMdToHtml(contentMd);
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

async function wikiCancelEdit() {
  if (wikiState.dirty && !await uiConfirm(t('wikiUnsaved'))) return;
  wikiState.dirty = false;
  navigateTo('/wiki/page/' + wikiState.page.id);
}

// === Page operations ===
async function wikiNewPage(parentId) {
  const title = await uiPrompt(t('wikiNewPageTitle'));
  if (!title || !title.trim()) return;
  // Offer space templates (if any) to seed the new page's content.
  let templates = [];
  try { templates = await api('/api/wiki/spaces/' + wikiState.spaceId + '/templates'); } catch (e) { templates = []; }
  const createWith = async (contentMd) => {
    try {
      const res = await api('/api/wiki/pages', 'POST', {
        space_id: wikiState.spaceId, parent_id: parentId, title: title.trim(), content: '', content_md: contentMd || '',
      });
      navigateTo('/wiki/page/' + res.id + '/edit');
    } catch (e) { alert(e.message); }
  };
  if (!templates.length) { createWith(''); return; }
  // Picker: blank or one of the templates.
  const ov = document.createElement('div');
  ov.className = 'modal-overlay';
  ov.innerHTML = `<div class="modal"><h3>${esc(t('wikiChooseTemplate'))}</h3>
    <div class="wiki-tpl-list">
      <button class="btn btn-ghost" data-md="">${esc(t('wikiBlankPage'))}</button>
      ${templates.map((tp, i) => `<button class="btn btn-ghost" data-i="${i}">${esc(tp.title)}</button>`).join('')}
    </div></div>`;
  ov.addEventListener('click', e => {
    if (e.target === ov) { ov.remove(); return; }
    const btn = e.target.closest('button');
    if (!btn) return;
    const md = btn.dataset.i !== undefined ? (templates[Number(btn.dataset.i)].content_md || '') : '';
    ov.remove();
    createWith(md);
  });
  document.body.appendChild(ov);
}

// wikiToggleTemplate marks/unmarks the current page as a reusable template.
async function wikiToggleTemplate(id) {
  const p = wikiState.page;
  const next = !(p && p.is_template);
  try {
    await api('/api/wiki/pages/' + id + '/template', 'POST', { is_template: next });
    if (p) p.is_template = next;
    wikiRenderMain();
  } catch (e) { alert(e.message); }
}

async function wikiDeletePage(id) {
  if (!await uiConfirm(t('wikiDeleteConfirm'))) return;
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
  wikiState.diffSel = [];
  el.innerHTML = `
    ${wikiBreadcrumbs()}
    <div class="wiki-page-header">
      <h1 class="wiki-title">${esc(p.title)} — ${t('wikiHistory')}</h1>
      <div class="wiki-actions"><button class="btn btn-ghost btn-sm" onclick="navigateTo('/wiki/page/${p.id}')">&larr; ${t('wikiBack')}</button></div>
    </div>
    <div class="wiki-history">
      <div class="wiki-rev-list">
        <div class="wiki-rev-comparebar">
          <button class="btn btn-primary btn-sm" id="wiki-diff-btn" disabled onclick="wikiCompareRevisions()">&#8646; ${t('wikiCompare')}</button>
          <span class="wiki-rev-comparehint">${t('wikiCompareHint')}</span>
        </div>
        ${revs.map(r => `<div class="wiki-rev-item ${wikiState.viewingRev === r.id ? 'active' : ''}" onclick="wikiViewRevision(${r.id})">
          <div class="wiki-rev-row1">
            <input type="checkbox" class="wiki-rev-cb" title="${t('wikiCompare')}" onclick="event.stopPropagation()" onchange="wikiToggleDiffSelect(${r.id}, this)">
            <b>v${r.rev_no}</b> ${r.rev_no === p.rev_no ? `<span class="wiki-rev-current">${t('wikiCurrent')}</span>` : ''}
          </div>
          <div class="wiki-rev-meta">${esc(r.author_name || '?')} &middot; ${fmtDate(r.created_at)}</div>
          ${r.comment ? `<div class="wiki-rev-comment">${esc(r.comment)}</div>` : ''}
        </div>`).join('')}
      </div>
      <div class="wiki-rev-view" id="wiki-rev-view"><div class="wiki-empty">${t('wikiPickRevision')}</div></div>
    </div>`;
}

// wikiToggleDiffSelect tracks up to two revisions selected for comparison.
function wikiToggleDiffSelect(revId, cb) {
  const sel = wikiState.diffSel || (wikiState.diffSel = []);
  if (cb.checked) {
    sel.push(revId);
    if (sel.length > 2) {
      const drop = sel.shift();
      document.querySelectorAll('.wiki-rev-cb').forEach(el => {
        if (el.getAttribute('onchange').includes('(' + drop + ',')) el.checked = false;
      });
    }
  } else {
    const i = sel.indexOf(revId);
    if (i >= 0) sel.splice(i, 1);
  }
  const btn = document.getElementById('wiki-diff-btn');
  if (btn) btn.disabled = sel.length !== 2;
}

// wikiRevText prefers the markdown source for a readable diff, falling back to
// the plain-text content of older revisions that predate stored markdown.
function wikiRevText(rev) {
  if (rev.content_md && rev.content_md.trim()) return rev.content_md;
  return new DOMParser().parseFromString(rev.content || '', 'text/html').body.textContent || '';
}

// wikiLineDiff computes a line-level LCS diff: ['ctx'|'add'|'del', text].
function wikiLineDiff(aText, bText) {
  const a = aText.split('\n'), b = bText.split('\n');
  const n = a.length, m = b.length;
  const dp = Array.from({ length: n + 1 }, () => new Int32Array(m + 1));
  for (let i = n - 1; i >= 0; i--)
    for (let j = m - 1; j >= 0; j--)
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
  const out = [];
  let i = 0, j = 0;
  while (i < n && j < m) {
    if (a[i] === b[j]) { out.push(['ctx', a[i]]); i++; j++; }
    else if (dp[i + 1][j] >= dp[i][j + 1]) { out.push(['del', a[i]]); i++; }
    else { out.push(['add', b[j]]); j++; }
  }
  while (i < n) out.push(['del', a[i++]]);
  while (j < m) out.push(['add', b[j++]]);
  return out;
}

async function wikiCompareRevisions() {
  const sel = wikiState.diffSel || [];
  if (sel.length !== 2) return;
  const view = document.getElementById('wiki-rev-view');
  try {
    const [r1, r2] = await Promise.all(sel.map(id => api('/api/wiki/revisions/' + id)));
    // Order older → newer so additions read as "what the newer version added".
    const [from, to] = r1.rev_no <= r2.rev_no ? [r1, r2] : [r2, r1];
    document.querySelectorAll('.wiki-rev-item').forEach(el => el.classList.remove('active'));
    const diff = wikiLineDiff(wikiRevText(from), wikiRevText(to));
    const changed = diff.some(d => d[0] !== 'ctx');
    const body = diff.map(([k, txt]) => {
      const sym = k === 'add' ? '+' : k === 'del' ? '−' : ' ';
      return `<div class="wiki-diff-line wiki-diff-${k}"><span class="wiki-diff-sym">${sym}</span><span class="wiki-diff-txt">${esc(txt) || ' '}</span></div>`;
    }).join('');
    view.innerHTML = `
      <div class="wiki-rev-view-head">${t('wikiCompare')}: <b>v${from.rev_no}</b> &rarr; <b>v${to.rev_no}</b></div>
      ${changed ? `<div class="wiki-diff">${body}</div>` : `<div class="wiki-empty">${t('wikiNoChanges')}</div>`}`;
  } catch (e) { alert(e.message); }
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
  if (!await uiConfirm(t('wikiRestoreConfirm'))) return;
  try {
    await api('/api/wiki/pages/' + wikiState.page.id + '/restore', 'POST', { revision_id: revId });
    navigateTo('/wiki/page/' + wikiState.page.id);
  } catch (e) { alert(e.message); }
}

// === Spaces: create & settings ===
async function wikiNewSpaceDialog() {
  const name = await uiPrompt(t('wikiSpaceNamePrompt'));
  if (!name || !name.trim()) return;
  let key = await uiPrompt(t('wikiSpaceKeyPrompt'), wikiSuggestKey(name));
  if (!key) return;
  key = key.trim().toUpperCase();
  const personal = await uiConfirm(t('wikiPersonalSpace'));
  try {
    const res = await api('/api/wiki/spaces', 'POST', { key, name: name.trim(), description: '', visibility: 'open', personal });
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

function wikiRenderSpaceSettings(el) {
  const sp = wikiSpace();
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
        <button class="btn btn-ghost btn-sm" onclick="window.open('/api/wiki/spaces/${sp.id}/export','_blank')">&#11015; ${t('wikiExportSpace')}</button>
        <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="wikiDeleteSpace()">${t('wikiDeleteSpace')}</button>
      </div>
      <h3>${t('wikiMembers')}</h3>
      <div id="wiki-members" class="wiki-members"></div>
    </div>`;
  wikiRenderMembers();
}

// Renders just the members block into #wiki-members so add/remove/role changes
// never wipe the unsaved name/description/visibility fields above it. The space
// owner (and the implicit "everyone" access of open spaces) is surfaced so the
// list never looks empty while people still have access.
async function wikiRenderMembers() {
  const container = document.getElementById('wiki-members');
  if (!container) return;
  const sp = wikiSpace();
  let members = [];
  try { members = await api('/api/wiki/spaces/' + sp.id + '/members'); }
  catch (e) { container.innerHTML = `<div class="wiki-empty">${esc(e.message)}</div>`; return; }
  // Re-target in case the view changed while the request was in flight.
  if (!document.getElementById('wiki-members')) return;
  const ownerId = sp.created_by;
  const owner = ownerId ? (state.users || []).find(u => u.id === ownerId) : null;
  const rows = members.filter(m => m.user_id !== ownerId);
  const taken = new Set(members.map(m => m.user_id));
  if (ownerId) taken.add(ownerId);
  const allUsers = (state.users || []).filter(u => !taken.has(u.id));

  let listHtml = '';
  if (owner) listHtml += `<div class="entity-item">
      <span>${esc(owner.username)}</span>
      <span class="wiki-owner-badge">${t('wikiRole_admin')} · ${t('wikiOwner')}</span>
    </div>`;
  listHtml += rows.map(m => `<div class="entity-item">
      <span>${esc(m.username)}</span>
      <select onchange="wikiSetMember(${m.user_id}, this.value)">
        ${['viewer', 'editor', 'admin'].map(r => `<option value="${r}" ${m.role === r ? 'selected' : ''}>${t('wikiRole_' + r)}</option>`).join('')}
      </select>
      <button class="btn-icon btn-sm" style="color:var(--red)" title="${escAttr(t('delete'))}" onclick="wikiRemoveMember(${m.user_id})">&#10005;</button>
    </div>`).join('');
  if (!owner && !rows.length) listHtml += `<div class="wiki-empty">${t('wikiNoMembers')}</div>`;

  const hint = sp.visibility === 'open' ? `<div class="wiki-members-hint">${t('wikiOpenHint')}</div>` : '';
  const addHtml = allUsers.length ? `<div class="entity-add" style="margin-top:8px">
      <select id="wiki-new-member">${allUsers.map(u => `<option value="${u.id}">${esc(u.username)}</option>`).join('')}</select>
      <select id="wiki-new-member-role">
        <option value="viewer">${t('wikiRole_viewer')}</option>
        <option value="editor" selected>${t('wikiRole_editor')}</option>
        <option value="admin">${t('wikiRole_admin')}</option>
      </select>
      <button class="btn btn-primary btn-sm" onclick="wikiAddMember()">+</button>
    </div>` : '';

  container.innerHTML = hint + `<div class="entity-list">${listHtml}</div>` + addHtml;
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
  if (!await uiConfirm(t('wikiDeleteSpaceConfirm').replace('{n}', sp.page_count))) return;
  try {
    await api('/api/wiki/spaces/' + sp.id, 'DELETE');
    localStorage.removeItem('kanban-wiki-space');
    navigateTo('/wiki');
  } catch (e) { alert(e.message); }
}

async function wikiAddMember() {
  const sp = wikiSpace();
  const sel = document.getElementById('wiki-new-member');
  if (!sel || !sel.value) return;
  const uid = Number(sel.value);
  const role = document.getElementById('wiki-new-member-role').value;
  try {
    await api('/api/wiki/spaces/' + sp.id + '/members', 'POST', { user_id: uid, role });
    wikiRenderMembers();
  } catch (e) { alert(e.message); }
}

async function wikiSetMember(uid, role) {
  const sp = wikiSpace();
  try { await api('/api/wiki/spaces/' + sp.id + '/members', 'POST', { user_id: uid, role }); }
  catch (e) { alert(e.message); wikiRenderMembers(); }
}

async function wikiRemoveMember(uid) {
  const sp = wikiSpace();
  try {
    await api('/api/wiki/spaces/' + sp.id + '/members?user_id=' + uid, 'DELETE');
    wikiRenderMembers();
  } catch (e) { alert(e.message); }
}

// warn about unsaved edits when leaving the tab
window.addEventListener('beforeunload', e => {
  if (wikiState.dirty) { e.preventDefault(); e.returnValue = ''; }
});
