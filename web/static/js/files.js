// Files service frontend (shared storage: folder tree + public links). Relies on
// shared helpers from the inline script (api, t, esc, escAttr, fmtDate, toast,
// uiConfirm, uiPrompt, openLightbox, isReadonly, navigateTo).

let filesState = {
  folderId: null, // null = root
  tree: [],       // flat folder list (whole tree)
  folders: [],    // subfolders of the current folder
  items: [],
  usage: null,
  es: null,
  uploading: 0,
};

const FILES_I18N = {
  en: {
    filesTitle: 'Files', filesUpload: 'Upload', filesNewFolder: 'New folder', filesRoot: 'Files',
    filesEmpty: 'Empty folder', filesDropHint: 'Drop files here to upload', filesUploading: 'Uploading…',
    filesRename: 'Rename', filesMove: 'Move', filesDelete: 'Delete', filesDownload: 'Download',
    filesPreview: 'Preview', filesShare: 'Public link', filesRevoke: 'Revoke link', filesCopyLink: 'Copy link',
    filesLinkCopied: 'Public link copied', filesLinkRevoked: 'Public link revoked', filesShared: 'Shared by link',
    filesDeleteConfirm: 'Delete this file?', filesDeleteFolderConfirm: 'Delete the folder with everything inside?',
    filesFolderName: 'Folder name', filesNewName: 'New name', filesMoveTo: 'Move to', filesNameTaken: 'This name is already taken',
    filesFree: 'free', filesReadonly: 'Read-only access',
  },
  ru: {
    filesTitle: 'Файлы', filesUpload: 'Загрузить', filesNewFolder: 'Новая папка', filesRoot: 'Файлы',
    filesEmpty: 'Папка пуста', filesDropHint: 'Перетащите файлы сюда для загрузки', filesUploading: 'Загрузка…',
    filesRename: 'Переименовать', filesMove: 'Переместить', filesDelete: 'Удалить', filesDownload: 'Скачать',
    filesPreview: 'Просмотр', filesShare: 'Публичная ссылка', filesRevoke: 'Отозвать ссылку', filesCopyLink: 'Копировать ссылку',
    filesLinkCopied: 'Публичная ссылка скопирована', filesLinkRevoked: 'Публичная ссылка отозвана', filesShared: 'Открыт по ссылке',
    filesDeleteConfirm: 'Удалить файл?', filesDeleteFolderConfirm: 'Удалить папку со всем содержимым?',
    filesFolderName: 'Название папки', filesNewName: 'Новое имя', filesMoveTo: 'Переместить в', filesNameTaken: 'Такое имя уже занято',
    filesFree: 'свободно', filesReadonly: 'Только чтение',
  },
  zh: {
    filesTitle: '文件', filesUpload: '上传', filesNewFolder: '新建文件夹', filesRoot: '文件',
    filesEmpty: '文件夹为空', filesDropHint: '将文件拖到此处上传', filesUploading: '上传中…',
    filesRename: '重命名', filesMove: '移动', filesDelete: '删除', filesDownload: '下载',
    filesPreview: '预览', filesShare: '公开链接', filesRevoke: '撤销链接', filesCopyLink: '复制链接',
    filesLinkCopied: '公开链接已复制', filesLinkRevoked: '公开链接已撤销', filesShared: '已通过链接共享',
    filesDeleteConfirm: '删除此文件？', filesDeleteFolderConfirm: '删除文件夹及其全部内容？',
    filesFolderName: '文件夹名称', filesNewName: '新名称', filesMoveTo: '移动到', filesNameTaken: '该名称已被占用',
    filesFree: '可用', filesReadonly: '只读访问',
  },
  es: {
    filesTitle: 'Archivos', filesUpload: 'Subir', filesNewFolder: 'Nueva carpeta', filesRoot: 'Archivos',
    filesEmpty: 'Carpeta vacía', filesDropHint: 'Suelta archivos aquí para subirlos', filesUploading: 'Subiendo…',
    filesRename: 'Renombrar', filesMove: 'Mover', filesDelete: 'Eliminar', filesDownload: 'Descargar',
    filesPreview: 'Vista previa', filesShare: 'Enlace público', filesRevoke: 'Revocar enlace', filesCopyLink: 'Copiar enlace',
    filesLinkCopied: 'Enlace público copiado', filesLinkRevoked: 'Enlace público revocado', filesShared: 'Compartido por enlace',
    filesDeleteConfirm: '¿Eliminar este archivo?', filesDeleteFolderConfirm: '¿Eliminar la carpeta con todo su contenido?',
    filesFolderName: 'Nombre de la carpeta', filesNewName: 'Nuevo nombre', filesMoveTo: 'Mover a', filesNameTaken: 'Ese nombre ya está en uso',
    filesFree: 'libre', filesReadonly: 'Acceso de solo lectura',
  },
  fr: {
    filesTitle: 'Fichiers', filesUpload: 'Téléverser', filesNewFolder: 'Nouveau dossier', filesRoot: 'Fichiers',
    filesEmpty: 'Dossier vide', filesDropHint: 'Déposez des fichiers ici pour les téléverser', filesUploading: 'Téléversement…',
    filesRename: 'Renommer', filesMove: 'Déplacer', filesDelete: 'Supprimer', filesDownload: 'Télécharger',
    filesPreview: 'Aperçu', filesShare: 'Lien public', filesRevoke: 'Révoquer le lien', filesCopyLink: 'Copier le lien',
    filesLinkCopied: 'Lien public copié', filesLinkRevoked: 'Lien public révoqué', filesShared: 'Partagé par lien',
    filesDeleteConfirm: 'Supprimer ce fichier ?', filesDeleteFolderConfirm: 'Supprimer le dossier et tout son contenu ?',
    filesFolderName: 'Nom du dossier', filesNewName: 'Nouveau nom', filesMoveTo: 'Déplacer vers', filesNameTaken: 'Ce nom est déjà pris',
    filesFree: 'libre', filesReadonly: 'Accès en lecture seule',
  },
  de: {
    filesTitle: 'Dateien', filesUpload: 'Hochladen', filesNewFolder: 'Neuer Ordner', filesRoot: 'Dateien',
    filesEmpty: 'Ordner ist leer', filesDropHint: 'Dateien zum Hochladen hierher ziehen', filesUploading: 'Wird hochgeladen…',
    filesRename: 'Umbenennen', filesMove: 'Verschieben', filesDelete: 'Löschen', filesDownload: 'Herunterladen',
    filesPreview: 'Vorschau', filesShare: 'Öffentlicher Link', filesRevoke: 'Link widerrufen', filesCopyLink: 'Link kopieren',
    filesLinkCopied: 'Öffentlicher Link kopiert', filesLinkRevoked: 'Öffentlicher Link widerrufen', filesShared: 'Per Link geteilt',
    filesDeleteConfirm: 'Diese Datei löschen?', filesDeleteFolderConfirm: 'Ordner mit gesamtem Inhalt löschen?',
    filesFolderName: 'Ordnername', filesNewName: 'Neuer Name', filesMoveTo: 'Verschieben nach', filesNameTaken: 'Dieser Name ist bereits vergeben',
    filesFree: 'frei', filesReadonly: 'Nur-Lese-Zugriff',
  },
  pt: {
    filesTitle: 'Arquivos', filesUpload: 'Enviar', filesNewFolder: 'Nova pasta', filesRoot: 'Arquivos',
    filesEmpty: 'Pasta vazia', filesDropHint: 'Solte arquivos aqui para enviar', filesUploading: 'Enviando…',
    filesRename: 'Renomear', filesMove: 'Mover', filesDelete: 'Excluir', filesDownload: 'Baixar',
    filesPreview: 'Visualizar', filesShare: 'Link público', filesRevoke: 'Revogar link', filesCopyLink: 'Copiar link',
    filesLinkCopied: 'Link público copiado', filesLinkRevoked: 'Link público revogado', filesShared: 'Compartilhado por link',
    filesDeleteConfirm: 'Excluir este arquivo?', filesDeleteFolderConfirm: 'Excluir a pasta com todo o conteúdo?',
    filesFolderName: 'Nome da pasta', filesNewName: 'Novo nome', filesMoveTo: 'Mover para', filesNameTaken: 'Esse nome já está em uso',
    filesFree: 'livre', filesReadonly: 'Acesso somente leitura',
  },
  ja: {
    filesTitle: 'ファイル', filesUpload: 'アップロード', filesNewFolder: '新しいフォルダー', filesRoot: 'ファイル',
    filesEmpty: 'フォルダーは空です', filesDropHint: 'ここにファイルをドロップしてアップロード', filesUploading: 'アップロード中…',
    filesRename: '名前を変更', filesMove: '移動', filesDelete: '削除', filesDownload: 'ダウンロード',
    filesPreview: 'プレビュー', filesShare: '公開リンク', filesRevoke: 'リンクを無効化', filesCopyLink: 'リンクをコピー',
    filesLinkCopied: '公開リンクをコピーしました', filesLinkRevoked: '公開リンクを無効化しました', filesShared: 'リンクで共有中',
    filesDeleteConfirm: 'このファイルを削除しますか？', filesDeleteFolderConfirm: 'フォルダーと中身をすべて削除しますか？',
    filesFolderName: 'フォルダー名', filesNewName: '新しい名前', filesMoveTo: '移動先', filesNameTaken: 'その名前は既に使われています',
    filesFree: '空き', filesReadonly: '読み取り専用アクセス',
  },
  ko: {
    filesTitle: '파일', filesUpload: '업로드', filesNewFolder: '새 폴더', filesRoot: '파일',
    filesEmpty: '폴더가 비어 있습니다', filesDropHint: '업로드하려면 파일을 여기에 놓으세요', filesUploading: '업로드 중…',
    filesRename: '이름 바꾸기', filesMove: '이동', filesDelete: '삭제', filesDownload: '다운로드',
    filesPreview: '미리보기', filesShare: '공개 링크', filesRevoke: '링크 취소', filesCopyLink: '링크 복사',
    filesLinkCopied: '공개 링크가 복사되었습니다', filesLinkRevoked: '공개 링크가 취소되었습니다', filesShared: '링크로 공유됨',
    filesDeleteConfirm: '이 파일을 삭제할까요?', filesDeleteFolderConfirm: '폴더와 모든 내용을 삭제할까요?',
    filesFolderName: '폴더 이름', filesNewName: '새 이름', filesMoveTo: '이동 위치', filesNameTaken: '이미 사용 중인 이름입니다',
    filesFree: '여유', filesReadonly: '읽기 전용 접근',
  },
  ar: {
    filesTitle: 'الملفات', filesUpload: 'رفع', filesNewFolder: 'مجلد جديد', filesRoot: 'الملفات',
    filesEmpty: 'المجلد فارغ', filesDropHint: 'أفلت الملفات هنا للرفع', filesUploading: 'جارٍ الرفع…',
    filesRename: 'إعادة تسمية', filesMove: 'نقل', filesDelete: 'حذف', filesDownload: 'تنزيل',
    filesPreview: 'معاينة', filesShare: 'رابط عام', filesRevoke: 'إلغاء الرابط', filesCopyLink: 'نسخ الرابط',
    filesLinkCopied: 'تم نسخ الرابط العام', filesLinkRevoked: 'تم إلغاء الرابط العام', filesShared: 'مشارك برابط',
    filesDeleteConfirm: 'حذف هذا الملف؟', filesDeleteFolderConfirm: 'حذف المجلد بكل محتوياته؟',
    filesFolderName: 'اسم المجلد', filesNewName: 'الاسم الجديد', filesMoveTo: 'نقل إلى', filesNameTaken: 'هذا الاسم مستخدم بالفعل',
    filesFree: 'متاح', filesReadonly: 'وصول للقراءة فقط',
  },
};
let filesI18nMerged = false;
function ensureFilesI18n() {
  if (filesI18nMerged || typeof LANGS === 'undefined') return;
  for (const lang of ['ru', 'en', 'zh', 'es', 'fr', 'de', 'pt', 'ja', 'ko', 'ar']) {
    if (!LANGS[lang]) continue;
    const dict = Object.assign({}, FILES_I18N.en, FILES_I18N[lang] || {});
    for (const k in dict) if (!(k in LANGS[lang])) LANGS[lang][k] = dict[k];
  }
  filesI18nMerged = true;
}

function filesInjectCSS() {
  if (document.getElementById('files-css')) return;
  const st = document.createElement('style');
  st.id = 'files-css';
  st.textContent = `
.files-wrap{max-width:920px;margin:0 auto;padding:12px 16px}
.files-crumbs{display:flex;align-items:center;flex-wrap:wrap;gap:4px;padding:6px 0 10px;font-size:.92rem}
.files-crumbs a{color:var(--accent);text-decoration:none;cursor:pointer}
.files-crumbs .sep{color:var(--fg2)}
.files-list{border:1px solid var(--border);border-radius:var(--radius-lg);overflow:hidden}
.files-row{display:flex;align-items:center;gap:10px;padding:8px 12px;border-bottom:1px solid var(--border);cursor:default}
.files-row:last-child{border-bottom:none}
.files-row:hover{background:var(--bg2)}
.files-row .f-icon{font-size:1.15rem;width:24px;text-align:center;flex:none}
.files-row .f-name{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.files-row .f-name.folder{cursor:pointer;font-weight:600}
.files-row .f-meta{color:var(--fg2);font-size:.8rem;flex:none;white-space:nowrap}
.files-row .f-actions{display:flex;gap:2px;flex:none;opacity:0;transition:opacity .15s}
.files-row:hover .f-actions{opacity:1}
.files-row .f-actions .btn{padding:2px 6px}
.files-row .f-shared{color:var(--accent)}
.files-empty{padding:40px 16px;text-align:center;color:var(--fg2)}
.files-drop{outline:2px dashed var(--accent);outline-offset:-8px;border-radius:var(--radius-lg)}
.files-usage{color:var(--fg2);font-size:.8rem;white-space:nowrap}
.files-preview-pre{max-height:70vh;overflow:auto;white-space:pre-wrap;word-break:break-word;background:var(--bg);border-radius:var(--radius);padding:12px;font-size:.85rem}
`;
  document.head.appendChild(st);
}

// === Entry point ===
async function renderFilesView() {
  ensureFilesI18n();
  filesInjectCSS();
  const m = (typeof currentServicePath === 'string' ? currentServicePath : '').match(/^\/folder\/(\d+)$/);
  filesState.folderId = m ? Number(m[1]) : null;
  filesRenderHeader();
  filesConnectSSE();
  await filesLoad();
}

function filesRenderHeader() {
  const hdr = document.getElementById('files-hdr');
  if (!hdr) return;
  const ro = isReadonly();
  hdr.innerHTML = `
    ${ro ? '' : `<button class="btn btn-ghost btn-sm" onclick="filesNewFolder()">&#128193;+ ${esc(t('filesNewFolder'))}</button>
    <button class="btn btn-primary btn-sm" onclick="document.getElementById('files-upload-input').click()">&#8679; ${esc(t('filesUpload'))}</button>
    <input type="file" id="files-upload-input" multiple style="display:none" onchange="filesUploadInput(this)">`}
    <span class="files-usage" id="files-usage"></span>`;
}

async function filesLoad() {
  try {
    const [tree, list] = await Promise.all([
      api('/api/storage/tree'),
      api('/api/storage/list' + (filesState.folderId ? '?folder=' + filesState.folderId : '')),
    ]);
    filesState.tree = tree || [];
    filesState.folders = list.folders || [];
    filesState.items = list.items || [];
  } catch (e) {
    // A stale folder id (deleted elsewhere) falls back to the root.
    if (filesState.folderId) { navigateTo('/files'); return; }
    toast(e.message, 'error');
    return;
  }
  filesRenderList();
  filesLoadUsage();
}

async function filesLoadUsage() {
  try { filesState.usage = await api('/api/storage/usage'); } catch (e) { return; }
  const el = document.getElementById('files-usage');
  if (!el || !filesState.usage) return;
  const u = filesState.usage;
  let s = `${u.count} · ${filesFmtSize(u.size)}`;
  if (u.disk_free) s += ` · ${esc(t('filesFree'))} ${filesFmtSize(u.disk_free)}`;
  el.textContent = s;
}

function filesFmtSize(n) {
  n = Number(n) || 0;
  if (n < 1024) return n + ' B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
  if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB';
  return (n / 1024 / 1024 / 1024).toFixed(1) + ' GB';
}

// filesCrumbs builds the breadcrumb chain from the flat tree.
function filesCrumbs() {
  const byId = {};
  for (const f of filesState.tree) byId[f.id] = f;
  const chain = [];
  let cur = filesState.folderId ? byId[filesState.folderId] : null;
  while (cur) { chain.unshift(cur); cur = cur.parent_id ? byId[cur.parent_id] : null; }
  return chain;
}

function filesRenderList() {
  const app = document.getElementById('app');
  const ro = isReadonly();
  const chain = filesCrumbs();
  const crumbs = [`<a onclick="navigateTo('/files')">&#128193; ${esc(t('filesRoot'))}</a>`]
    .concat(chain.map(f => `<a onclick="navigateTo('/files/folder/${f.id}')">${esc(f.name)}</a>`))
    .join('<span class="sep">/</span>');

  const folderRows = filesState.folders.map(f => `
    <div class="files-row">
      <span class="f-icon">&#128193;</span>
      <span class="f-name folder" onclick="navigateTo('/files/folder/${f.id}')">${esc(f.name)}</span>
      <span class="f-meta">${esc(fmtDate(f.created_at))}</span>
      ${ro ? '' : `<span class="f-actions">
        <button class="btn btn-ghost btn-sm" title="${escAttr(t('filesRename'))}" onclick="filesRenameFolder(${f.id})">&#9998;</button>
        <button class="btn btn-ghost btn-sm" title="${escAttr(t('filesMove'))}" onclick="filesMoveFolder(${f.id})">&#10132;</button>
        <button class="btn btn-ghost btn-sm" title="${escAttr(t('filesDelete'))}" onclick="filesDeleteFolder(${f.id})">&#128465;</button>
      </span>`}
    </div>`).join('');

  const itemRows = filesState.items.map(it => {
    const shared = it.public_token ? `<span class="f-shared" title="${escAttr(t('filesShared'))}">&#128279;</span>` : '';
    const canPreview = filesCanPreview(it.mime);
    return `
    <div class="files-row">
      <span class="f-icon">${filesIcon(it)}</span>
      <span class="f-name" ${canPreview ? `style="cursor:pointer" onclick="filesPreview(${it.id})"` : ''}>${esc(it.name)} ${shared}</span>
      <span class="f-meta">${filesFmtSize(it.size)} · ${esc(it.uploader_name || '')} · ${esc(fmtDate(it.created_at))}</span>
      <span class="f-actions">
        ${canPreview ? `<button class="btn btn-ghost btn-sm" title="${escAttr(t('filesPreview'))}" onclick="filesPreview(${it.id})">&#128065;</button>` : ''}
        <a class="btn btn-ghost btn-sm" title="${escAttr(t('filesDownload'))}" href="/api/storage/items/${it.id}/download">&#8681;</a>
        ${ro ? '' : `
        <button class="btn btn-ghost btn-sm ${it.public_token ? 'f-shared' : ''}" title="${escAttr(t('filesShare'))}" onclick="filesShare(${it.id})">&#128279;</button>
        <button class="btn btn-ghost btn-sm" title="${escAttr(t('filesRename'))}" onclick="filesRenameItem(${it.id})">&#9998;</button>
        <button class="btn btn-ghost btn-sm" title="${escAttr(t('filesMove'))}" onclick="filesMoveItem(${it.id})">&#10132;</button>
        <button class="btn btn-ghost btn-sm" title="${escAttr(t('filesDelete'))}" onclick="filesDeleteItem(${it.id})">&#128465;</button>`}
      </span>
    </div>`;
  }).join('');

  const rows = folderRows + itemRows;
  app.innerHTML = `
  <div class="files-wrap">
    <div class="files-crumbs">${crumbs}</div>
    <div class="files-list" id="files-list">
      ${rows || `<div class="files-empty">${esc(t('filesEmpty'))}${ro ? '' : '<br><small>' + esc(t('filesDropHint')) + '</small>'}</div>`}
    </div>
  </div>`;
  if (!ro) filesSetupDrop();
}

function filesIcon(it) {
  const m = it.mime || '';
  if (m.startsWith('image/')) return '&#128444;';
  if (m.startsWith('audio/')) return '&#127925;';
  if (m.startsWith('video/')) return '&#127916;';
  if (m.includes('zip') || m.includes('tar') || m.includes('compress') || m.includes('7z')) return '&#128230;';
  if (m.includes('pdf')) return '&#128209;';
  if (m.startsWith('text/') || m.includes('json')) return '&#128196;';
  return '&#128196;';
}

function filesCanPreview(mime) {
  const m = (mime || '').split(';')[0].trim();
  return ['image/png', 'image/jpeg', 'image/gif', 'image/webp',
    'text/plain', 'text/csv', 'text/markdown', 'application/json'].includes(m);
}

// === Actions ===
async function filesApi(promise) {
  try { await promise; return true; }
  catch (e) {
    toast(/taken|conflict/i.test(e.message || '') ? t('filesNameTaken') : e.message, 'error');
    return false;
  }
}

async function filesNewFolder() {
  const name = await uiPrompt(t('filesFolderName'), '');
  if (!name || !name.trim()) return;
  if (await filesApi(api('/api/storage/folders', 'POST', { name: name.trim(), parent_id: filesState.folderId || 0 }))) filesLoad();
}

async function filesRenameFolder(id) {
  const f = filesState.folders.find(x => x.id === id);
  const name = await uiPrompt(t('filesNewName'), f ? f.name : '');
  if (!name || !name.trim() || (f && name.trim() === f.name)) return;
  if (await filesApi(api(`/api/storage/folders/${id}/rename`, 'POST', { name: name.trim() }))) filesLoad();
}

async function filesDeleteFolder(id) {
  if (!await uiConfirm(t('filesDeleteFolderConfirm'))) return;
  if (await filesApi(api(`/api/storage/folders/${id}`, 'DELETE'))) filesLoad();
}

async function filesMoveFolder(id) {
  const dest = await filesPickFolder(id);
  if (dest === null) return;
  if (await filesApi(api(`/api/storage/folders/${id}/move`, 'POST', { folder_id: dest }))) filesLoad();
}

async function filesRenameItem(id) {
  const it = filesState.items.find(x => x.id === id);
  const name = await uiPrompt(t('filesNewName'), it ? it.name : '');
  if (!name || !name.trim() || (it && name.trim() === it.name)) return;
  if (await filesApi(api(`/api/storage/items/${id}/rename`, 'POST', { name: name.trim() }))) filesLoad();
}

async function filesDeleteItem(id) {
  if (!await uiConfirm(t('filesDeleteConfirm'))) return;
  if (await filesApi(api(`/api/storage/items/${id}`, 'DELETE'))) filesLoad();
}

async function filesMoveItem(id) {
  const dest = await filesPickFolder(0);
  if (dest === null) return;
  if (await filesApi(api(`/api/storage/items/${id}/move`, 'POST', { folder_id: dest }))) filesLoad();
}

// filesPickFolder shows a destination picker built from the tree; excludeId (a
// folder being moved) hides its own subtree. Resolves folder id (0 = root) or
// null on cancel.
function filesPickFolder(excludeId) {
  return new Promise(resolve => {
    const kids = {};
    for (const f of filesState.tree) (kids[f.parent_id || 0] = kids[f.parent_id || 0] || []).push(f);
    const opts = [{ id: 0, name: t('filesRoot'), depth: 0 }];
    (function walk(pid, depth) {
      for (const f of (kids[pid] || [])) {
        if (f.id === excludeId) continue;
        opts.push({ id: f.id, name: f.name, depth });
        walk(f.id, depth + 1);
      }
    })(0, 1);
    const ov = document.createElement('div');
    ov.className = 'modal-overlay';
    ov.innerHTML = `<div class="modal modal-narrow">
      <div class="ui-dialog-body">${esc(t('filesMoveTo'))}
        <select class="ui-dialog-input" id="files-move-select">
          ${opts.map(o => `<option value="${o.id}">${'&nbsp;'.repeat(o.depth * 3)}${o.depth ? '&#128193; ' : ''}${esc(o.name)}</option>`).join('')}
        </select>
      </div>
      <div class="ui-dialog-foot">
        <button class="btn btn-ghost btn-sm" id="files-move-cancel">${esc(t('cancel'))}</button>
        <button class="btn btn-primary btn-sm" id="files-move-ok">${esc(t('ok'))}</button>
      </div></div>`;
    document.body.appendChild(ov);
    let done = false;
    const close = v => { if (done) return; done = true; document.removeEventListener('keydown', onKey, true); ov.remove(); resolve(v); };
    function onKey(e) { if (e.key === 'Escape') { e.preventDefault(); close(null); } }
    document.addEventListener('keydown', onKey, true);
    ov.querySelector('#files-move-cancel').addEventListener('click', () => close(null));
    ov.querySelector('#files-move-ok').addEventListener('click', () => close(Number(ov.querySelector('#files-move-select').value)));
    ov.addEventListener('mousedown', e => { if (e.target === ov) close(null); });
  });
}

// === Share links ===
async function filesShare(id) {
  const it = filesState.items.find(x => x.id === id);
  if (it && it.public_token) {
    const choice = await uiChoice(t('filesShare'), [
      { label: t('filesCopyLink'), value: 'copy', primary: true },
      { label: t('filesRevoke'), value: 'revoke' },
    ]);
    if (choice === 'copy') {
      filesCopy(location.origin + '/api/storage/public/' + it.public_token);
      toast(t('filesLinkCopied'));
    } else if (choice === 'revoke') {
      if (await filesApi(api(`/api/storage/items/${id}/share`, 'DELETE'))) { toast(t('filesLinkRevoked')); filesLoad(); }
    }
    return;
  }
  try {
    const resp = await api(`/api/storage/items/${id}/share`, 'POST');
    filesCopy(location.origin + resp.url);
    toast(t('filesLinkCopied'));
    filesLoad();
  } catch (e) { toast(e.message, 'error'); }
}

function filesCopy(text) {
  if (navigator.clipboard && navigator.clipboard.writeText) { navigator.clipboard.writeText(text); return; }
  const ta = document.createElement('textarea');
  ta.value = text; document.body.appendChild(ta); ta.select();
  try { document.execCommand('copy'); } catch (e) { }
  ta.remove();
}

// === Preview ===
async function filesPreview(id) {
  const it = filesState.items.find(x => x.id === id);
  if (!it) return;
  const url = `/api/storage/items/${id}/preview`;
  if ((it.mime || '').startsWith('image/')) { openLightbox(url); return; }
  try {
    const r = await fetch(url);
    if (!r.ok) throw new Error(r.statusText);
    const text = await r.text();
    const ov = document.createElement('div');
    ov.className = 'modal-overlay';
    ov.innerHTML = `<div class="modal">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
        <strong>${esc(it.name)}</strong>
        <button class="btn btn-ghost btn-sm" onclick="this.closest('.modal-overlay').remove()">&#10005;</button>
      </div>
      <pre class="files-preview-pre">${esc(text)}</pre></div>`;
    ov.addEventListener('mousedown', e => { if (e.target === ov) ov.remove(); });
    document.body.appendChild(ov);
  } catch (e) { toast(e.message, 'error'); }
}

// === Upload ===
function filesUploadInput(input) {
  if (input.files && input.files.length) filesUploadMany([...input.files]);
  input.value = '';
}

async function filesUploadMany(fileList) {
  if (isReadonly()) return;
  filesState.uploading += fileList.length;
  toast(t('filesUploading'));
  for (const f of fileList) {
    try {
      const data = await filesReadBase64(f);
      await api('/api/storage/upload', 'POST', {
        data, filename: f.name, mime: f.type || 'application/octet-stream',
        folder_id: filesState.folderId || 0,
      });
    } catch (e) {
      toast(f.name + ': ' + e.message, 'error');
    } finally {
      filesState.uploading--;
    }
  }
  filesLoad();
}

function filesReadBase64(file) {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(String(r.result).split(',')[1] || '');
    r.onerror = () => reject(new Error('read error'));
    r.readAsDataURL(file);
  });
}

function filesSetupDrop() {
  const list = document.getElementById('files-list');
  if (!list) return;
  let depth = 0;
  list.addEventListener('dragenter', e => { e.preventDefault(); depth++; list.classList.add('files-drop'); });
  list.addEventListener('dragover', e => e.preventDefault());
  list.addEventListener('dragleave', () => { if (--depth <= 0) { depth = 0; list.classList.remove('files-drop'); } });
  list.addEventListener('drop', e => {
    e.preventDefault(); depth = 0; list.classList.remove('files-drop');
    if (e.dataTransfer && e.dataTransfer.files.length) filesUploadMany([...e.dataTransfer.files]);
  });
}

// === Live updates ===
function filesConnectSSE() {
  if (filesState.es) return;
  try {
    const es = new EventSource('/api/storage/events');
    es.onmessage = ev => {
      let data;
      try { data = JSON.parse(ev.data); } catch (e) { return; }
      // The SSE hub is shared across services; react to storage pings only, and
      // skip mid-upload refreshes (the batch reloads once at the end).
      if (data.type !== 'storage' || filesState.uploading > 0) return;
      if (currentView === 'files') filesLoad();
    };
    es.onerror = () => { es.close(); filesState.es = null; };
    filesState.es = es;
  } catch (e) { filesState.es = null; }
}
