// Mind-map page type for the wiki. Loaded after wiki.js; relies on the shared
// helpers of the main inline script (api, t, esc, escAttr, navigateTo, toast,
// uiPrompt, uiConfirm, uiChoice, fmtDate, displayName, LANGS).
//
// Structure of this file:
//   1. i18n            — MM_I18N merged into LANGS (10 languages)
//   2. pure helpers    — ids, outline text <-> tree, layout, edge paths, colours
//   3. MindMap         — the canvas engine (render, pan/zoom, keyboard, drag&drop,
//                        undo/redo, search, relationships, panels, export)
//   4. page glue       — wikiRenderMindmapPage(): toolbar, autosave, conflicts,
//                        drafts, SSE refresh, embeds in regular pages
/* global api, t, esc, escAttr, navigateTo, toast, uiPrompt, uiConfirm, uiChoice, fmtDate, displayName, LANGS, wikiState, wikiSpace, wikiCanEdit, wikiBreadcrumbs, wikiRenderCommentsSection, wikiToggleMenu, wikiNewPage, wikiDeletePage, wikiTogglePin, wikiToggleTemplate, wikiRoleIsAdmin, wikiMovePage, wikiIndentPage, wikiOutdentPage, currentUser, wikiMdToHtml, wikiSanitizePreview, MAX_UPLOAD_SIZE, compressImage */

// === 1. i18n ===
const MM_I18N = {
  en: {
    mmNewChild: 'Add child', mmNewSibling: 'Add sibling', mmDelete: 'Delete', mmUndo: 'Undo', mmRedo: 'Redo',
    mmLayout: 'Layout', mmLayoutBalanced: 'Mind map (both sides)', mmLayoutRight: 'Logic chart (right)', mmLayoutLeft: 'Logic chart (left)', mmLayoutDown: 'Org chart (down)', mmLayoutUp: 'Org chart (up)', mmLayoutTree: 'Tree (outline)',
    mmLine: 'Branches', mmLineCurve: 'Curved', mmLineElbow: 'Elbow', mmLineStraight: 'Straight',
    mmPalette: 'Colors', mmPaletteRainbow: 'Rainbow', mmPaletteMono: 'Monochrome', mmPaletteWarm: 'Warm', mmPaletteCool: 'Cool', mmPalettePastel: 'Pastel',
    mmSearch: 'Find in map (Ctrl+F)', mmOutline: 'Outline', mmProperties: 'Properties', mmZoomIn: 'Zoom in', mmZoomOut: 'Zoom out', mmFit: 'Fit to screen', mmCenter: 'Center root',
    mmExport: 'Export', mmImport: 'Import', mmHelp: 'Keyboard shortcuts', mmFullscreen: 'Full screen',
    mmSaved: 'Saved', mmSaving: 'Saving…', mmUnsaved: 'Unsaved changes', mmSaveError: 'Save failed', mmSaveVersion: 'Save version…', mmVersionComment: 'Version comment',
    mmConflict: 'This map was changed by someone else (v{n}). Reload to see their version or keep yours and overwrite.', mmReload: 'Reload', mmKeepMine: 'Keep mine', mmUpdatedBy: 'Updated by {u} — map reloaded', mmReadOnly: 'Read only',
    mmNodes: 'nodes', mmText: 'Text', mmNote: 'Note (Markdown)', mmLink: 'Hyperlink', mmWikiPage: 'Wiki page', mmTaskRef: 'Task (KEY-123)', mmIcon: 'Icon', mmColor: 'Branch color', mmInherit: 'Inherit', mmShape: 'Shape',
    mmShapeDefault: 'Rounded', mmShapeRect: 'Rectangle', mmShapePill: 'Pill', mmShapeUnderline: 'Underline', mmPriority: 'Priority', mmNone: 'None', mmProgress: 'Progress', mmTask: 'Checkbox (task)', mmTags: 'Tags', mmTagsHint: 'comma separated',
    mmImage: 'Image', mmRemoveImage: 'Remove image', mmBoundary: 'Boundary around subtree', mmSide: 'Side', mmSideAuto: 'Auto', mmSideLeft: 'Left', mmSideRight: 'Right',
    mmEdit: 'Edit text', mmInsertParent: 'Insert parent', mmCopy: 'Copy', mmCut: 'Cut', mmPaste: 'Paste', mmDuplicate: 'Duplicate', mmFold: 'Collapse', mmUnfold: 'Expand', mmFoldAll: 'Collapse all', mmUnfoldAll: 'Expand all', mmExpandLevel: 'Expand to level',
    mmAddRelation: 'Add relationship…', mmRelationHint: 'Click the target node (Esc to cancel)', mmRelationLabel: 'Relationship label', mmRemoveRelation: 'Remove relationship', mmCenterNode: 'Center on node',
    mmOpenPage: 'Open wiki page', mmOpenTask: 'Open task', mmOpenLink: 'Open link', mmEditAsText: 'Edit as text', mmApply: 'Apply', mmCancel: 'Cancel',
    mmOutlineHint: 'One node per line; indent with spaces or tabs. "- [x]", "(P1)", "(50%)", "#tag", "[text](url)" are understood.',
    mmDraftFound: 'An unsaved draft of this map was found. Restore it?', mmRestore: 'Restore', mmDiscard: 'Discard',
    mmImportTitle: 'Import mind map', mmImportHint: 'Markdown outline, plain text, OPML, FreeMind (.mm), XMind, Mermaid or JSON', mmImportMode: 'Mode', mmImportReplace: 'Replace this map', mmImportAppend: 'Append as a branch', mmImportFile: 'Choose file…', mmImportPaste: 'or paste an outline', mmImportDone: 'Imported {n} nodes',
    mmExportSVG: 'SVG image', mmExportPNG: 'PNG image', mmExportMd: 'Markdown outline', mmExportOPML: 'OPML', mmExportFreeMind: 'FreeMind / Freeplane (.mm)', mmExportXMind: 'XMind', mmExportMermaid: 'Mermaid', mmExportJSON: 'JSON', mmExportTxt: 'Plain text', mmPrint: 'Print',
    mmShortcuts: 'Keyboard shortcuts', mmKeyNav: 'Move between nodes', mmKeyChild: 'Add child', mmKeySibling: 'Add sibling below / above', mmKeyParent: 'Insert parent', mmKeyEdit: 'Edit text (or just start typing)', mmKeyNewline: 'New line while editing', mmKeyDelete: 'Delete node and subtree',
    mmKeyFold: 'Collapse / expand', mmKeyMove: 'Move among siblings', mmKeyUndo: 'Undo / redo', mmKeyCopy: 'Copy / cut / paste subtree (text outlines paste as nodes)', mmKeyDup: 'Duplicate', mmKeyAll: 'Select all', mmKeyZoom: 'Zoom', mmKeyFit: 'Fit map / zoom to selection / 100%', mmKeyFind: 'Find', mmKeyMulti: 'Multi-select / rubber band', mmKeyPan: 'Pan (drag empty space, or Space+drag)', mmKeyPriority: 'Priority 1–6 / clear', mmKeyTask: 'Toggle checkbox', mmKeyProps: 'Properties', mmKeySave: 'Save a named version', mmKeyHelp: 'This help',
    mmNoMatches: 'No matches', mmMatchOf: '{i} of {n}', mmNewMindmap: 'New mind map', mmPageType: 'What do you want to create?', mmPageTypePage: 'Page', mmPageTypeMindmap: 'Mind map', mmPageTypeImport: 'Import mind map file',
    mmEmbedOpen: 'Open mind map', mmLinkTo: 'Search pages…', mmNoResults: 'No results', mmRootHint: 'Press Tab to add a child, double-click to edit', mmBadDoc: 'The map could not be loaded', mmTreeIcon: 'Mind map',
    mmSelectedN: '{n} selected', mmVersion: 'v{n}', mmApplied: 'Outline applied', mmSizeWarning: 'Large map: animations off',
  },
  ru: {
    mmNewChild: 'Дочерний узел', mmNewSibling: 'Соседний узел', mmDelete: 'Удалить', mmUndo: 'Отменить', mmRedo: 'Повторить',
    mmLayout: 'Раскладка', mmLayoutBalanced: 'Карта (в обе стороны)', mmLayoutRight: 'Логическая (вправо)', mmLayoutLeft: 'Логическая (влево)', mmLayoutDown: 'Оргсхема (вниз)', mmLayoutUp: 'Оргсхема (вверх)', mmLayoutTree: 'Дерево (список)',
    mmLine: 'Ветви', mmLineCurve: 'Плавные', mmLineElbow: 'Угловые', mmLineStraight: 'Прямые',
    mmPalette: 'Цвета', mmPaletteRainbow: 'Радуга', mmPaletteMono: 'Монохром', mmPaletteWarm: 'Тёплые', mmPaletteCool: 'Холодные', mmPalettePastel: 'Пастель',
    mmSearch: 'Поиск по карте (Ctrl+F)', mmOutline: 'Структура', mmProperties: 'Свойства', mmZoomIn: 'Приблизить', mmZoomOut: 'Отдалить', mmFit: 'Вписать в экран', mmCenter: 'К центру',
    mmExport: 'Экспорт', mmImport: 'Импорт', mmHelp: 'Горячие клавиши', mmFullscreen: 'Во весь экран',
    mmSaved: 'Сохранено', mmSaving: 'Сохранение…', mmUnsaved: 'Есть несохранённые изменения', mmSaveError: 'Ошибка сохранения', mmSaveVersion: 'Сохранить версию…', mmVersionComment: 'Комментарий к версии',
    mmConflict: 'Карту изменил кто-то другой (v{n}). Перезагрузите, чтобы увидеть его версию, или оставьте свою и перезапишите.', mmReload: 'Перезагрузить', mmKeepMine: 'Оставить мою', mmUpdatedBy: 'Обновил(а) {u} — карта перезагружена', mmReadOnly: 'Только чтение',
    mmNodes: 'узлов', mmText: 'Текст', mmNote: 'Заметка (Markdown)', mmLink: 'Ссылка', mmWikiPage: 'Страница вики', mmTaskRef: 'Задача (KEY-123)', mmIcon: 'Иконка', mmColor: 'Цвет ветви', mmInherit: 'Наследовать', mmShape: 'Форма',
    mmShapeDefault: 'Скруглённая', mmShapeRect: 'Прямоугольник', mmShapePill: 'Капсула', mmShapeUnderline: 'Подчёркивание', mmPriority: 'Приоритет', mmNone: 'Нет', mmProgress: 'Прогресс', mmTask: 'Чекбокс (задача)', mmTags: 'Теги', mmTagsHint: 'через запятую',
    mmImage: 'Изображение', mmRemoveImage: 'Убрать изображение', mmBoundary: 'Граница вокруг поддерева', mmSide: 'Сторона', mmSideAuto: 'Авто', mmSideLeft: 'Слева', mmSideRight: 'Справа',
    mmEdit: 'Редактировать текст', mmInsertParent: 'Вставить родителя', mmCopy: 'Копировать', mmCut: 'Вырезать', mmPaste: 'Вставить', mmDuplicate: 'Дублировать', mmFold: 'Свернуть', mmUnfold: 'Развернуть', mmFoldAll: 'Свернуть всё', mmUnfoldAll: 'Развернуть всё', mmExpandLevel: 'Развернуть до уровня',
    mmAddRelation: 'Добавить связь…', mmRelationHint: 'Кликните по целевому узлу (Esc — отмена)', mmRelationLabel: 'Подпись связи', mmRemoveRelation: 'Удалить связь', mmCenterNode: 'Центрировать узел',
    mmOpenPage: 'Открыть страницу вики', mmOpenTask: 'Открыть задачу', mmOpenLink: 'Открыть ссылку', mmEditAsText: 'Редактировать как текст', mmApply: 'Применить', mmCancel: 'Отмена',
    mmOutlineHint: 'Один узел на строку; вложенность — отступами (пробелы или табуляция). Понимаются «- [x]», «(P1)», «(50%)», «#тег», «[текст](url)».',
    mmDraftFound: 'Найден несохранённый черновик этой карты. Восстановить?', mmRestore: 'Восстановить', mmDiscard: 'Отбросить',
    mmImportTitle: 'Импорт карты', mmImportHint: 'Markdown-структура, текст, OPML, FreeMind (.mm), XMind, Mermaid или JSON', mmImportMode: 'Режим', mmImportReplace: 'Заменить карту', mmImportAppend: 'Добавить как ветвь', mmImportFile: 'Выбрать файл…', mmImportPaste: 'или вставьте структуру', mmImportDone: 'Импортировано узлов: {n}',
    mmExportSVG: 'Картинка SVG', mmExportPNG: 'Картинка PNG', mmExportMd: 'Структура Markdown', mmExportOPML: 'OPML', mmExportFreeMind: 'FreeMind / Freeplane (.mm)', mmExportXMind: 'XMind', mmExportMermaid: 'Mermaid', mmExportJSON: 'JSON', mmExportTxt: 'Простой текст', mmPrint: 'Печать',
    mmShortcuts: 'Горячие клавиши', mmKeyNav: 'Переход между узлами', mmKeyChild: 'Дочерний узел', mmKeySibling: 'Соседний узел ниже / выше', mmKeyParent: 'Вставить родителя', mmKeyEdit: 'Редактировать текст (или просто начните печатать)', mmKeyNewline: 'Перенос строки при редактировании', mmKeyDelete: 'Удалить узел с поддеревом',
    mmKeyFold: 'Свернуть / развернуть', mmKeyMove: 'Переместить среди соседей', mmKeyUndo: 'Отменить / повторить', mmKeyCopy: 'Копировать / вырезать / вставить поддерево (текст вставляется как узлы)', mmKeyDup: 'Дублировать', mmKeyAll: 'Выделить всё', mmKeyZoom: 'Масштаб', mmKeyFit: 'Вписать карту / к выделению / 100%', mmKeyFind: 'Поиск', mmKeyMulti: 'Множественное выделение / рамка', mmKeyPan: 'Перемещение (тянуть пустое место или Space+тянуть)', mmKeyPriority: 'Приоритет 1–6 / сброс', mmKeyTask: 'Переключить чекбокс', mmKeyProps: 'Свойства', mmKeySave: 'Сохранить именованную версию', mmKeyHelp: 'Эта справка',
    mmNoMatches: 'Нет совпадений', mmMatchOf: '{i} из {n}', mmNewMindmap: 'Новая карта', mmPageType: 'Что создать?', mmPageTypePage: 'Страницу', mmPageTypeMindmap: 'Интеллект-карту', mmPageTypeImport: 'Импортировать файл карты',
    mmEmbedOpen: 'Открыть карту', mmLinkTo: 'Поиск страниц…', mmNoResults: 'Ничего не найдено', mmRootHint: 'Tab — добавить дочерний узел, двойной клик — редактировать', mmBadDoc: 'Не удалось загрузить карту', mmTreeIcon: 'Интеллект-карта',
    mmSelectedN: 'Выделено: {n}', mmVersion: 'v{n}', mmApplied: 'Структура применена', mmSizeWarning: 'Большая карта: анимация отключена',
  },
  de: { mmNewChild: 'Unterknoten', mmNewSibling: 'Nachbarknoten', mmDelete: 'Löschen', mmUndo: 'Rückgängig', mmRedo: 'Wiederholen', mmLayout: 'Layout', mmLayoutBalanced: 'Mindmap (beidseitig)', mmLayoutRight: 'Logik (rechts)', mmLayoutLeft: 'Logik (links)', mmLayoutDown: 'Organigramm (unten)', mmLayoutUp: 'Organigramm (oben)', mmLayoutTree: 'Baum (Gliederung)', mmLine: 'Äste', mmLineCurve: 'Gebogen', mmLineElbow: 'Abgewinkelt', mmLineStraight: 'Gerade', mmPalette: 'Farben', mmPaletteRainbow: 'Regenbogen', mmPaletteMono: 'Einfarbig', mmPaletteWarm: 'Warm', mmPaletteCool: 'Kühl', mmPalettePastel: 'Pastell', mmSearch: 'In Karte suchen (Strg+F)', mmOutline: 'Gliederung', mmProperties: 'Eigenschaften', mmZoomIn: 'Vergrößern', mmZoomOut: 'Verkleinern', mmFit: 'Einpassen', mmCenter: 'Zentrieren', mmExport: 'Export', mmImport: 'Import', mmHelp: 'Tastenkürzel', mmFullscreen: 'Vollbild', mmSaved: 'Gespeichert', mmSaving: 'Speichern…', mmUnsaved: 'Ungespeicherte Änderungen', mmSaveError: 'Speichern fehlgeschlagen', mmSaveVersion: 'Version speichern…', mmVersionComment: 'Versionskommentar', mmConflict: 'Die Karte wurde von jemand anderem geändert (v{n}). Neu laden oder eigene Version behalten.', mmReload: 'Neu laden', mmKeepMine: 'Meine behalten', mmUpdatedBy: 'Aktualisiert von {u} — Karte neu geladen', mmReadOnly: 'Nur lesen', mmNodes: 'Knoten', mmText: 'Text', mmNote: 'Notiz (Markdown)', mmLink: 'Hyperlink', mmWikiPage: 'Wiki-Seite', mmTaskRef: 'Aufgabe (KEY-123)', mmIcon: 'Symbol', mmColor: 'Astfarbe', mmInherit: 'Erben', mmShape: 'Form', mmShapeDefault: 'Abgerundet', mmShapeRect: 'Rechteck', mmShapePill: 'Pille', mmShapeUnderline: 'Unterstrichen', mmPriority: 'Priorität', mmNone: 'Keine', mmProgress: 'Fortschritt', mmTask: 'Checkbox (Aufgabe)', mmTags: 'Tags', mmTagsHint: 'kommagetrennt', mmImage: 'Bild', mmRemoveImage: 'Bild entfernen', mmBoundary: 'Rahmen um Teilbaum', mmSide: 'Seite', mmSideAuto: 'Auto', mmSideLeft: 'Links', mmSideRight: 'Rechts', mmEdit: 'Text bearbeiten', mmInsertParent: 'Elternknoten einfügen', mmCopy: 'Kopieren', mmCut: 'Ausschneiden', mmPaste: 'Einfügen', mmDuplicate: 'Duplizieren', mmFold: 'Einklappen', mmUnfold: 'Ausklappen', mmFoldAll: 'Alle einklappen', mmUnfoldAll: 'Alle ausklappen', mmExpandLevel: 'Bis Ebene ausklappen', mmAddRelation: 'Beziehung hinzufügen…', mmRelationHint: 'Zielknoten anklicken (Esc = Abbruch)', mmRelationLabel: 'Beziehungsbeschriftung', mmRemoveRelation: 'Beziehung entfernen', mmCenterNode: 'Auf Knoten zentrieren', mmOpenPage: 'Wiki-Seite öffnen', mmOpenTask: 'Aufgabe öffnen', mmOpenLink: 'Link öffnen', mmEditAsText: 'Als Text bearbeiten', mmApply: 'Anwenden', mmCancel: 'Abbrechen', mmOutlineHint: 'Ein Knoten pro Zeile; Einrückung mit Leerzeichen oder Tabs. „- [x]“, „(P1)“, „(50%)“, „#tag“, „[Text](url)“ werden erkannt.', mmDraftFound: 'Ein ungespeicherter Entwurf wurde gefunden. Wiederherstellen?', mmRestore: 'Wiederherstellen', mmDiscard: 'Verwerfen', mmImportTitle: 'Mindmap importieren', mmImportHint: 'Markdown-Gliederung, Text, OPML, FreeMind (.mm), XMind, Mermaid oder JSON', mmImportMode: 'Modus', mmImportReplace: 'Karte ersetzen', mmImportAppend: 'Als Ast anhängen', mmImportFile: 'Datei wählen…', mmImportPaste: 'oder Gliederung einfügen', mmImportDone: '{n} Knoten importiert', mmExportSVG: 'SVG-Bild', mmExportPNG: 'PNG-Bild', mmExportMd: 'Markdown-Gliederung', mmExportOPML: 'OPML', mmExportFreeMind: 'FreeMind / Freeplane (.mm)', mmExportXMind: 'XMind', mmExportMermaid: 'Mermaid', mmExportJSON: 'JSON', mmExportTxt: 'Reiner Text', mmPrint: 'Drucken', mmShortcuts: 'Tastenkürzel', mmKeyNav: 'Zwischen Knoten wechseln', mmKeyChild: 'Unterknoten', mmKeySibling: 'Nachbar unten / oben', mmKeyParent: 'Elternknoten einfügen', mmKeyEdit: 'Text bearbeiten (oder einfach tippen)', mmKeyNewline: 'Zeilenumbruch beim Bearbeiten', mmKeyDelete: 'Knoten mit Teilbaum löschen', mmKeyFold: 'Ein-/ausklappen', mmKeyMove: 'Unter Nachbarn verschieben', mmKeyUndo: 'Rückgängig / wiederholen', mmKeyCopy: 'Teilbaum kopieren / ausschneiden / einfügen', mmKeyDup: 'Duplizieren', mmKeyAll: 'Alles auswählen', mmKeyZoom: 'Zoom', mmKeyFit: 'Einpassen / Auswahl / 100 %', mmKeyFind: 'Suchen', mmKeyMulti: 'Mehrfachauswahl / Rahmen', mmKeyPan: 'Verschieben (leere Fläche ziehen oder Leertaste+Ziehen)', mmKeyPriority: 'Priorität 1–6 / löschen', mmKeyTask: 'Checkbox umschalten', mmKeyProps: 'Eigenschaften', mmKeySave: 'Benannte Version speichern', mmKeyHelp: 'Diese Hilfe', mmNoMatches: 'Keine Treffer', mmMatchOf: '{i} von {n}', mmNewMindmap: 'Neue Mindmap', mmPageType: 'Was möchten Sie erstellen?', mmPageTypePage: 'Seite', mmPageTypeMindmap: 'Mindmap', mmPageTypeImport: 'Mindmap-Datei importieren', mmEmbedOpen: 'Mindmap öffnen', mmLinkTo: 'Seiten suchen…', mmNoResults: 'Keine Ergebnisse', mmRootHint: 'Tab für Unterknoten, Doppelklick zum Bearbeiten', mmBadDoc: 'Karte konnte nicht geladen werden', mmTreeIcon: 'Mindmap', mmSelectedN: '{n} ausgewählt', mmVersion: 'v{n}', mmApplied: 'Gliederung angewendet', mmSizeWarning: 'Große Karte: Animationen aus' },
  es: { mmNewChild: 'Nodo hijo', mmNewSibling: 'Nodo hermano', mmDelete: 'Eliminar', mmUndo: 'Deshacer', mmRedo: 'Rehacer', mmLayout: 'Diseño', mmLayoutBalanced: 'Mapa (ambos lados)', mmLayoutRight: 'Lógico (derecha)', mmLayoutLeft: 'Lógico (izquierda)', mmLayoutDown: 'Organigrama (abajo)', mmLayoutUp: 'Organigrama (arriba)', mmLayoutTree: 'Árbol (esquema)', mmLine: 'Ramas', mmLineCurve: 'Curvas', mmLineElbow: 'Codo', mmLineStraight: 'Rectas', mmPalette: 'Colores', mmPaletteRainbow: 'Arcoíris', mmPaletteMono: 'Monocromo', mmPaletteWarm: 'Cálidos', mmPaletteCool: 'Fríos', mmPalettePastel: 'Pastel', mmSearch: 'Buscar en el mapa (Ctrl+F)', mmOutline: 'Esquema', mmProperties: 'Propiedades', mmZoomIn: 'Acercar', mmZoomOut: 'Alejar', mmFit: 'Ajustar a pantalla', mmCenter: 'Centrar', mmExport: 'Exportar', mmImport: 'Importar', mmHelp: 'Atajos de teclado', mmFullscreen: 'Pantalla completa', mmSaved: 'Guardado', mmSaving: 'Guardando…', mmUnsaved: 'Cambios sin guardar', mmSaveError: 'Error al guardar', mmSaveVersion: 'Guardar versión…', mmVersionComment: 'Comentario de la versión', mmConflict: 'Otra persona cambió este mapa (v{n}). Recarga para ver su versión o conserva la tuya.', mmReload: 'Recargar', mmKeepMine: 'Conservar la mía', mmUpdatedBy: 'Actualizado por {u} — mapa recargado', mmReadOnly: 'Solo lectura', mmNodes: 'nodos', mmText: 'Texto', mmNote: 'Nota (Markdown)', mmLink: 'Enlace', mmWikiPage: 'Página wiki', mmTaskRef: 'Tarea (KEY-123)', mmIcon: 'Icono', mmColor: 'Color de rama', mmInherit: 'Heredar', mmShape: 'Forma', mmShapeDefault: 'Redondeada', mmShapeRect: 'Rectángulo', mmShapePill: 'Píldora', mmShapeUnderline: 'Subrayado', mmPriority: 'Prioridad', mmNone: 'Ninguna', mmProgress: 'Progreso', mmTask: 'Casilla (tarea)', mmTags: 'Etiquetas', mmTagsHint: 'separadas por comas', mmImage: 'Imagen', mmRemoveImage: 'Quitar imagen', mmBoundary: 'Borde alrededor del subárbol', mmSide: 'Lado', mmSideAuto: 'Auto', mmSideLeft: 'Izquierda', mmSideRight: 'Derecha', mmEdit: 'Editar texto', mmInsertParent: 'Insertar padre', mmCopy: 'Copiar', mmCut: 'Cortar', mmPaste: 'Pegar', mmDuplicate: 'Duplicar', mmFold: 'Contraer', mmUnfold: 'Expandir', mmFoldAll: 'Contraer todo', mmUnfoldAll: 'Expandir todo', mmExpandLevel: 'Expandir hasta nivel', mmAddRelation: 'Añadir relación…', mmRelationHint: 'Haz clic en el nodo destino (Esc para cancelar)', mmRelationLabel: 'Etiqueta de la relación', mmRemoveRelation: 'Quitar relación', mmCenterNode: 'Centrar en el nodo', mmOpenPage: 'Abrir página wiki', mmOpenTask: 'Abrir tarea', mmOpenLink: 'Abrir enlace', mmEditAsText: 'Editar como texto', mmApply: 'Aplicar', mmCancel: 'Cancelar', mmOutlineHint: 'Un nodo por línea; sangría con espacios o tabuladores. Se reconocen "- [x]", "(P1)", "(50%)", "#etiqueta", "[texto](url)".', mmDraftFound: 'Se encontró un borrador sin guardar. ¿Restaurarlo?', mmRestore: 'Restaurar', mmDiscard: 'Descartar', mmImportTitle: 'Importar mapa mental', mmImportHint: 'Esquema Markdown, texto, OPML, FreeMind (.mm), XMind, Mermaid o JSON', mmImportMode: 'Modo', mmImportReplace: 'Reemplazar el mapa', mmImportAppend: 'Añadir como rama', mmImportFile: 'Elegir archivo…', mmImportPaste: 'o pega un esquema', mmImportDone: '{n} nodos importados', mmExportSVG: 'Imagen SVG', mmExportPNG: 'Imagen PNG', mmExportMd: 'Esquema Markdown', mmExportOPML: 'OPML', mmExportFreeMind: 'FreeMind / Freeplane (.mm)', mmExportXMind: 'XMind', mmExportMermaid: 'Mermaid', mmExportJSON: 'JSON', mmExportTxt: 'Texto plano', mmPrint: 'Imprimir', mmShortcuts: 'Atajos de teclado', mmKeyNav: 'Moverse entre nodos', mmKeyChild: 'Nodo hijo', mmKeySibling: 'Hermano abajo / arriba', mmKeyParent: 'Insertar padre', mmKeyEdit: 'Editar texto (o empieza a escribir)', mmKeyNewline: 'Nueva línea al editar', mmKeyDelete: 'Eliminar nodo y subárbol', mmKeyFold: 'Contraer / expandir', mmKeyMove: 'Mover entre hermanos', mmKeyUndo: 'Deshacer / rehacer', mmKeyCopy: 'Copiar / cortar / pegar subárbol', mmKeyDup: 'Duplicar', mmKeyAll: 'Seleccionar todo', mmKeyZoom: 'Zoom', mmKeyFit: 'Ajustar / selección / 100 %', mmKeyFind: 'Buscar', mmKeyMulti: 'Selección múltiple / rectángulo', mmKeyPan: 'Desplazar (arrastrar espacio vacío o Espacio+arrastrar)', mmKeyPriority: 'Prioridad 1–6 / quitar', mmKeyTask: 'Alternar casilla', mmKeyProps: 'Propiedades', mmKeySave: 'Guardar versión con nombre', mmKeyHelp: 'Esta ayuda', mmNoMatches: 'Sin coincidencias', mmMatchOf: '{i} de {n}', mmNewMindmap: 'Nuevo mapa mental', mmPageType: '¿Qué quieres crear?', mmPageTypePage: 'Página', mmPageTypeMindmap: 'Mapa mental', mmPageTypeImport: 'Importar archivo de mapa', mmEmbedOpen: 'Abrir mapa', mmLinkTo: 'Buscar páginas…', mmNoResults: 'Sin resultados', mmRootHint: 'Tab para añadir hijo, doble clic para editar', mmBadDoc: 'No se pudo cargar el mapa', mmTreeIcon: 'Mapa mental', mmSelectedN: '{n} seleccionados', mmVersion: 'v{n}', mmApplied: 'Esquema aplicado', mmSizeWarning: 'Mapa grande: animaciones desactivadas' },
  fr: { mmNewChild: 'Nœud enfant', mmNewSibling: 'Nœud frère', mmDelete: 'Supprimer', mmUndo: 'Annuler', mmRedo: 'Rétablir', mmLayout: 'Disposition', mmLayoutBalanced: 'Carte (deux côtés)', mmLayoutRight: 'Logique (droite)', mmLayoutLeft: 'Logique (gauche)', mmLayoutDown: 'Organigramme (bas)', mmLayoutUp: 'Organigramme (haut)', mmLayoutTree: 'Arbre (plan)', mmLine: 'Branches', mmLineCurve: 'Courbes', mmLineElbow: 'Coudées', mmLineStraight: 'Droites', mmPalette: 'Couleurs', mmPaletteRainbow: 'Arc-en-ciel', mmPaletteMono: 'Monochrome', mmPaletteWarm: 'Chaudes', mmPaletteCool: 'Froides', mmPalettePastel: 'Pastel', mmSearch: 'Rechercher (Ctrl+F)', mmOutline: 'Plan', mmProperties: 'Propriétés', mmZoomIn: 'Zoom avant', mmZoomOut: 'Zoom arrière', mmFit: 'Ajuster à l’écran', mmCenter: 'Centrer', mmExport: 'Exporter', mmImport: 'Importer', mmHelp: 'Raccourcis clavier', mmFullscreen: 'Plein écran', mmSaved: 'Enregistré', mmSaving: 'Enregistrement…', mmUnsaved: 'Modifications non enregistrées', mmSaveError: 'Échec de l’enregistrement', mmSaveVersion: 'Enregistrer une version…', mmVersionComment: 'Commentaire de version', mmConflict: 'Cette carte a été modifiée par quelqu’un d’autre (v{n}). Rechargez ou conservez la vôtre.', mmReload: 'Recharger', mmKeepMine: 'Garder la mienne', mmUpdatedBy: 'Mis à jour par {u} — carte rechargée', mmReadOnly: 'Lecture seule', mmNodes: 'nœuds', mmText: 'Texte', mmNote: 'Note (Markdown)', mmLink: 'Lien', mmWikiPage: 'Page wiki', mmTaskRef: 'Tâche (KEY-123)', mmIcon: 'Icône', mmColor: 'Couleur de branche', mmInherit: 'Hériter', mmShape: 'Forme', mmShapeDefault: 'Arrondie', mmShapeRect: 'Rectangle', mmShapePill: 'Pilule', mmShapeUnderline: 'Souligné', mmPriority: 'Priorité', mmNone: 'Aucune', mmProgress: 'Progression', mmTask: 'Case à cocher (tâche)', mmTags: 'Tags', mmTagsHint: 'séparés par des virgules', mmImage: 'Image', mmRemoveImage: 'Retirer l’image', mmBoundary: 'Contour du sous-arbre', mmSide: 'Côté', mmSideAuto: 'Auto', mmSideLeft: 'Gauche', mmSideRight: 'Droite', mmEdit: 'Modifier le texte', mmInsertParent: 'Insérer un parent', mmCopy: 'Copier', mmCut: 'Couper', mmPaste: 'Coller', mmDuplicate: 'Dupliquer', mmFold: 'Replier', mmUnfold: 'Déplier', mmFoldAll: 'Tout replier', mmUnfoldAll: 'Tout déplier', mmExpandLevel: 'Déplier jusqu’au niveau', mmAddRelation: 'Ajouter une relation…', mmRelationHint: 'Cliquez sur le nœud cible (Échap pour annuler)', mmRelationLabel: 'Libellé de la relation', mmRemoveRelation: 'Supprimer la relation', mmCenterNode: 'Centrer sur le nœud', mmOpenPage: 'Ouvrir la page wiki', mmOpenTask: 'Ouvrir la tâche', mmOpenLink: 'Ouvrir le lien', mmEditAsText: 'Modifier en texte', mmApply: 'Appliquer', mmCancel: 'Annuler', mmOutlineHint: 'Un nœud par ligne ; indentation par espaces ou tabulations. « - [x] », « (P1) », « (50%) », « #tag », « [texte](url) » sont compris.', mmDraftFound: 'Un brouillon non enregistré a été trouvé. Le restaurer ?', mmRestore: 'Restaurer', mmDiscard: 'Abandonner', mmImportTitle: 'Importer une carte mentale', mmImportHint: 'Plan Markdown, texte, OPML, FreeMind (.mm), XMind, Mermaid ou JSON', mmImportMode: 'Mode', mmImportReplace: 'Remplacer la carte', mmImportAppend: 'Ajouter comme branche', mmImportFile: 'Choisir un fichier…', mmImportPaste: 'ou collez un plan', mmImportDone: '{n} nœuds importés', mmExportSVG: 'Image SVG', mmExportPNG: 'Image PNG', mmExportMd: 'Plan Markdown', mmExportOPML: 'OPML', mmExportFreeMind: 'FreeMind / Freeplane (.mm)', mmExportXMind: 'XMind', mmExportMermaid: 'Mermaid', mmExportJSON: 'JSON', mmExportTxt: 'Texte brut', mmPrint: 'Imprimer', mmShortcuts: 'Raccourcis clavier', mmKeyNav: 'Naviguer entre les nœuds', mmKeyChild: 'Nœud enfant', mmKeySibling: 'Frère dessous / dessus', mmKeyParent: 'Insérer un parent', mmKeyEdit: 'Modifier le texte (ou commencez à taper)', mmKeyNewline: 'Nouvelle ligne en édition', mmKeyDelete: 'Supprimer le nœud et son sous-arbre', mmKeyFold: 'Replier / déplier', mmKeyMove: 'Déplacer parmi les frères', mmKeyUndo: 'Annuler / rétablir', mmKeyCopy: 'Copier / couper / coller un sous-arbre', mmKeyDup: 'Dupliquer', mmKeyAll: 'Tout sélectionner', mmKeyZoom: 'Zoom', mmKeyFit: 'Ajuster / sélection / 100 %', mmKeyFind: 'Rechercher', mmKeyMulti: 'Sélection multiple / rectangle', mmKeyPan: 'Déplacer (glisser le vide ou Espace+glisser)', mmKeyPriority: 'Priorité 1–6 / effacer', mmKeyTask: 'Basculer la case', mmKeyProps: 'Propriétés', mmKeySave: 'Enregistrer une version nommée', mmKeyHelp: 'Cette aide', mmNoMatches: 'Aucun résultat', mmMatchOf: '{i} sur {n}', mmNewMindmap: 'Nouvelle carte mentale', mmPageType: 'Que voulez-vous créer ?', mmPageTypePage: 'Page', mmPageTypeMindmap: 'Carte mentale', mmPageTypeImport: 'Importer un fichier de carte', mmEmbedOpen: 'Ouvrir la carte', mmLinkTo: 'Rechercher des pages…', mmNoResults: 'Aucun résultat', mmRootHint: 'Tab pour ajouter un enfant, double-clic pour modifier', mmBadDoc: 'Impossible de charger la carte', mmTreeIcon: 'Carte mentale', mmSelectedN: '{n} sélectionnés', mmVersion: 'v{n}', mmApplied: 'Plan appliqué', mmSizeWarning: 'Grande carte : animations désactivées' },
  pt: { mmNewChild: 'Nó filho', mmNewSibling: 'Nó irmão', mmDelete: 'Excluir', mmUndo: 'Desfazer', mmRedo: 'Refazer', mmLayout: 'Layout', mmLayoutBalanced: 'Mapa (ambos os lados)', mmLayoutRight: 'Lógico (direita)', mmLayoutLeft: 'Lógico (esquerda)', mmLayoutDown: 'Organograma (baixo)', mmLayoutUp: 'Organograma (cima)', mmLayoutTree: 'Árvore (tópicos)', mmLine: 'Ramos', mmLineCurve: 'Curvos', mmLineElbow: 'Cotovelo', mmLineStraight: 'Retos', mmPalette: 'Cores', mmPaletteRainbow: 'Arco-íris', mmPaletteMono: 'Monocromático', mmPaletteWarm: 'Quentes', mmPaletteCool: 'Frias', mmPalettePastel: 'Pastel', mmSearch: 'Buscar no mapa (Ctrl+F)', mmOutline: 'Estrutura', mmProperties: 'Propriedades', mmZoomIn: 'Aproximar', mmZoomOut: 'Afastar', mmFit: 'Ajustar à tela', mmCenter: 'Centralizar', mmExport: 'Exportar', mmImport: 'Importar', mmHelp: 'Atalhos de teclado', mmFullscreen: 'Tela cheia', mmSaved: 'Salvo', mmSaving: 'Salvando…', mmUnsaved: 'Alterações não salvas', mmSaveError: 'Falha ao salvar', mmSaveVersion: 'Salvar versão…', mmVersionComment: 'Comentário da versão', mmConflict: 'Este mapa foi alterado por outra pessoa (v{n}). Recarregue ou mantenha o seu.', mmReload: 'Recarregar', mmKeepMine: 'Manter o meu', mmUpdatedBy: 'Atualizado por {u} — mapa recarregado', mmReadOnly: 'Somente leitura', mmNodes: 'nós', mmText: 'Texto', mmNote: 'Nota (Markdown)', mmLink: 'Link', mmWikiPage: 'Página wiki', mmTaskRef: 'Tarefa (KEY-123)', mmIcon: 'Ícone', mmColor: 'Cor do ramo', mmInherit: 'Herdar', mmShape: 'Forma', mmShapeDefault: 'Arredondada', mmShapeRect: 'Retângulo', mmShapePill: 'Pílula', mmShapeUnderline: 'Sublinhado', mmPriority: 'Prioridade', mmNone: 'Nenhuma', mmProgress: 'Progresso', mmTask: 'Caixa de seleção (tarefa)', mmTags: 'Tags', mmTagsHint: 'separadas por vírgula', mmImage: 'Imagem', mmRemoveImage: 'Remover imagem', mmBoundary: 'Borda ao redor da subárvore', mmSide: 'Lado', mmSideAuto: 'Auto', mmSideLeft: 'Esquerda', mmSideRight: 'Direita', mmEdit: 'Editar texto', mmInsertParent: 'Inserir pai', mmCopy: 'Copiar', mmCut: 'Recortar', mmPaste: 'Colar', mmDuplicate: 'Duplicar', mmFold: 'Recolher', mmUnfold: 'Expandir', mmFoldAll: 'Recolher tudo', mmUnfoldAll: 'Expandir tudo', mmExpandLevel: 'Expandir até o nível', mmAddRelation: 'Adicionar relação…', mmRelationHint: 'Clique no nó de destino (Esc para cancelar)', mmRelationLabel: 'Rótulo da relação', mmRemoveRelation: 'Remover relação', mmCenterNode: 'Centralizar no nó', mmOpenPage: 'Abrir página wiki', mmOpenTask: 'Abrir tarefa', mmOpenLink: 'Abrir link', mmEditAsText: 'Editar como texto', mmApply: 'Aplicar', mmCancel: 'Cancelar', mmOutlineHint: 'Um nó por linha; recuo com espaços ou tabs. "- [x]", "(P1)", "(50%)", "#tag", "[texto](url)" são reconhecidos.', mmDraftFound: 'Um rascunho não salvo foi encontrado. Restaurar?', mmRestore: 'Restaurar', mmDiscard: 'Descartar', mmImportTitle: 'Importar mapa mental', mmImportHint: 'Estrutura Markdown, texto, OPML, FreeMind (.mm), XMind, Mermaid ou JSON', mmImportMode: 'Modo', mmImportReplace: 'Substituir o mapa', mmImportAppend: 'Adicionar como ramo', mmImportFile: 'Escolher arquivo…', mmImportPaste: 'ou cole uma estrutura', mmImportDone: '{n} nós importados', mmExportSVG: 'Imagem SVG', mmExportPNG: 'Imagem PNG', mmExportMd: 'Estrutura Markdown', mmExportOPML: 'OPML', mmExportFreeMind: 'FreeMind / Freeplane (.mm)', mmExportXMind: 'XMind', mmExportMermaid: 'Mermaid', mmExportJSON: 'JSON', mmExportTxt: 'Texto simples', mmPrint: 'Imprimir', mmShortcuts: 'Atalhos de teclado', mmKeyNav: 'Mover entre nós', mmKeyChild: 'Nó filho', mmKeySibling: 'Irmão abaixo / acima', mmKeyParent: 'Inserir pai', mmKeyEdit: 'Editar texto (ou comece a digitar)', mmKeyNewline: 'Nova linha ao editar', mmKeyDelete: 'Excluir nó e subárvore', mmKeyFold: 'Recolher / expandir', mmKeyMove: 'Mover entre irmãos', mmKeyUndo: 'Desfazer / refazer', mmKeyCopy: 'Copiar / recortar / colar subárvore', mmKeyDup: 'Duplicar', mmKeyAll: 'Selecionar tudo', mmKeyZoom: 'Zoom', mmKeyFit: 'Ajustar / seleção / 100%', mmKeyFind: 'Buscar', mmKeyMulti: 'Seleção múltipla / retângulo', mmKeyPan: 'Mover (arrastar área vazia ou Espaço+arrastar)', mmKeyPriority: 'Prioridade 1–6 / limpar', mmKeyTask: 'Alternar caixa', mmKeyProps: 'Propriedades', mmKeySave: 'Salvar versão nomeada', mmKeyHelp: 'Esta ajuda', mmNoMatches: 'Sem correspondências', mmMatchOf: '{i} de {n}', mmNewMindmap: 'Novo mapa mental', mmPageType: 'O que deseja criar?', mmPageTypePage: 'Página', mmPageTypeMindmap: 'Mapa mental', mmPageTypeImport: 'Importar arquivo de mapa', mmEmbedOpen: 'Abrir mapa', mmLinkTo: 'Buscar páginas…', mmNoResults: 'Sem resultados', mmRootHint: 'Tab para adicionar filho, duplo clique para editar', mmBadDoc: 'Não foi possível carregar o mapa', mmTreeIcon: 'Mapa mental', mmSelectedN: '{n} selecionados', mmVersion: 'v{n}', mmApplied: 'Estrutura aplicada', mmSizeWarning: 'Mapa grande: animações desativadas' },
  zh: { mmNewChild: '添加子节点', mmNewSibling: '添加同级节点', mmDelete: '删除', mmUndo: '撤销', mmRedo: '重做', mmLayout: '布局', mmLayoutBalanced: '思维导图（双侧）', mmLayoutRight: '逻辑图（右）', mmLayoutLeft: '逻辑图（左）', mmLayoutDown: '组织结构图（下）', mmLayoutUp: '组织结构图（上）', mmLayoutTree: '树形（大纲）', mmLine: '分支线', mmLineCurve: '曲线', mmLineElbow: '折线', mmLineStraight: '直线', mmPalette: '配色', mmPaletteRainbow: '彩虹', mmPaletteMono: '单色', mmPaletteWarm: '暖色', mmPaletteCool: '冷色', mmPalettePastel: '柔和', mmSearch: '在图中查找 (Ctrl+F)', mmOutline: '大纲', mmProperties: '属性', mmZoomIn: '放大', mmZoomOut: '缩小', mmFit: '适应屏幕', mmCenter: '居中', mmExport: '导出', mmImport: '导入', mmHelp: '快捷键', mmFullscreen: '全屏', mmSaved: '已保存', mmSaving: '保存中…', mmUnsaved: '有未保存的更改', mmSaveError: '保存失败', mmSaveVersion: '保存版本…', mmVersionComment: '版本备注', mmConflict: '此图已被他人修改 (v{n})。重新加载查看对方版本，或保留并覆盖。', mmReload: '重新加载', mmKeepMine: '保留我的', mmUpdatedBy: '{u} 已更新 — 图已重新加载', mmReadOnly: '只读', mmNodes: '个节点', mmText: '文本', mmNote: '备注 (Markdown)', mmLink: '超链接', mmWikiPage: 'Wiki 页面', mmTaskRef: '任务 (KEY-123)', mmIcon: '图标', mmColor: '分支颜色', mmInherit: '继承', mmShape: '形状', mmShapeDefault: '圆角', mmShapeRect: '矩形', mmShapePill: '胶囊', mmShapeUnderline: '下划线', mmPriority: '优先级', mmNone: '无', mmProgress: '进度', mmTask: '复选框（任务）', mmTags: '标签', mmTagsHint: '逗号分隔', mmImage: '图片', mmRemoveImage: '移除图片', mmBoundary: '子树外框', mmSide: '方向', mmSideAuto: '自动', mmSideLeft: '左', mmSideRight: '右', mmEdit: '编辑文本', mmInsertParent: '插入父节点', mmCopy: '复制', mmCut: '剪切', mmPaste: '粘贴', mmDuplicate: '创建副本', mmFold: '折叠', mmUnfold: '展开', mmFoldAll: '全部折叠', mmUnfoldAll: '全部展开', mmExpandLevel: '展开到层级', mmAddRelation: '添加关联…', mmRelationHint: '点击目标节点（Esc 取消）', mmRelationLabel: '关联标签', mmRemoveRelation: '删除关联', mmCenterNode: '定位到节点', mmOpenPage: '打开 Wiki 页面', mmOpenTask: '打开任务', mmOpenLink: '打开链接', mmEditAsText: '以文本编辑', mmApply: '应用', mmCancel: '取消', mmOutlineHint: '每行一个节点，用空格或制表符缩进。支持 "- [x]"、"(P1)"、"(50%)"、"#标签"、"[文本](url)"。', mmDraftFound: '发现未保存的草稿，是否恢复？', mmRestore: '恢复', mmDiscard: '丢弃', mmImportTitle: '导入思维导图', mmImportHint: 'Markdown 大纲、纯文本、OPML、FreeMind (.mm)、XMind、Mermaid 或 JSON', mmImportMode: '模式', mmImportReplace: '替换此图', mmImportAppend: '作为分支追加', mmImportFile: '选择文件…', mmImportPaste: '或粘贴大纲', mmImportDone: '已导入 {n} 个节点', mmExportSVG: 'SVG 图片', mmExportPNG: 'PNG 图片', mmExportMd: 'Markdown 大纲', mmExportOPML: 'OPML', mmExportFreeMind: 'FreeMind / Freeplane (.mm)', mmExportXMind: 'XMind', mmExportMermaid: 'Mermaid', mmExportJSON: 'JSON', mmExportTxt: '纯文本', mmPrint: '打印', mmShortcuts: '快捷键', mmKeyNav: '在节点间移动', mmKeyChild: '添加子节点', mmKeySibling: '在下方 / 上方添加同级', mmKeyParent: '插入父节点', mmKeyEdit: '编辑文本（或直接输入）', mmKeyNewline: '编辑时换行', mmKeyDelete: '删除节点及子树', mmKeyFold: '折叠 / 展开', mmKeyMove: '在同级间移动', mmKeyUndo: '撤销 / 重做', mmKeyCopy: '复制 / 剪切 / 粘贴子树', mmKeyDup: '创建副本', mmKeyAll: '全选', mmKeyZoom: '缩放', mmKeyFit: '适应 / 选区 / 100%', mmKeyFind: '查找', mmKeyMulti: '多选 / 框选', mmKeyPan: '平移（拖动空白处或 空格+拖动）', mmKeyPriority: '优先级 1–6 / 清除', mmKeyTask: '切换复选框', mmKeyProps: '属性', mmKeySave: '保存命名版本', mmKeyHelp: '本帮助', mmNoMatches: '无匹配', mmMatchOf: '{i} / {n}', mmNewMindmap: '新建思维导图', mmPageType: '要创建什么？', mmPageTypePage: '页面', mmPageTypeMindmap: '思维导图', mmPageTypeImport: '导入导图文件', mmEmbedOpen: '打开导图', mmLinkTo: '搜索页面…', mmNoResults: '无结果', mmRootHint: '按 Tab 添加子节点，双击编辑', mmBadDoc: '无法加载导图', mmTreeIcon: '思维导图', mmSelectedN: '已选 {n} 项', mmVersion: 'v{n}', mmApplied: '大纲已应用', mmSizeWarning: '导图较大：已关闭动画' },
  ja: { mmNewChild: '子ノード', mmNewSibling: '兄弟ノード', mmDelete: '削除', mmUndo: '元に戻す', mmRedo: 'やり直す', mmLayout: 'レイアウト', mmLayoutBalanced: 'マインドマップ（両側）', mmLayoutRight: 'ロジック（右）', mmLayoutLeft: 'ロジック（左）', mmLayoutDown: '組織図（下）', mmLayoutUp: '組織図（上）', mmLayoutTree: 'ツリー（アウトライン）', mmLine: '枝線', mmLineCurve: '曲線', mmLineElbow: '折れ線', mmLineStraight: '直線', mmPalette: '配色', mmPaletteRainbow: 'レインボー', mmPaletteMono: 'モノクロ', mmPaletteWarm: '暖色', mmPaletteCool: '寒色', mmPalettePastel: 'パステル', mmSearch: 'マップ内を検索 (Ctrl+F)', mmOutline: 'アウトライン', mmProperties: 'プロパティ', mmZoomIn: '拡大', mmZoomOut: '縮小', mmFit: '画面に合わせる', mmCenter: '中央へ', mmExport: 'エクスポート', mmImport: 'インポート', mmHelp: 'ショートカット', mmFullscreen: '全画面', mmSaved: '保存済み', mmSaving: '保存中…', mmUnsaved: '未保存の変更', mmSaveError: '保存に失敗', mmSaveVersion: 'バージョンを保存…', mmVersionComment: 'バージョンのコメント', mmConflict: 'このマップは他の人に変更されました (v{n})。再読み込みするか、自分の版を保持して上書きします。', mmReload: '再読み込み', mmKeepMine: '自分の版を保持', mmUpdatedBy: '{u} が更新 — マップを再読み込みしました', mmReadOnly: '読み取り専用', mmNodes: 'ノード', mmText: 'テキスト', mmNote: 'ノート (Markdown)', mmLink: 'リンク', mmWikiPage: 'Wiki ページ', mmTaskRef: 'タスク (KEY-123)', mmIcon: 'アイコン', mmColor: '枝の色', mmInherit: '継承', mmShape: '形', mmShapeDefault: '角丸', mmShapeRect: '長方形', mmShapePill: 'ピル', mmShapeUnderline: '下線', mmPriority: '優先度', mmNone: 'なし', mmProgress: '進捗', mmTask: 'チェックボックス（タスク）', mmTags: 'タグ', mmTagsHint: 'カンマ区切り', mmImage: '画像', mmRemoveImage: '画像を削除', mmBoundary: 'サブツリーの囲み', mmSide: '配置側', mmSideAuto: '自動', mmSideLeft: '左', mmSideRight: '右', mmEdit: 'テキストを編集', mmInsertParent: '親を挿入', mmCopy: 'コピー', mmCut: '切り取り', mmPaste: '貼り付け', mmDuplicate: '複製', mmFold: '折りたたむ', mmUnfold: '展開', mmFoldAll: 'すべて折りたたむ', mmUnfoldAll: 'すべて展開', mmExpandLevel: 'レベルまで展開', mmAddRelation: '関連を追加…', mmRelationHint: '対象ノードをクリック（Esc でキャンセル）', mmRelationLabel: '関連のラベル', mmRemoveRelation: '関連を削除', mmCenterNode: 'ノードを中央へ', mmOpenPage: 'Wiki ページを開く', mmOpenTask: 'タスクを開く', mmOpenLink: 'リンクを開く', mmEditAsText: 'テキストとして編集', mmApply: '適用', mmCancel: 'キャンセル', mmOutlineHint: '1 行 1 ノード。スペースかタブで字下げ。"- [x]"、"(P1)"、"(50%)"、"#タグ"、"[テキスト](url)" を認識します。', mmDraftFound: '未保存の下書きがあります。復元しますか？', mmRestore: '復元', mmDiscard: '破棄', mmImportTitle: 'マインドマップをインポート', mmImportHint: 'Markdown アウトライン、テキスト、OPML、FreeMind (.mm)、XMind、Mermaid、JSON', mmImportMode: 'モード', mmImportReplace: 'このマップを置き換え', mmImportAppend: '枝として追加', mmImportFile: 'ファイルを選択…', mmImportPaste: 'またはアウトラインを貼り付け', mmImportDone: '{n} ノードをインポートしました', mmExportSVG: 'SVG 画像', mmExportPNG: 'PNG 画像', mmExportMd: 'Markdown アウトライン', mmExportOPML: 'OPML', mmExportFreeMind: 'FreeMind / Freeplane (.mm)', mmExportXMind: 'XMind', mmExportMermaid: 'Mermaid', mmExportJSON: 'JSON', mmExportTxt: 'プレーンテキスト', mmPrint: '印刷', mmShortcuts: 'キーボードショートカット', mmKeyNav: 'ノード間を移動', mmKeyChild: '子ノード', mmKeySibling: '下 / 上に兄弟を追加', mmKeyParent: '親を挿入', mmKeyEdit: 'テキストを編集（入力を始めても可）', mmKeyNewline: '編集中の改行', mmKeyDelete: 'ノードとサブツリーを削除', mmKeyFold: '折りたたみ / 展開', mmKeyMove: '兄弟間で移動', mmKeyUndo: '元に戻す / やり直す', mmKeyCopy: 'サブツリーをコピー / 切り取り / 貼り付け', mmKeyDup: '複製', mmKeyAll: 'すべて選択', mmKeyZoom: 'ズーム', mmKeyFit: '全体 / 選択範囲 / 100%', mmKeyFind: '検索', mmKeyMulti: '複数選択 / 範囲選択', mmKeyPan: 'パン（空白をドラッグ、または Space+ドラッグ）', mmKeyPriority: '優先度 1–6 / 解除', mmKeyTask: 'チェックを切り替え', mmKeyProps: 'プロパティ', mmKeySave: '名前付きバージョンを保存', mmKeyHelp: 'このヘルプ', mmNoMatches: '一致なし', mmMatchOf: '{i} / {n}', mmNewMindmap: '新しいマインドマップ', mmPageType: '何を作成しますか？', mmPageTypePage: 'ページ', mmPageTypeMindmap: 'マインドマップ', mmPageTypeImport: 'マップファイルをインポート', mmEmbedOpen: 'マップを開く', mmLinkTo: 'ページを検索…', mmNoResults: '結果なし', mmRootHint: 'Tab で子を追加、ダブルクリックで編集', mmBadDoc: 'マップを読み込めません', mmTreeIcon: 'マインドマップ', mmSelectedN: '{n} 件選択', mmVersion: 'v{n}', mmApplied: 'アウトラインを適用', mmSizeWarning: '大きなマップ: アニメーション無効' },
  ko: { mmNewChild: '하위 노드', mmNewSibling: '형제 노드', mmDelete: '삭제', mmUndo: '실행 취소', mmRedo: '다시 실행', mmLayout: '레이아웃', mmLayoutBalanced: '마인드맵(양쪽)', mmLayoutRight: '로직 차트(오른쪽)', mmLayoutLeft: '로직 차트(왼쪽)', mmLayoutDown: '조직도(아래)', mmLayoutUp: '조직도(위)', mmLayoutTree: '트리(개요)', mmLine: '가지', mmLineCurve: '곡선', mmLineElbow: '꺾인 선', mmLineStraight: '직선', mmPalette: '색상', mmPaletteRainbow: '무지개', mmPaletteMono: '단색', mmPaletteWarm: '따뜻한', mmPaletteCool: '차가운', mmPalettePastel: '파스텔', mmSearch: '맵에서 찾기 (Ctrl+F)', mmOutline: '개요', mmProperties: '속성', mmZoomIn: '확대', mmZoomOut: '축소', mmFit: '화면에 맞춤', mmCenter: '가운데로', mmExport: '내보내기', mmImport: '가져오기', mmHelp: '단축키', mmFullscreen: '전체 화면', mmSaved: '저장됨', mmSaving: '저장 중…', mmUnsaved: '저장되지 않은 변경', mmSaveError: '저장 실패', mmSaveVersion: '버전 저장…', mmVersionComment: '버전 설명', mmConflict: '다른 사용자가 이 맵을 변경했습니다 (v{n}). 다시 불러오거나 내 버전을 유지해 덮어씁니다.', mmReload: '다시 불러오기', mmKeepMine: '내 버전 유지', mmUpdatedBy: '{u} 님이 업데이트 — 맵을 다시 불러왔습니다', mmReadOnly: '읽기 전용', mmNodes: '노드', mmText: '텍스트', mmNote: '메모 (Markdown)', mmLink: '하이퍼링크', mmWikiPage: '위키 페이지', mmTaskRef: '작업 (KEY-123)', mmIcon: '아이콘', mmColor: '가지 색', mmInherit: '상속', mmShape: '모양', mmShapeDefault: '둥근', mmShapeRect: '사각형', mmShapePill: '알약', mmShapeUnderline: '밑줄', mmPriority: '우선순위', mmNone: '없음', mmProgress: '진행률', mmTask: '체크박스(작업)', mmTags: '태그', mmTagsHint: '쉼표로 구분', mmImage: '이미지', mmRemoveImage: '이미지 제거', mmBoundary: '하위 트리 테두리', mmSide: '방향', mmSideAuto: '자동', mmSideLeft: '왼쪽', mmSideRight: '오른쪽', mmEdit: '텍스트 편집', mmInsertParent: '상위 노드 삽입', mmCopy: '복사', mmCut: '잘라내기', mmPaste: '붙여넣기', mmDuplicate: '복제', mmFold: '접기', mmUnfold: '펼치기', mmFoldAll: '모두 접기', mmUnfoldAll: '모두 펼치기', mmExpandLevel: '레벨까지 펼치기', mmAddRelation: '관계 추가…', mmRelationHint: '대상 노드를 클릭하세요 (Esc 취소)', mmRelationLabel: '관계 레이블', mmRemoveRelation: '관계 제거', mmCenterNode: '노드로 이동', mmOpenPage: '위키 페이지 열기', mmOpenTask: '작업 열기', mmOpenLink: '링크 열기', mmEditAsText: '텍스트로 편집', mmApply: '적용', mmCancel: '취소', mmOutlineHint: '한 줄에 노드 하나, 공백이나 탭으로 들여쓰기. "- [x]", "(P1)", "(50%)", "#태그", "[텍스트](url)"를 인식합니다.', mmDraftFound: '저장되지 않은 초안이 있습니다. 복원할까요?', mmRestore: '복원', mmDiscard: '버리기', mmImportTitle: '마인드맵 가져오기', mmImportHint: 'Markdown 개요, 텍스트, OPML, FreeMind (.mm), XMind, Mermaid 또는 JSON', mmImportMode: '모드', mmImportReplace: '이 맵 교체', mmImportAppend: '가지로 추가', mmImportFile: '파일 선택…', mmImportPaste: '또는 개요 붙여넣기', mmImportDone: '{n}개 노드를 가져왔습니다', mmExportSVG: 'SVG 이미지', mmExportPNG: 'PNG 이미지', mmExportMd: 'Markdown 개요', mmExportOPML: 'OPML', mmExportFreeMind: 'FreeMind / Freeplane (.mm)', mmExportXMind: 'XMind', mmExportMermaid: 'Mermaid', mmExportJSON: 'JSON', mmExportTxt: '일반 텍스트', mmPrint: '인쇄', mmShortcuts: '키보드 단축키', mmKeyNav: '노드 간 이동', mmKeyChild: '하위 노드', mmKeySibling: '아래 / 위에 형제 추가', mmKeyParent: '상위 노드 삽입', mmKeyEdit: '텍스트 편집(바로 입력해도 됨)', mmKeyNewline: '편집 중 줄바꿈', mmKeyDelete: '노드와 하위 트리 삭제', mmKeyFold: '접기 / 펼치기', mmKeyMove: '형제 사이에서 이동', mmKeyUndo: '실행 취소 / 다시 실행', mmKeyCopy: '하위 트리 복사 / 잘라내기 / 붙여넣기', mmKeyDup: '복제', mmKeyAll: '모두 선택', mmKeyZoom: '확대/축소', mmKeyFit: '전체 / 선택 / 100%', mmKeyFind: '찾기', mmKeyMulti: '다중 선택 / 영역 선택', mmKeyPan: '이동(빈 곳 드래그 또는 Space+드래그)', mmKeyPriority: '우선순위 1–6 / 해제', mmKeyTask: '체크박스 전환', mmKeyProps: '속성', mmKeySave: '이름 있는 버전 저장', mmKeyHelp: '이 도움말', mmNoMatches: '일치 항목 없음', mmMatchOf: '{n} 중 {i}', mmNewMindmap: '새 마인드맵', mmPageType: '무엇을 만들까요?', mmPageTypePage: '페이지', mmPageTypeMindmap: '마인드맵', mmPageTypeImport: '맵 파일 가져오기', mmEmbedOpen: '맵 열기', mmLinkTo: '페이지 검색…', mmNoResults: '결과 없음', mmRootHint: 'Tab으로 하위 노드 추가, 더블클릭으로 편집', mmBadDoc: '맵을 불러올 수 없습니다', mmTreeIcon: '마인드맵', mmSelectedN: '{n}개 선택됨', mmVersion: 'v{n}', mmApplied: '개요 적용됨', mmSizeWarning: '큰 맵: 애니메이션 꺼짐' },
  ar: { mmNewChild: 'عقدة فرعية', mmNewSibling: 'عقدة شقيقة', mmDelete: 'حذف', mmUndo: 'تراجع', mmRedo: 'إعادة', mmLayout: 'التخطيط', mmLayoutBalanced: 'خريطة ذهنية (الجانبان)', mmLayoutRight: 'مخطط منطقي (يمين)', mmLayoutLeft: 'مخطط منطقي (يسار)', mmLayoutDown: 'هيكل تنظيمي (أسفل)', mmLayoutUp: 'هيكل تنظيمي (أعلى)', mmLayoutTree: 'شجرة (مخطط)', mmLine: 'الفروع', mmLineCurve: 'منحنية', mmLineElbow: 'زاوية', mmLineStraight: 'مستقيمة', mmPalette: 'الألوان', mmPaletteRainbow: 'قوس قزح', mmPaletteMono: 'أحادي', mmPaletteWarm: 'دافئة', mmPaletteCool: 'باردة', mmPalettePastel: 'باستيل', mmSearch: 'بحث في الخريطة (Ctrl+F)', mmOutline: 'المخطط', mmProperties: 'الخصائص', mmZoomIn: 'تكبير', mmZoomOut: 'تصغير', mmFit: 'ملاءمة الشاشة', mmCenter: 'توسيط', mmExport: 'تصدير', mmImport: 'استيراد', mmHelp: 'اختصارات لوحة المفاتيح', mmFullscreen: 'ملء الشاشة', mmSaved: 'تم الحفظ', mmSaving: 'جارٍ الحفظ…', mmUnsaved: 'تغييرات غير محفوظة', mmSaveError: 'فشل الحفظ', mmSaveVersion: 'حفظ إصدار…', mmVersionComment: 'تعليق الإصدار', mmConflict: 'غيّر شخص آخر هذه الخريطة (v{n}). أعد التحميل لرؤية نسخته أو احتفظ بنسختك.', mmReload: 'إعادة التحميل', mmKeepMine: 'الاحتفاظ بنسختي', mmUpdatedBy: 'حدّثها {u} — أُعيد تحميل الخريطة', mmReadOnly: 'للقراءة فقط', mmNodes: 'عقد', mmText: 'النص', mmNote: 'ملاحظة (Markdown)', mmLink: 'رابط', mmWikiPage: 'صفحة ويكي', mmTaskRef: 'مهمة (KEY-123)', mmIcon: 'أيقونة', mmColor: 'لون الفرع', mmInherit: 'وراثة', mmShape: 'الشكل', mmShapeDefault: 'مستدير', mmShapeRect: 'مستطيل', mmShapePill: 'كبسولة', mmShapeUnderline: 'تسطير', mmPriority: 'الأولوية', mmNone: 'لا شيء', mmProgress: 'التقدم', mmTask: 'مربع اختيار (مهمة)', mmTags: 'وسوم', mmTagsHint: 'مفصولة بفواصل', mmImage: 'صورة', mmRemoveImage: 'إزالة الصورة', mmBoundary: 'إطار حول الشجرة الفرعية', mmSide: 'الجانب', mmSideAuto: 'تلقائي', mmSideLeft: 'يسار', mmSideRight: 'يمين', mmEdit: 'تحرير النص', mmInsertParent: 'إدراج أصل', mmCopy: 'نسخ', mmCut: 'قص', mmPaste: 'لصق', mmDuplicate: 'تكرار', mmFold: 'طي', mmUnfold: 'توسيع', mmFoldAll: 'طي الكل', mmUnfoldAll: 'توسيع الكل', mmExpandLevel: 'توسيع حتى المستوى', mmAddRelation: 'إضافة علاقة…', mmRelationHint: 'انقر على العقدة الهدف (Esc للإلغاء)', mmRelationLabel: 'تسمية العلاقة', mmRemoveRelation: 'إزالة العلاقة', mmCenterNode: 'توسيط العقدة', mmOpenPage: 'فتح صفحة الويكي', mmOpenTask: 'فتح المهمة', mmOpenLink: 'فتح الرابط', mmEditAsText: 'تحرير كنص', mmApply: 'تطبيق', mmCancel: 'إلغاء', mmOutlineHint: 'عقدة في كل سطر؛ المسافة البادئة بالمسافات أو Tab. يُفهم "- [x]" و"(P1)" و"(50%)" و"#وسم" و"[نص](url)".', mmDraftFound: 'عُثر على مسودة غير محفوظة. استعادتها؟', mmRestore: 'استعادة', mmDiscard: 'تجاهل', mmImportTitle: 'استيراد خريطة ذهنية', mmImportHint: 'مخطط Markdown أو نص أو OPML أو FreeMind (.mm) أو XMind أو Mermaid أو JSON', mmImportMode: 'الوضع', mmImportReplace: 'استبدال الخريطة', mmImportAppend: 'إضافة كفرع', mmImportFile: 'اختيار ملف…', mmImportPaste: 'أو الصق مخططًا', mmImportDone: 'تم استيراد {n} عقدة', mmExportSVG: 'صورة SVG', mmExportPNG: 'صورة PNG', mmExportMd: 'مخطط Markdown', mmExportOPML: 'OPML', mmExportFreeMind: 'FreeMind / Freeplane (.mm)', mmExportXMind: 'XMind', mmExportMermaid: 'Mermaid', mmExportJSON: 'JSON', mmExportTxt: 'نص عادي', mmPrint: 'طباعة', mmShortcuts: 'اختصارات لوحة المفاتيح', mmKeyNav: 'التنقل بين العقد', mmKeyChild: 'عقدة فرعية', mmKeySibling: 'شقيقة أسفل / أعلى', mmKeyParent: 'إدراج أصل', mmKeyEdit: 'تحرير النص (أو ابدأ الكتابة)', mmKeyNewline: 'سطر جديد أثناء التحرير', mmKeyDelete: 'حذف العقدة وفروعها', mmKeyFold: 'طي / توسيع', mmKeyMove: 'نقل بين الأشقاء', mmKeyUndo: 'تراجع / إعادة', mmKeyCopy: 'نسخ / قص / لصق شجرة فرعية', mmKeyDup: 'تكرار', mmKeyAll: 'تحديد الكل', mmKeyZoom: 'تكبير/تصغير', mmKeyFit: 'ملاءمة / التحديد / 100%', mmKeyFind: 'بحث', mmKeyMulti: 'تحديد متعدد / إطار', mmKeyPan: 'تحريك (اسحب الفراغ أو Space+سحب)', mmKeyPriority: 'أولوية 1–6 / مسح', mmKeyTask: 'تبديل مربع الاختيار', mmKeyProps: 'الخصائص', mmKeySave: 'حفظ إصدار مسمّى', mmKeyHelp: 'هذه المساعدة', mmNoMatches: 'لا نتائج', mmMatchOf: '{i} من {n}', mmNewMindmap: 'خريطة ذهنية جديدة', mmPageType: 'ماذا تريد أن تنشئ؟', mmPageTypePage: 'صفحة', mmPageTypeMindmap: 'خريطة ذهنية', mmPageTypeImport: 'استيراد ملف خريطة', mmEmbedOpen: 'فتح الخريطة', mmLinkTo: 'بحث عن صفحات…', mmNoResults: 'لا نتائج', mmRootHint: 'Tab لإضافة فرع، نقر مزدوج للتحرير', mmBadDoc: 'تعذر تحميل الخريطة', mmTreeIcon: 'خريطة ذهنية', mmSelectedN: 'محدد: {n}', mmVersion: 'v{n}', mmApplied: 'تم تطبيق المخطط', mmSizeWarning: 'خريطة كبيرة: الرسوم المتحركة متوقفة' },
};
let mmI18nMerged = false;
function ensureMindmapI18n() {
  if (mmI18nMerged || typeof LANGS === 'undefined') return;
  for (const lang of Object.keys(LANGS)) {
    const dict = Object.assign({}, MM_I18N.en, MM_I18N[lang] || {});
    for (const k in dict) if (!(k in LANGS[lang])) LANGS[lang][k] = dict[k];
  }
  mmI18nMerged = true;
}
// mmT: t() with {placeholders}.
function mmT(key, vars) {
  ensureMindmapI18n();
  let s = t(key);
  if (vars) for (const k in vars) s = s.replace('{' + k + '}', vars[k]);
  return s;
}

// === 2. Pure helpers ===
const MM_PALETTES = {
  rainbow: ['#e74c3c', '#e67e22', '#f1c40f', '#2ecc71', '#1abc9c', '#3498db', '#9b59b6', '#e84393'],
  warm: ['#e74c3c', '#e67e22', '#f39c12', '#d35400', '#c0392b', '#ff7675'],
  cool: ['#3498db', '#1abc9c', '#2980b9', '#16a085', '#9b59b6', '#0984e3'],
  pastel: ['#ef9a9a', '#ffcc80', '#a5d6a7', '#81d4fa', '#b39ddb', '#f48fb1', '#80cbc4'],
  mono: null, // resolved to the theme accent at render time
};
const MM_LAYOUTS = ['balanced', 'right', 'left', 'down', 'up', 'tree'];
const MM_LINES = ['curve', 'elbow', 'straight'];
const MM_SHAPES = ['', 'rect', 'pill', 'underline'];
const MM_ICONS = ['⭐', '🚩', '✅', '❌', '❓', '❗', '⚠️', '💡', '🔥', '📌', '🎯', '🚀', '💰', '📅', '⏰', '📎', '🔗', '📝', '📊', '🧩', '🛠️', '🐞', '🔒', '👤', '👥', '💬', '❤️', '👍', '👎', '🟢', '🟡', '🔴', '🔵', '🟣', '1️⃣', '2️⃣', '3️⃣'];
const MM_GAP_X = 56, MM_GAP_Y = 14, MM_GAP_Y_DOWN = 48, MM_GAP_X_DOWN = 18, MM_TREE_INDENT = 34;
const MM_MAX_W = 280, MM_ANIM_MS = 170, MM_ANIM_MAX_NODES = 600, MM_CULL_FROM = 500;

function mmId() {
  const a = 'abcdefghijklmnopqrstuvwxyz0123456789';
  let s = '';
  for (let i = 0; i < 8; i++) s += a[Math.floor(Math.random() * a.length)];
  return s;
}
function mmClone(o) { return JSON.parse(JSON.stringify(o)); }
function mmReducedMotion() { return window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches; }
function mmCssVar(name, fallback) {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}
function mmFirstLine(s) { return String(s || '').split('\n')[0]; }

// mmNormalizeDoc fills in ids / arrays so the engine can rely on them (the
// server does the authoritative validation).
function mmNormalizeDoc(doc) {
  doc = doc && typeof doc === 'object' ? doc : {};
  if (!doc.root || typeof doc.root !== 'object') doc.root = { text: '' };
  doc.version = 1;
  if (!MM_LAYOUTS.includes(doc.layout)) doc.layout = 'balanced';
  if (!MM_LINES.includes(doc.line)) doc.line = 'curve';
  if (!(doc.palette in MM_PALETTES)) doc.palette = 'rainbow';
  if (!Array.isArray(doc.links)) doc.links = [];
  const seen = new Set();
  const walk = (n, isRoot) => {
    if (!n.id || seen.has(n.id)) n.id = mmId();
    seen.add(n.id);
    if (typeof n.text !== 'string') n.text = n.text == null ? '' : String(n.text);
    if (!Array.isArray(n.children)) n.children = [];
    if (isRoot) { n.side = ''; n.collapsed = false; }
    n.children = n.children.filter(c => c && typeof c === 'object');
    n.children.forEach(c => walk(c, false));
  };
  walk(doc.root, true);
  doc.links = doc.links.filter(l => l && seen.has(l.from) && seen.has(l.to) && l.from !== l.to).map(l => ({ id: l.id || mmId(), from: l.from, to: l.to, label: l.label || '' }));
  return doc;
}

// --- Outline text <-> nodes (mirrors the server's Markdown parser) ---
function mmNodeLine(n) {
  const parts = [];
  if (n.done === true) parts.push('[x]'); else if (n.done === false) parts.push('[ ]');
  if (n.icon) parts.push(n.icon);
  let text = (n.text || '').replace(/\n/g, ' ');
  if (n.url) text = '[' + text + '](' + n.url + ')';
  parts.push(text);
  if (n.priority) parts.push('(P' + n.priority + ')');
  if (n.progress != null) parts.push('(' + n.progress + '%)');
  (n.tags || []).forEach(tg => parts.push('#' + String(tg).replace(/\s+/g, '_')));
  if (n.task_ref) parts.push('[[' + n.task_ref + ']]');
  return parts.join(' ');
}
// mmOutlineText renders nodes (an array of subtrees) as a markdown list.
function mmOutlineText(nodes, depth) {
  depth = depth || 0;
  let out = '';
  for (const n of nodes) {
    const ind = '  '.repeat(depth);
    out += ind + '- ' + mmNodeLine(n) + '\n';
    if (n.note) for (const ln of String(n.note).split('\n')) out += ind + '  > ' + ln + '\n';
    out += mmOutlineText(n.children || [], depth + 1);
  }
  return out;
}
// mmParseInline reads the markers mmNodeLine writes.
function mmParseInline(text) {
  const n = { id: mmId(), text: '', children: [] };
  text = text.trim();
  if (/^\[ \]\s/.test(text)) { n.done = false; text = text.slice(3).trim(); }
  else if (/^\[[xX]\]\s/.test(text)) { n.done = true; text = text.slice(3).trim(); }
  text = text.replace(/\[\[\s*([^\]\n|]+?)\s*\]\]/g, (m, tok) => {
    tok = tok.trim();
    if (/^[A-Za-z][A-Za-z0-9_]*-\d+$/.test(tok)) n.task_ref = tok.toUpperCase();
    else n._page = tok; // resolved by the caller if it knows the space
    return '';
  }).trim();
  const tags = [];
  text = text.replace(/(^|\s)#([\p{L}\p{N}_-]+)/gu, (m, sp, tg) => { tags.push(tg.replace(/_/g, ' ')); return sp; }).trim();
  if (tags.length) n.tags = tags;
  const pm = text.match(/\s\(P([1-6])\)/);
  if (pm) { n.priority = Number(pm[1]); text = text.replace(pm[0], '').trim(); }
  const gm = text.match(/\s\((\d{1,3})%\)/);
  if (gm && Number(gm[1]) <= 100) { n.progress = Number(gm[1]); text = text.replace(gm[0], '').trim(); }
  const im = text.match(/^(\S+)\s+/);
  if (im && !/[\p{L}\p{N}]/u.test(im[1]) && /[☀-➿\u{1F000}-\u{1FAFF}]/u.test(im[1])) { n.icon = im[1]; text = text.slice(im[0].length); }
  const lm = text.match(/^\[([^\]]*)\]\(([^)\s]+)\)/);
  if (lm) { n.url = lm[2]; text = (lm[1] + text.slice(lm[0].length)).trim(); }
  n.text = text;
  return n;
}
// mmParseOutline turns indented text (markdown list, headings or plain lines)
// into an array of subtrees. Blockquote / plain lines become notes.
function mmParseOutline(src) {
  src = String(src || '').replace(/\r\n/g, '\n');
  const lines = src.split('\n');
  let unit = 0;
  for (const ln of lines) {
    if (!ln.trim()) continue;
    const ws = ln.match(/^[ \t]*/)[0].replace(/\t/g, '    ').length;
    if (ws > 0 && (unit === 0 || ws < unit)) unit = ws;
  }
  if (!unit) unit = 2;
  const virtual = { children: [] };
  const stack = [{ depth: -1, node: virtual }];
  let last = null;
  for (const raw of lines) {
    if (!raw.trim() || /^\s*<!--/.test(raw) || /^\s*```/.test(raw)) continue;
    const hm = raw.match(/^(#{1,6})\s+(.*)$/);
    const lm = raw.match(/^([ \t]*)([-*+]|\d+[.)])\s+(.*)$/);
    let depth, text;
    if (hm) { depth = hm[1].length - 1; text = hm[2]; }
    else if (lm) { depth = 100 + Math.floor(lm[1].replace(/\t/g, '    ').length / unit); text = lm[3]; }
    else {
      const tx = raw.trim();
      if (/^>/.test(tx) || last) {
        const note = tx.replace(/^>\s?/, '');
        if (last) last.note = last.note ? last.note + '\n' + note : note;
        continue;
      }
      depth = 100 + Math.floor(raw.match(/^[ \t]*/)[0].replace(/\t/g, '    ').length / unit); text = tx;
    }
    while (stack.length > 1 && stack[stack.length - 1].depth >= depth) stack.pop();
    const n = mmParseInline(text);
    stack[stack.length - 1].node.children.push(n);
    stack.push({ depth, node: n });
    last = n;
  }
  return virtual.children;
}
// mmParsePlainLines: every non-empty line is a node (used for plain text pastes).
function mmParsePlainLines(src) {
  const trimmed = String(src || '').trim();
  if (!trimmed) return [];
  if (/^\s*([-*+]|\d+[.)]|#)\s/m.test(trimmed) || /\n[ \t]+\S/.test(trimmed)) return mmParseOutline(trimmed);
  return trimmed.split('\n').map(l => l.trim()).filter(Boolean).map(l => mmParseInline(l));
}

// --- Layout -------------------------------------------------------------
// mmLayout computes positions for the visible part of the tree.
//   doc    – the document
//   size   – function(node, isRoot) -> {w, h}
//   colors – palette array (resolved), rootColor
// Returns {pos: Map(id -> {x,y,w,h,color,depth,side,parent}), order: [ids], bbox}.
function mmLayout(doc, size, colors, rootColor) {
  const pos = new Map();
  const order = [];
  const layout = doc.layout;
  const colorOf = (n, parentColor, idx, depth) => n.color || (depth === 1 ? colors[idx % colors.length] : parentColor);
  const visibleKids = n => n.collapsed ? [] : n.children;
  // Pre-pass: sizes + subtree extents.
  const ext = new Map(); // id -> {w,h} of the whole visible subtree in the layout's main axis
  const horizontal = layout === 'balanced' || layout === 'right' || layout === 'left';
  const vertical = layout === 'down' || layout === 'up';
  const measure = (n, isRoot) => {
    const s = size(n, isRoot);
    const kids = visibleKids(n);
    let sum = 0;
    for (const c of kids) { const e = measure(c, false); sum += horizontal ? e.h : e.w; }
    if (kids.length) sum += (kids.length - 1) * (horizontal ? MM_GAP_Y : MM_GAP_X_DOWN);
    const e = horizontal ? { w: s.w, h: Math.max(s.h, sum), nw: s.w, nh: s.h } : { w: Math.max(s.w, sum), h: s.h, nw: s.w, nh: s.h };
    ext.set(n.id, e);
    return e;
  };
  measure(doc.root, true);
  const place = (n, x, y, dir, parentId, depth, color) => {
    // horizontal: dir = +1 (right) / -1 (left); x is the edge nearest the parent
    const e = ext.get(n.id);
    const nx = dir > 0 ? x : x - e.nw;
    const ny = y + (e.h - e.nh) / 2;
    pos.set(n.id, { x: nx, y: ny, w: e.nw, h: e.nh, color, depth, parent: parentId, side: dir > 0 ? 'right' : 'left' });
    order.push(n.id);
    const kids = visibleKids(n);
    const total = kids.reduce((a, c) => a + ext.get(c.id).h, 0) + Math.max(0, kids.length - 1) * MM_GAP_Y;
    let cy = y + (e.h - total) / 2;
    const cx = dir > 0 ? nx + e.nw + MM_GAP_X : nx - MM_GAP_X;
    kids.forEach((c, i) => {
      place(c, cx, cy, dir, n.id, depth + 1, colorOf(c, color, i, depth + 1));
      cy += ext.get(c.id).h + MM_GAP_Y;
    });
  };
  const placeV = (n, x, y, dir, parentId, depth, color) => {
    // vertical: dir = +1 (down) / -1 (up); y is the edge nearest the parent
    const e = ext.get(n.id);
    const nx = x + (e.w - e.nw) / 2;
    const ny = dir > 0 ? y : y - e.nh;
    pos.set(n.id, { x: nx, y: ny, w: e.nw, h: e.nh, color, depth, parent: parentId, side: dir > 0 ? 'down' : 'up' });
    order.push(n.id);
    const kids = visibleKids(n);
    const total = kids.reduce((a, c) => a + ext.get(c.id).w, 0) + Math.max(0, kids.length - 1) * MM_GAP_X_DOWN;
    let cx = x + (e.w - total) / 2;
    const cy = dir > 0 ? ny + e.nh + MM_GAP_Y_DOWN : ny - MM_GAP_Y_DOWN;
    kids.forEach((c, i) => {
      placeV(c, cx, cy, dir, n.id, depth + 1, colorOf(c, color, i, depth + 1));
      cx += ext.get(c.id).w + MM_GAP_X_DOWN;
    });
  };
  const root = doc.root;
  const rs = ext.get(root.id);
  if (layout === 'tree') {
    let cursor = 0;
    const placeT = (n, depth, parentId, color) => {
      const e = ext.get(n.id);
      pos.set(n.id, { x: depth * MM_TREE_INDENT, y: cursor, w: e.nw, h: e.nh, color, depth, parent: parentId, side: 'down' });
      order.push(n.id);
      cursor += e.nh + MM_GAP_Y;
      visibleKids(n).forEach((c, i) => placeT(c, depth + 1, n.id, colorOf(c, color, i, depth + 1)));
    };
    placeT(root, 0, null, rootColor);
  } else if (horizontal) {
    const kids = visibleKids(root);
    let rightKids = kids, leftKids = [];
    if (layout === 'left') { rightKids = []; leftKids = kids; }
    else if (layout === 'balanced') {
      // Explicit sides win; the rest are distributed to balance total height.
      rightKids = []; leftKids = [];
      let hr = 0, hl = 0;
      const auto = [];
      kids.forEach(c => {
        if (c.side === 'left') { leftKids.push(c); hl += ext.get(c.id).h; }
        else if (c.side === 'right') { rightKids.push(c); hr += ext.get(c.id).h; }
        else auto.push(c);
      });
      auto.forEach(c => {
        if (hr <= hl) { rightKids.push(c); hr += ext.get(c.id).h; } else { leftKids.push(c); hl += ext.get(c.id).h; }
      });
      // keep document order within each side
      const idx = new Map(kids.map((c, i) => [c.id, i]));
      rightKids.sort((a, b) => idx.get(a.id) - idx.get(b.id));
      leftKids.sort((a, b) => idx.get(a.id) - idx.get(b.id));
    }
    const sumH = arr => arr.reduce((a, c) => a + ext.get(c.id).h, 0) + Math.max(0, arr.length - 1) * MM_GAP_Y;
    const hR = sumH(rightKids), hL = sumH(leftKids);
    const H = Math.max(rs.nh, hR, hL);
    const rx = -rs.nw / 2, ry = -rs.nh / 2;
    pos.set(root.id, { x: rx, y: ry, w: rs.nw, h: rs.nh, color: rootColor, depth: 0, parent: null, side: '' });
    order.push(root.id);
    const colorIdx = new Map(kids.map((c, i) => [c.id, i]));
    let cy = -hR / 2;
    rightKids.forEach(c => { place(c, rx + rs.nw + MM_GAP_X, cy, 1, root.id, 1, colorOf(c, rootColor, colorIdx.get(c.id), 1)); cy += ext.get(c.id).h + MM_GAP_Y; });
    cy = -hL / 2;
    leftKids.forEach(c => { place(c, rx - MM_GAP_X, cy, -1, root.id, 1, colorOf(c, rootColor, colorIdx.get(c.id), 1)); cy += ext.get(c.id).h + MM_GAP_Y; });
    void H;
  } else if (vertical) {
    const dir = layout === 'down' ? 1 : -1;
    const kids = visibleKids(root);
    const total = kids.reduce((a, c) => a + ext.get(c.id).w, 0) + Math.max(0, kids.length - 1) * MM_GAP_X_DOWN;
    const rx = -rs.nw / 2, ry = -rs.nh / 2;
    pos.set(root.id, { x: rx, y: ry, w: rs.nw, h: rs.nh, color: rootColor, depth: 0, parent: null, side: '' });
    order.push(root.id);
    let cx = -total / 2;
    const cy = dir > 0 ? ry + rs.nh + MM_GAP_Y_DOWN : ry - MM_GAP_Y_DOWN;
    kids.forEach((c, i) => { placeV(c, cx, cy, dir, root.id, 1, colorOf(c, rootColor, i, 1)); cx += ext.get(c.id).w + MM_GAP_X_DOWN; });
  }
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
  for (const p of pos.values()) { minX = Math.min(minX, p.x); minY = Math.min(minY, p.y); maxX = Math.max(maxX, p.x + p.w); maxY = Math.max(maxY, p.y + p.h); }
  return { pos, order, bbox: { x: minX, y: minY, w: maxX - minX, h: maxY - minY }, layout };
}

// mmEdgePath returns the SVG path between a parent and child box.
function mmEdgePath(line, layout, p, c) {
  let x1, y1, x2, y2;
  if (layout === 'tree') {
    x1 = p.x + 14; y1 = p.y + p.h; x2 = c.x; y2 = c.y + c.h / 2;
    return `M${x1} ${y1} V${y2 - 6} Q${x1} ${y2} ${x1 + 6} ${y2} H${x2}`;
  }
  if (c.side === 'down' || c.side === 'up') {
    x1 = p.x + p.w / 2; x2 = c.x + c.w / 2;
    if (c.side === 'down') { y1 = p.y + p.h; y2 = c.y; } else { y1 = p.y; y2 = c.y + c.h; }
    const my = (y1 + y2) / 2;
    if (line === 'straight') return `M${x1} ${y1} L${x2} ${y2}`;
    if (line === 'elbow') return `M${x1} ${y1} V${my} H${x2} V${y2}`;
    return `M${x1} ${y1} C${x1} ${my} ${x2} ${my} ${x2} ${y2}`;
  }
  y1 = p.y + p.h / 2; y2 = c.y + c.h / 2;
  if (c.side === 'right') { x1 = p.x + p.w; x2 = c.x; } else { x1 = p.x; x2 = c.x + c.w; }
  const mx = (x1 + x2) / 2;
  if (line === 'straight') return `M${x1} ${y1} L${x2} ${y2}`;
  if (line === 'elbow') return `M${x1} ${y1} H${mx} V${y2} H${x2}`;
  return `M${x1} ${y1} C${mx} ${y1} ${mx} ${y2} ${x2} ${y2}`;
}
// mmRelPath: a relationship curve between two boxes (bulging perpendicular).
function mmRelPath(a, b) {
  const ax = a.x + a.w / 2, ay = a.y + a.h / 2, bx = b.x + b.w / 2, by = b.y + b.h / 2;
  const dx = bx - ax, dy = by - ay, len = Math.hypot(dx, dy) || 1;
  const nx = -dy / len, ny = dx / len, bulge = Math.min(120, len * 0.25);
  const cx = (ax + bx) / 2 + nx * bulge, cy = (ay + by) / 2 + ny * bulge;
  // start/end on the box edges along the curve direction
  const clip = (box, fx, fy) => {
    const cxb = box.x + box.w / 2, cyb = box.y + box.h / 2;
    const vx = fx - cxb, vy = fy - cyb;
    const sx = Math.abs(vx) > 1e-6 ? (box.w / 2) / Math.abs(vx) : Infinity, sy = Math.abs(vy) > 1e-6 ? (box.h / 2) / Math.abs(vy) : Infinity;
    const s = Math.min(sx, sy);
    return [cxb + vx * s, cyb + vy * s];
  };
  const [sx, sy] = clip(a, cx, cy), [ex, ey] = clip(b, cx, cy);
  return { d: `M${sx} ${sy} Q${cx} ${cy} ${ex} ${ey}`, mx: (sx + 2 * cx + ex) / 4, my: (sy + 2 * cy + ey) / 4 };
}

// === 3. MindMap engine ===
class MindMap {
  // opts: {readOnly, onChange(doc), onSelect(ids), onOpenPage(id), onOpenTask(ref), onStatus(), uploadImage(file)->url, resolvePage(title)->Promise<id>}
  constructor(container, opts) {
    this.opts = opts || {};
    this.readOnly = !!this.opts.readOnly;
    this.container = container;
    this.doc = mmNormalizeDoc({ root: { text: '' } });
    this.index = new Map(); this.parentOf = new Map();
    this.sel = new Set(); this.anchor = null; this.selLink = null;
    this.k = 1; this.tx = 0; this.ty = 0;
    this.sizeCache = new Map();
    this.undoStack = []; this.redoStack = [];
    this.editing = null; this.query = ''; this.matches = []; this.matchIdx = -1;
    this.linkMode = null; this.panel = null; this.hover = null;
    this.animFrom = null; this.animRaf = null; this.renderRaf = null;
    this.elems = new Map();
    this.embedded = !!this.opts.embedded;
    this.buildDom();
    this.bindEvents();
  }

  buildDom() {
    const c = this.container;
    c.classList.add('mm-canvas');
    c.tabIndex = 0;
    c.setAttribute('role', 'tree');
    c.innerHTML = `
      <div class="mm-viewport">
        <svg class="mm-edges" xmlns="http://www.w3.org/2000/svg">
          <defs><marker id="mm-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0 L10 5 L0 10 z"/></marker></defs>
          <g class="mm-g-bounds"></g><g class="mm-g-branches"></g><g class="mm-g-rels"></g>
        </svg>
        <div class="mm-nodes"></div>
      </div>
      <div class="mm-measure" aria-hidden="true"></div>
      <div class="mm-marquee hidden"></div>
      <div class="mm-drop-line hidden"></div>
      <div class="mm-live" aria-live="polite"></div>
      ${this.embedded ? '' : `<div class="mm-hud">
        <button class="mm-hud-btn" data-act="zoomOut" title="${escAttr(mmT('mmZoomOut'))}">−</button>
        <button class="mm-hud-btn mm-hud-zoom" data-act="zoom100" title="100%">100%</button>
        <button class="mm-hud-btn" data-act="zoomIn" title="${escAttr(mmT('mmZoomIn'))}">+</button>
        <button class="mm-hud-btn" data-act="fit" title="${escAttr(mmT('mmFit'))}">⤢</button>
        <button class="mm-hud-btn" data-act="center" title="${escAttr(mmT('mmCenter'))}">◎</button>
      </div>
      <canvas class="mm-minimap" width="180" height="110"></canvas>`}`;
    this.viewport = c.querySelector('.mm-viewport');
    this.svg = c.querySelector('.mm-edges');
    this.gBounds = c.querySelector('.mm-g-bounds');
    this.gBranches = c.querySelector('.mm-g-branches');
    this.gRels = c.querySelector('.mm-g-rels');
    this.nodesEl = c.querySelector('.mm-nodes');
    this.measureEl = c.querySelector('.mm-measure');
    this.marqueeEl = c.querySelector('.mm-marquee');
    this.dropLineEl = c.querySelector('.mm-drop-line');
    this.liveEl = c.querySelector('.mm-live');
    this.minimap = c.querySelector('.mm-minimap');
  }

  destroy() {
    this.detachGlobal && this.detachGlobal();
    this.container.innerHTML = '';
  }

  // --- model ---------------------------------------------------------------
  setDoc(doc, keepView) {
    this.doc = mmNormalizeDoc(mmClone(doc));
    this.reindex();
    this.undoStack = []; this.redoStack = [];
    if (!keepView) { this.sel.clear(); this.anchor = null; }
    else { for (const id of [...this.sel]) if (!this.index.has(id)) this.sel.delete(id); if (this.anchor && !this.index.has(this.anchor)) this.anchor = null; }
    this.editing = null; this.selLink = null;
    this.render(false);
    if (!keepView) this.fit();
    this.updateStatus();
  }
  getDoc() { return mmClone(this.doc); }
  reindex() {
    this.index.clear(); this.parentOf.clear(); this.depth = new Map();
    const walk = (n, p, d) => { this.index.set(n.id, n); this.parentOf.set(n.id, p ? p.id : null); this.depth.set(n.id, d); n.children.forEach(c => walk(c, n, d + 1)); };
    walk(this.doc.root, null, 0);
  }
  node(id) { return this.index.get(id); }
  parent(id) { const p = this.parentOf.get(id); return p ? this.index.get(p) : null; }
  isVisible(id) {
    let p = this.parentOf.get(id);
    while (p) { const pn = this.index.get(p); if (pn.collapsed) return false; p = this.parentOf.get(p); }
    return true;
  }
  revealNode(id) { // expand ancestors
    let p = this.parentOf.get(id), changed = false;
    while (p) { const pn = this.index.get(p); if (pn.collapsed) { pn.collapsed = false; changed = true; } p = this.parentOf.get(p); }
    return changed;
  }
  subtreeIds(id) { const out = []; const w = n => { out.push(n.id); n.children.forEach(w); }; const n = this.node(id); if (n) w(n); return out; }
  count() { return this.index.size; }

  // --- history -------------------------------------------------------------
  snapshot() { return JSON.stringify({ doc: this.doc, sel: [...this.sel], anchor: this.anchor }); }
  // commit records the state before a change (call before mutating) …
  begin() { this._pending = this.snapshot(); }
  // … and finishes it: push to undo, reindex, render, notify.
  commit(opts) {
    opts = opts || {};
    if (this._pending) { this.undoStack.push(this._pending); if (this.undoStack.length > 80) this.undoStack.shift(); this.redoStack = []; this._pending = null; }
    this.reindex();
    for (const id of [...this.sel]) if (!this.index.has(id)) this.sel.delete(id);
    if (this.anchor && !this.index.has(this.anchor)) this.anchor = null;
    this.render(opts.animate !== false);
    this.updateStatus();
    if (this.opts.onChange) this.opts.onChange(this.doc);
  }
  restore(json) {
    const s = JSON.parse(json);
    this.doc = s.doc; this.sel = new Set(s.sel.filter(id => true)); this.anchor = s.anchor;
    this.reindex();
    for (const id of [...this.sel]) if (!this.index.has(id)) this.sel.delete(id);
    this.render(true); this.updateStatus();
    if (this.opts.onChange) this.opts.onChange(this.doc);
  }
  undo() { if (this.editing) return; const s = this.undoStack.pop(); if (!s) return; this.redoStack.push(this.snapshot()); this.restore(s); }
  redo() { if (this.editing) return; const s = this.redoStack.pop(); if (!s) return; this.undoStack.push(this.snapshot()); this.restore(s); }

  // --- measuring -----------------------------------------------------------
  sizeKey(n, isRoot) {
    return [isRoot ? 'R' : 'd' + Math.min(this.depth && this.depth.get(n.id) || 1, 3), n.text, n.icon || '', n.shape || '', n.priority || 0, n.progress == null ? '' : n.progress, n.done == null ? '' : n.done,
      n.note ? 1 : 0, n.url ? 1 : 0, n.page_id || 0, n.task_ref || '', (n.tags || []).join(','), n.image || '', n.collapsed ? n.children.length : 0].join('\u0001');
  }
  nodeInner(n, isRoot) {
    const badges = [];
    if (n.priority) badges.push(`<span class="mm-b mm-b-prio mm-prio-${n.priority}" title="P${n.priority}">${n.priority}</span>`);
    if (n.progress != null) badges.push(`<span class="mm-b mm-b-prog" title="${n.progress}%" style="--mm-p:${n.progress}"><i></i></span>`);
    if (n.note) badges.push(`<span class="mm-b mm-b-note" title="${escAttr(mmFirstLine(n.note).slice(0, 120))}">≡</span>`);
    if (n.url) badges.push(`<a class="mm-b mm-b-url" href="${escAttr(n.url)}" target="_blank" rel="noopener noreferrer" title="${escAttr(n.url)}">🔗</a>`);
    if (n.page_id) badges.push(`<span class="mm-b mm-b-page" data-page="${n.page_id}" title="${escAttr(mmT('mmOpenPage'))}">📄</span>`);
    if (n.task_ref) badges.push(`<span class="mm-b mm-b-task" data-task="${escAttr(n.task_ref)}" title="${escAttr(mmT('mmOpenTask'))}">${esc(n.task_ref)}</span>`);
    const tags = (n.tags || []).map(tg => `<span class="mm-tag">${esc(tg)}</span>`).join('');
    const check = n.done == null ? '' : `<span class="mm-check ${n.done ? 'on' : ''}" role="checkbox" aria-checked="${n.done}"></span>`;
    const icon = n.icon ? `<span class="mm-icon">${esc(n.icon)}</span>` : '';
    const img = n.image ? `<img class="mm-img" src="${escAttr(n.image)}" alt="" draggable="false">` : '';
    const fold = n.children.length && !isRoot ? `<button class="mm-fold ${n.collapsed ? 'closed' : ''}" tabindex="-1" aria-label="${n.collapsed ? escAttr(mmT('mmUnfold')) : escAttr(mmT('mmFold'))}">${n.collapsed ? n.children.length : '−'}</button>` : '';
    const add = (!this.readOnly && !this.embedded) ? `<button class="mm-add" tabindex="-1" aria-label="${escAttr(mmT('mmNewChild'))}">+</button>` : '';
    return `${img}<div class="mm-line">${check}${icon}<span class="mm-text">${esc(n.text) || '&#8203;'}</span>${badges.join('')}</div>${tags ? `<div class="mm-tags">${tags}</div>` : ''}${fold}${add}`;
  }
  nodeClass(n, isRoot, p) {
    const side = p && p.side ? ' mm-side-' + p.side : '';
    return `mm-node mm-d${Math.min(p ? p.depth : 0, 3)}${isRoot ? ' mm-root' : ''}${n.shape ? ' mm-shape-' + n.shape : ''}${this.sel.has(n.id) ? ' mm-sel' : ''}${n.collapsed ? ' mm-collapsed' : ''}${side}`;
  }
  measureAll() {
    const todo = [];
    const walk = (n, isRoot) => { const k = this.sizeKey(n, isRoot); if (!this.sizeCache.has(k)) todo.push([n, isRoot, k]); if (!n.collapsed) n.children.forEach(c => walk(c, false)); };
    walk(this.doc.root, true);
    if (!todo.length) return;
    const frag = document.createDocumentFragment();
    const els = todo.map(([n, isRoot]) => {
      const el = document.createElement('div');
      el.className = this.nodeClass(n, isRoot, { depth: this.depth.get(n.id) || 0 }).replace(' mm-sel', '');
      el.innerHTML = this.nodeInner(n, isRoot);
      frag.appendChild(el);
      return el;
    });
    this.measureEl.appendChild(frag);
    todo.forEach(([n, isRoot, k], i) => this.sizeCache.set(k, { w: els[i].offsetWidth, h: els[i].offsetHeight }));
    this.measureEl.innerHTML = '';
  }
  sizeOf(n, isRoot) {
    if (this.editing === n.id) { const el = this.elems.get(n.id); if (el) return { w: el.offsetWidth, h: el.offsetHeight }; }
    return this.sizeCache.get(this.sizeKey(n, isRoot)) || { w: 80, h: 32 };
  }
  invalidateSize(n) { this.sizeCache.delete(this.sizeKey(n, n === this.doc.root)); }

  // --- rendering -----------------------------------------------------------
  paletteColors() {
    const accent = mmCssVar('--accent', '#7c6ff7');
    const pal = MM_PALETTES[this.doc.palette];
    return { colors: pal || [accent], root: accent };
  }
  scheduleRender() { if (this.renderRaf) return; this.renderRaf = requestAnimationFrame(() => { this.renderRaf = null; this.render(false); }); }
  render(animate) {
    this.measureAll();
    const { colors, root } = this.paletteColors();
    const prev = this.layoutData ? this.layoutData.pos : null;
    const L = mmLayout(this.doc, (n, r) => this.sizeOf(n, r), colors, root);
    this.layoutData = L;
    const canAnimate = animate && prev && !mmReducedMotion() && L.order.length <= MM_ANIM_MAX_NODES;
    if (this.animRaf) { cancelAnimationFrame(this.animRaf); this.animRaf = null; }
    if (canAnimate) {
      const from = new Map();
      for (const id of L.order) { const p = prev.get(id); if (p) from.set(id, { x: p.x, y: p.y }); }
      if (from.size) { this.syncDom(L); this.animate(from, L); return; }
    }
    this.syncDom(L);
    this.applyPositions(L, null, 1);
  }
  animate(from, L) {
    const start = performance.now();
    const step = now => {
      const tt = Math.min(1, (now - start) / MM_ANIM_MS);
      const e = 1 - Math.pow(1 - tt, 3);
      this.applyPositions(L, from, e);
      if (tt < 1) this.animRaf = requestAnimationFrame(step); else { this.animRaf = null; }
    };
    this.animRaf = requestAnimationFrame(step);
  }
  // syncDom creates/updates node elements for the visible nodes (culled to the
  // viewport on large maps) and removes the rest.
  syncDom(L) {
    const want = new Set(L.order);
    const cull = L.order.length > MM_CULL_FROM ? this.visibleWorldRect(400) : null;
    for (const [id, el] of this.elems) if (!want.has(id)) { el.remove(); this.elems.delete(id); }
    for (const id of L.order) {
      const n = this.index.get(id), p = L.pos.get(id), isRoot = id === this.doc.root.id;
      if (cull && (p.x + p.w < cull.x || p.x > cull.x + cull.w || p.y + p.h < cull.y || p.y > cull.y + cull.h) && !this.sel.has(id) && this.editing !== id) {
        const old = this.elems.get(id); if (old) { old.remove(); this.elems.delete(id); }
        continue;
      }
      let el = this.elems.get(id);
      const key = this.sizeKey(n, isRoot) + '|' + (n.color || '');
      if (!el) {
        el = document.createElement('div');
        el.dataset.id = id;
        el.setAttribute('role', 'treeitem');
        el.tabIndex = -1;
        this.nodesEl.appendChild(el);
        this.elems.set(id, el);
        el._key = null;
      }
      if (el._key !== key && this.editing !== id) { el.innerHTML = this.nodeInner(n, isRoot); el._key = key; }
      el.className = this.nodeClass(n, isRoot, p) + (this.editing === id ? ' mm-editing' : '') + (this.query && !this.matches.includes(id) ? ' mm-dim' : '') + (this.query && this.matches.includes(id) ? ' mm-match' : '') + (this.matches[this.matchIdx] === id ? ' mm-match-cur' : '');
      el.style.setProperty('--mm-c', p.color);
      el.setAttribute('aria-level', p.depth + 1);
      el.setAttribute('aria-selected', this.sel.has(id));
      if (n.children.length) el.setAttribute('aria-expanded', !n.collapsed); else el.removeAttribute('aria-expanded');
      el.setAttribute('aria-label', mmFirstLine(n.text));
      if (this.editing !== id) el.style.width = p.w + 'px';
    }
  }
  applyPositions(L, from, e) {
    const cur = new Map();
    for (const id of L.order) {
      const p = L.pos.get(id);
      let x = p.x, y = p.y;
      if (from && from.has(id) && e < 1) { const f = from.get(id); x = f.x + (p.x - f.x) * e; y = f.y + (p.y - f.y) * e; }
      cur.set(id, { x, y, w: p.w, h: p.h, side: p.side, color: p.color, depth: p.depth, parent: p.parent });
      const el = this.elems.get(id);
      if (el) el.style.transform = `translate(${x}px,${y}px)`;
    }
    this.drawEdges(L, cur);
    if (e >= 1) this.drawMinimap();
  }
  drawEdges(L, cur) {
    const line = this.doc.line, layout = this.doc.layout;
    let branches = '', bounds = '', rels = '';
    for (const id of L.order) {
      const p = cur.get(id);
      if (!p.parent) continue;
      const pp = cur.get(p.parent);
      const w = p.depth === 1 ? 3 : p.depth === 2 ? 2.2 : 1.6;
      branches += `<path d="${mmEdgePath(line, layout, pp, p)}" stroke="${p.color}" stroke-width="${w}" fill="none" stroke-linecap="round"/>`;
    }
    // boundaries: shaded rounded rect around a node's visible subtree
    for (const id of L.order) {
      const n = this.index.get(id);
      if (!n.boundary) continue;
      let x1 = Infinity, y1 = Infinity, x2 = -Infinity, y2 = -Infinity;
      for (const sid of this.subtreeIds(id)) { const q = cur.get(sid); if (!q) continue; x1 = Math.min(x1, q.x); y1 = Math.min(y1, q.y); x2 = Math.max(x2, q.x + q.w); y2 = Math.max(y2, q.y + q.h); }
      if (x1 === Infinity) continue;
      const c = cur.get(id).color;
      bounds += `<rect x="${x1 - 10}" y="${y1 - 8}" width="${x2 - x1 + 20}" height="${y2 - y1 + 16}" rx="14" fill="${c}" fill-opacity=".08" stroke="${c}" stroke-opacity=".6" stroke-dasharray="6 4" stroke-width="1.5"/>`;
    }
    for (const l of this.doc.links) {
      const a = cur.get(l.from), b = cur.get(l.to);
      if (!a || !b) continue;
      const r = mmRelPath(a, b);
      const selc = this.selLink === l.id ? ' mm-rel-sel' : '';
      rels += `<g class="mm-rel${selc}" data-link="${l.id}"><path class="mm-rel-hit" d="${r.d}"/><path class="mm-rel-line" d="${r.d}" marker-end="url(#mm-arrow)"/>${l.label ? `<text x="${r.mx}" y="${r.my}" text-anchor="middle" dy="-4">${esc(l.label)}</text>` : ''}</g>`;
    }
    this.gBranches.innerHTML = branches;
    this.gBounds.innerHTML = bounds;
    this.gRels.innerHTML = rels;
    const b = L.bbox, m = 2000;
    this.svg.setAttribute('viewBox', `${b.x - m} ${b.y - m} ${b.w + 2 * m} ${b.h + 2 * m}`);
    this.svg.style.left = (b.x - m) + 'px'; this.svg.style.top = (b.y - m) + 'px';
    this.svg.style.width = (b.w + 2 * m) + 'px'; this.svg.style.height = (b.h + 2 * m) + 'px';
  }

  // --- viewport ------------------------------------------------------------
  applyView() {
    this.viewport.style.transform = `translate(${this.tx}px,${this.ty}px) scale(${this.k})`;
    const z = this.container.querySelector('.mm-hud-zoom'); if (z) z.textContent = Math.round(this.k * 100) + '%';
    if (this.layoutData && this.layoutData.order.length > MM_CULL_FROM) this.scheduleRender();
    this.drawMinimap();
    if (this.opts.onView) this.opts.onView({ k: this.k, tx: this.tx, ty: this.ty });
  }
  setView(v) { this.k = v.k; this.tx = v.tx; this.ty = v.ty; this.applyView(); }
  visibleWorldRect(margin) {
    const r = this.container.getBoundingClientRect();
    margin = margin || 0;
    return { x: (-this.tx - margin) / this.k, y: (-this.ty - margin) / this.k, w: (r.width + 2 * margin) / this.k, h: (r.height + 2 * margin) / this.k };
  }
  zoomAt(sx, sy, factor) {
    const nk = Math.min(4, Math.max(0.1, this.k * factor));
    const r = this.container.getBoundingClientRect();
    const px = sx - r.left, py = sy - r.top;
    this.tx = px - (px - this.tx) * (nk / this.k);
    this.ty = py - (py - this.ty) * (nk / this.k);
    this.k = nk;
    this.applyView();
  }
  zoomBy(f) { const r = this.container.getBoundingClientRect(); this.zoomAt(r.left + r.width / 2, r.top + r.height / 2, f); }
  fit(ids) {
    if (!this.layoutData) return;
    let b = this.layoutData.bbox;
    if (ids && ids.length) {
      let x1 = Infinity, y1 = Infinity, x2 = -Infinity, y2 = -Infinity;
      ids.forEach(id => { const p = this.layoutData.pos.get(id); if (!p) return; x1 = Math.min(x1, p.x); y1 = Math.min(y1, p.y); x2 = Math.max(x2, p.x + p.w); y2 = Math.max(y2, p.y + p.h); });
      if (x1 !== Infinity) b = { x: x1, y: y1, w: x2 - x1, h: y2 - y1 };
    }
    const r = this.container.getBoundingClientRect();
    if (!r.width || !r.height) return;
    const pad = 40;
    this.k = Math.min(1.5, Math.max(0.1, Math.min((r.width - pad * 2) / Math.max(1, b.w), (r.height - pad * 2) / Math.max(1, b.h))));
    this.tx = r.width / 2 - (b.x + b.w / 2) * this.k;
    this.ty = r.height / 2 - (b.y + b.h / 2) * this.k;
    this.applyView();
  }
  centerOn(id, keepZoom) {
    const p = this.layoutData && this.layoutData.pos.get(id);
    if (!p) return;
    const r = this.container.getBoundingClientRect();
    if (!keepZoom) this.k = 1;
    this.tx = r.width / 2 - (p.x + p.w / 2) * this.k;
    this.ty = r.height / 2 - (p.y + p.h / 2) * this.k;
    this.applyView();
  }
  // ensureVisible pans minimally so the node is on screen.
  ensureVisible(id) {
    const p = this.layoutData && this.layoutData.pos.get(id);
    if (!p) return;
    const r = this.container.getBoundingClientRect();
    const sx1 = this.tx + p.x * this.k, sy1 = this.ty + p.y * this.k, sx2 = sx1 + p.w * this.k, sy2 = sy1 + p.h * this.k;
    const pad = 24;
    let dx = 0, dy = 0;
    if (sx1 < pad) dx = pad - sx1; else if (sx2 > r.width - pad) dx = r.width - pad - sx2;
    if (sy1 < pad) dy = pad - sy1; else if (sy2 > r.height - pad) dy = r.height - pad - sy2;
    if (dx || dy) { this.tx += dx; this.ty += dy; this.applyView(); }
  }
  drawMinimap() {
    const mm = this.minimap;
    if (!mm || !this.layoutData) return;
    const ctx = mm.getContext('2d');
    const b = this.layoutData.bbox;
    ctx.clearRect(0, 0, mm.width, mm.height);
    if (!b.w || !b.h) return;
    const s = Math.min((mm.width - 8) / b.w, (mm.height - 8) / b.h);
    const ox = (mm.width - b.w * s) / 2 - b.x * s, oy = (mm.height - b.h * s) / 2 - b.y * s;
    this._mmScale = { s, ox, oy };
    for (const [id, p] of this.layoutData.pos) {
      ctx.fillStyle = p.color; ctx.globalAlpha = id === this.doc.root.id ? 1 : 0.7;
      ctx.fillRect(ox + p.x * s, oy + p.y * s, Math.max(2, p.w * s), Math.max(1.5, p.h * s));
    }
    ctx.globalAlpha = 1;
    const v = this.visibleWorldRect(0);
    ctx.strokeStyle = mmCssVar('--fg2', '#888'); ctx.lineWidth = 1;
    ctx.strokeRect(ox + v.x * s, oy + v.y * s, v.w * s, v.h * s);
    mm.classList.toggle('hidden', b.w * this.k < this.container.clientWidth && b.h * this.k < this.container.clientHeight);
  }

  // --- selection -----------------------------------------------------------
  select(id, add) {
    if (!add) this.sel.clear();
    if (id) { if (add && this.sel.has(id) && this.sel.size > 1) this.sel.delete(id); else this.sel.add(id); this.anchor = id; }
    else if (!add) this.anchor = null;
    this.selLink = null;
    this.refreshSelection();
  }
  refreshSelection() {
    for (const [id, el] of this.elems) { const on = this.sel.has(id); el.classList.toggle('mm-sel', on); el.setAttribute('aria-selected', on); }
    this.gRels.querySelectorAll('.mm-rel').forEach(g => g.classList.toggle('mm-rel-sel', g.dataset.link === this.selLink));
    const a = this.anchor && this.elems.get(this.anchor);
    if (a && !this.editing && document.activeElement !== a && this.container.contains(document.activeElement)) a.focus({ preventScroll: true });
    if (this.opts.onSelect) this.opts.onSelect([...this.sel], this.anchor);
    this.updateStatus();
  }
  selectAll() { this.sel = new Set(this.layoutData ? this.layoutData.order : []); this.refreshSelection(); }
  updateStatus() { if (this.opts.onStatus) this.opts.onStatus(); }
  announce(msg) { if (this.liveEl) { this.liveEl.textContent = ''; setTimeout(() => { this.liveEl.textContent = msg; }, 20); } }

  // --- structural edits ----------------------------------------------------
  newNode(text) { return { id: mmId(), text: text || '', children: [] }; }
  addChild(parentId, text, edit) {
    if (this.readOnly) return null;
    const p = this.node(parentId || this.anchor || this.doc.root.id);
    if (!p) return null;
    this.begin();
    const n = this.newNode(text);
    p.collapsed = false;
    p.children.push(n);
    this.sel = new Set([n.id]); this.anchor = n.id;
    this.commit();
    this.ensureVisible(n.id);
    if (edit !== false) this.edit(n.id);
    return n.id;
  }
  addSibling(id, before, text) {
    if (this.readOnly) return null;
    id = id || this.anchor;
    const p = this.parent(id);
    if (!p) return this.addChild(id, text);
    this.begin();
    const n = this.newNode(text);
    const cur = this.node(id);
    if (p === this.doc.root && cur.side) n.side = cur.side;
    const i = p.children.indexOf(cur);
    p.children.splice(before ? i : i + 1, 0, n);
    this.sel = new Set([n.id]); this.anchor = n.id;
    this.commit();
    this.ensureVisible(n.id);
    this.edit(n.id);
    return n.id;
  }
  insertParent(id) {
    if (this.readOnly) return;
    id = id || this.anchor;
    const p = this.parent(id);
    if (!p) return;
    this.begin();
    const cur = this.node(id);
    const n = this.newNode('');
    n.side = cur.side; cur.side = '';
    p.children.splice(p.children.indexOf(cur), 1, n);
    n.children.push(cur);
    this.sel = new Set([n.id]); this.anchor = n.id;
    this.commit();
    this.edit(n.id);
  }
  deleteSelected() {
    if (this.readOnly || !this.sel.size) return;
    const ids = [...this.sel].filter(id => id !== this.doc.root.id);
    if (!ids.length) return;
    this.begin();
    let next = null;
    for (const id of ids) {
      const p = this.parent(id);
      if (!p || !this.index.has(id)) continue;
      const i = p.children.findIndex(c => c.id === id);
      if (i < 0) continue;
      p.children.splice(i, 1);
      next = p.children[i] ? p.children[i].id : p.children[i - 1] ? p.children[i - 1].id : p.id;
      const gone = new Set(this.subtreeIds(id));
      this.doc.links = this.doc.links.filter(l => !gone.has(l.from) && !gone.has(l.to));
      this.reindex();
    }
    this.sel = new Set(next ? [next] : []); this.anchor = next;
    this.commit();
    this.announce(mmT('mmDelete'));
  }
  setText(id, text) {
    const n = this.node(id); if (!n) return;
    text = String(text).replace(/\r/g, '').trim();
    if (n.text === text) return;
    this.begin(); n.text = text; this.commit();
  }
  updateNode(id, patch) {
    const n = this.node(id); if (!n || this.readOnly) return;
    this.begin();
    for (const k in patch) { if (patch[k] == null || patch[k] === '' || (Array.isArray(patch[k]) && !patch[k].length)) delete n[k]; else n[k] = patch[k]; }
    if (!Array.isArray(n.children)) n.children = [];
    this.commit();
  }
  toggleFold(id, force) {
    const n = this.node(id || this.anchor);
    if (!n || n === this.doc.root || !n.children.length) return;
    const v = force == null ? !n.collapsed : !!force;
    if (v === !!n.collapsed) return;
    if (this.readOnly) { n.collapsed = v; this.reindex(); this.render(true); return; } // fold state is a view concern for readers
    this.begin(); n.collapsed = v; this.commit();
  }
  foldAll(level) { // level: 0 = collapse everything below root, Infinity = expand all, N = expand to depth N
    const walk = (n, d) => { if (n !== this.doc.root && n.children.length) n.collapsed = d >= level; n.children.forEach(c => walk(c, d + 1)); };
    if (!this.readOnly) this.begin();
    walk(this.doc.root, 0);
    if (this.readOnly) { this.reindex(); this.render(true); } else this.commit();
    this.fit();
  }
  moveSibling(id, dir) {
    if (this.readOnly) return;
    id = id || this.anchor;
    const p = this.parent(id); if (!p) return;
    const i = p.children.findIndex(c => c.id === id), j = i + dir;
    if (j < 0 || j >= p.children.length) return;
    this.begin();
    const [n] = p.children.splice(i, 1); p.children.splice(j, 0, n);
    this.commit();
  }
  // moveNode re-parents id under parentId at index (or before/after refId).
  moveNode(id, parentId, index, side) {
    if (this.readOnly || id === parentId || this.subtreeIds(id).includes(parentId)) return false;
    const n = this.node(id), op = this.parent(id), np = this.node(parentId);
    if (!n || !op || !np) return false;
    this.begin();
    op.children.splice(op.children.indexOf(n), 1);
    if (index == null || index < 0 || index > np.children.length) index = np.children.length;
    np.children.splice(index, 0, n);
    n.side = np === this.doc.root ? (side || n.side || '') : '';
    np.collapsed = false;
    this.commit();
    return true;
  }
  togglePriority(id, p) { const n = this.node(id || this.anchor); if (!n) return; this.updateNode(n.id, { priority: n.priority === p ? 0 : p }); }
  toggleTask(id) {
    const n = this.node(id || this.anchor); if (!n) return;
    this.updateNode(n.id, { done: n.done == null ? false : n.done ? null : true });
  }
  addLink(from, to, label) {
    if (this.readOnly || from === to || !this.node(from) || !this.node(to)) return;
    if (this.doc.links.some(l => l.from === from && l.to === to)) return;
    this.begin(); this.doc.links.push({ id: mmId(), from, to, label: label || '' }); this.commit();
  }
  removeLink(lid) { if (this.readOnly) return; this.begin(); this.doc.links = this.doc.links.filter(l => l.id !== lid); this.selLink = null; this.commit(); }
  setLinkLabel(lid, label) { const l = this.doc.links.find(x => x.id === lid); if (!l || this.readOnly) return; this.begin(); l.label = label; this.commit(); }
  setSettings(patch) { if (this.readOnly) return; this.begin(); Object.assign(this.doc, patch); this.commit(); setTimeout(() => this.fit(), MM_ANIM_MS); }
  // replaceChildren swaps a node's subtree with parsed outline nodes, keeping
  // ids of nodes whose text matches in document order (so links survive).
  replaceChildren(id, kids) {
    if (this.readOnly) return;
    const n = this.node(id); if (!n) return;
    const oldByText = new Map();
    const w = x => { const k = x.text; if (!oldByText.has(k)) oldByText.set(k, []); oldByText.get(k).push(x); x.children.forEach(w); };
    n.children.forEach(w);
    const reuse = x => { const arr = oldByText.get(x.text); if (arr && arr.length) { const o = arr.shift(); x.id = o.id; for (const f of ['color', 'image', 'page_id', 'collapsed', 'boundary', 'side', 'shape']) if (o[f] != null && x[f] == null) x[f] = o[f]; } x.children.forEach(reuse); };
    kids.forEach(reuse);
    this.begin(); n.children = kids; this.commit();
  }

  // --- clipboard -----------------------------------------------------------
  copySelection(cut) {
    const ids = [...this.sel].filter(id => id !== this.doc.root.id && !this.subtreeIds(id).some(x => x !== id && this.sel.has(x)));
    if (!ids.length) return null;
    const nodes = ids.map(id => mmClone(this.node(id)));
    const text = mmOutlineText(nodes);
    try { localStorage.setItem('mm-clipboard', JSON.stringify({ text, nodes, at: Date.now() })); } catch (e) { /* ignore */ }
    this._clip = { text, nodes };
    if (cut && !this.readOnly) this.deleteSelected();
    return text;
  }
  pasteText(text) {
    if (this.readOnly) return;
    const target = this.node(this.anchor || this.doc.root.id);
    if (!target) return;
    let nodes = null;
    let clip = this._clip;
    if (!clip) { try { clip = JSON.parse(localStorage.getItem('mm-clipboard') || 'null'); } catch (e) { clip = null; } }
    if (clip && (text == null || text.trim() === clip.text.trim())) nodes = mmClone(clip.nodes);
    else if (text && text.trim()) nodes = mmParsePlainLines(text);
    if (!nodes || !nodes.length) return;
    const reid = x => { x.id = mmId(); x.side = ''; (x.children || []).forEach(reid); };
    nodes.forEach(reid);
    this.begin();
    target.collapsed = false;
    target.children.push(...nodes);
    this.sel = new Set(nodes.map(x => x.id)); this.anchor = nodes[0].id;
    this.commit();
    this.ensureVisible(nodes[0].id);
  }
  duplicate() {
    if (this.readOnly) return;
    const src = this.node(this.anchor); const p = this.parent(this.anchor);
    if (!src || !p) return;
    const copy = mmClone(src);
    const reid = x => { x.id = mmId(); x.children.forEach(reid); };
    reid(copy);
    this.begin();
    p.children.splice(p.children.indexOf(src) + 1, 0, copy);
    this.sel = new Set([copy.id]); this.anchor = copy.id;
    this.commit();
  }

  // --- inline editing ------------------------------------------------------
  edit(id, replaceWith) {
    if (this.readOnly) return;
    id = id || this.anchor;
    let el = this.elems.get(id);
    if (!el) { // collapsed ancestor or culled off-screen: reveal, pan to it, re-render once
      if (this.revealNode(id)) this.reindex();
      this.render(false); this.ensureVisible(id); this.render(false);
      el = this.elems.get(id);
      if (!el) return;
    }
    if (this.editing && this.editing !== id) this.commitEdit();
    const n = this.node(id);
    this.editing = id;
    const te = el.querySelector('.mm-text');
    el.classList.add('mm-editing');
    el.style.width = '';
    el.style.minWidth = Math.min(MM_MAX_W, Math.max(60, el.offsetWidth)) + 'px';
    te.textContent = replaceWith != null ? replaceWith : n.text;
    try { te.contentEditable = 'plaintext-only'; } catch (e) { /* fallback below */ }
    if (te.contentEditable !== 'plaintext-only') te.contentEditable = 'true';
    te.focus();
    const range = document.createRange(); range.selectNodeContents(te); range.collapse(false);
    const s = window.getSelection(); s.removeAllRanges(); s.addRange(range);
    this._editOrig = n.text;
    this.scheduleRender();
  }
  commitEdit() {
    const id = this.editing; if (!id) return;
    const el = this.elems.get(id); const te = el && el.querySelector('.mm-text');
    const text = te ? te.innerText.replace(/​/g, '') : this._editOrig;
    this.editing = null;
    if (el) { el.classList.remove('mm-editing'); el.style.minWidth = ''; if (te) { te.contentEditable = 'false'; te.removeAttribute('contenteditable'); } el._key = null; }
    const n = this.node(id);
    if (n) { this.invalidateSize(n); if (text.trim() !== n.text) this.setText(id, text); else this.render(false); }
    this.container.focus({ preventScroll: true });
    this.refreshSelection();
  }
  cancelEdit() {
    const id = this.editing; if (!id) return;
    this.editing = null;
    const el = this.elems.get(id);
    if (el) { el.classList.remove('mm-editing'); el.style.minWidth = ''; el._key = null; }
    this.render(false);
    this.container.focus({ preventScroll: true });
    this.refreshSelection();
  }

  // --- search ----------------------------------------------------------------
  search(q) {
    this.query = (q || '').trim().toLowerCase();
    this.matches = [];
    this.matchIdx = -1;
    if (this.query) {
      const w = n => { const hay = [n.text, n.note, (n.tags || []).join(' '), n.task_ref, n.url].join(' ').toLowerCase(); if (hay.includes(this.query)) this.matches.push(n.id); n.children.forEach(w); };
      w(this.doc.root);
      if (this.matches.length) this.nextMatch(0);
    }
    this.render(false);
    this.updateStatus();
  }
  nextMatch(dir) {
    if (!this.matches.length) return;
    this.matchIdx = ((this.matchIdx + (dir == null ? 1 : dir)) % this.matches.length + this.matches.length) % this.matches.length;
    const id = this.matches[this.matchIdx];
    if (this.revealNode(id)) { this.reindex(); this.render(false); }
    this.select(id);
    this.render(false);
    this.centerOn(id, true);
  }

  // --- navigation ------------------------------------------------------------
  navigate(dir) {
    const id = this.anchor || this.doc.root.id;
    const L = this.layoutData; if (!L) return;
    const n = this.node(id), p = this.parent(id);
    const pos = L.pos.get(id); if (!pos) return;
    const sib = d => { if (!p) return null; const i = p.children.indexOf(n), j = i + d; return p.children[j] ? p.children[j].id : null; };
    const firstKid = () => n.children.length && !n.collapsed ? n.children[0].id : null;
    let target = null;
    const lay = this.doc.layout;
    if (lay === 'tree') {
      const i = L.order.indexOf(id);
      if (dir === 'up') target = L.order[i - 1]; else if (dir === 'down') target = L.order[i + 1];
      else if (dir === 'left') target = p ? p.id : null; else target = firstKid();
    } else if (lay === 'down' || lay === 'up') {
      const toward = lay === 'down' ? 'up' : 'down';
      if (dir === toward) target = p ? p.id : null;
      else if (dir === 'up' || dir === 'down') target = firstKid();
      else target = sib(dir === 'left' ? -1 : 1);
    } else {
      const side = pos.side || 'right';
      if (dir === 'up' || dir === 'down') target = sib(dir === 'up' ? -1 : 1);
      else if (!p) { // root: go to the first child on that side
        const kid = n.children.find(c => { const q = L.pos.get(c.id); return q && q.side === dir; });
        target = kid ? kid.id : null;
      } else if ((dir === 'left') === (side === 'right')) target = p.id;
      else target = firstKid();
    }
    if (!target) target = this.nearest(id, dir);
    if (target) { this.select(target); this.ensureVisible(target); }
  }
  nearest(id, dir) {
    const L = this.layoutData, a = L.pos.get(id);
    const ax = a.x + a.w / 2, ay = a.y + a.h / 2;
    let best = null, bd = Infinity;
    for (const [oid, b] of L.pos) {
      if (oid === id) continue;
      const dx = b.x + b.w / 2 - ax, dy = b.y + b.h / 2 - ay;
      const ok = dir === 'left' ? dx < -1 && Math.abs(dy) < Math.abs(dx) * 1.5 : dir === 'right' ? dx > 1 && Math.abs(dy) < Math.abs(dx) * 1.5 : dir === 'up' ? dy < -1 && Math.abs(dx) < Math.abs(dy) * 1.5 : dy > 1 && Math.abs(dx) < Math.abs(dy) * 1.5;
      if (!ok) continue;
      const d = dx * dx + dy * dy;
      if (d < bd) { bd = d; best = oid; }
    }
    return best;
  }

  // --- events ----------------------------------------------------------------
  bindEvents() {
    const c = this.container;
    this.pointers = new Map();
    this.drag = null;
    c.addEventListener('pointerdown', e => this.onPointerDown(e));
    c.addEventListener('pointermove', e => this.onPointerMove(e));
    c.addEventListener('pointerup', e => this.onPointerUp(e));
    c.addEventListener('pointercancel', e => this.onPointerUp(e));
    c.addEventListener('wheel', e => this.onWheel(e), { passive: false });
    c.addEventListener('dblclick', e => this.onDblClick(e));
    c.addEventListener('keydown', e => this.onKey(e));
    c.addEventListener('contextmenu', e => this.onContextMenu(e));
    c.addEventListener('click', e => this.onClick(e));
    c.addEventListener('input', e => { if (this.editing && e.target.closest('.mm-text')) this.scheduleRender(); });
    c.addEventListener('focusin', e => { const el = e.target.closest && e.target.closest('.mm-node'); if (el && !this.sel.has(el.dataset.id) && !this.editing) this.select(el.dataset.id); });
    const onCopy = e => { if (!this.hasFocus() || this.editing) return; const txt = this.copySelection(e.type === 'cut'); if (txt != null) { e.clipboardData.setData('text/plain', txt); e.preventDefault(); } };
    const onPaste = e => { if (!this.hasFocus() || this.editing) return; e.preventDefault(); this.pasteText(e.clipboardData.getData('text/plain')); };
    const onKeyUp = e => { if (e.key === ' ') this.spaceDown = false; };
    const onResize = () => this.drawMinimap();
    document.addEventListener('copy', onCopy); document.addEventListener('cut', onCopy); document.addEventListener('paste', onPaste);
    document.addEventListener('keyup', onKeyUp); window.addEventListener('resize', onResize);
    this.detachGlobal = () => { document.removeEventListener('copy', onCopy); document.removeEventListener('cut', onCopy); document.removeEventListener('paste', onPaste); document.removeEventListener('keyup', onKeyUp); window.removeEventListener('resize', onResize); };
    if (this.minimap) {
      const nav = e => { if (!this._mmScale) return; const r = this.minimap.getBoundingClientRect(); const { s, ox, oy } = this._mmScale; const wx = (e.clientX - r.left - ox) / s, wy = (e.clientY - r.top - oy) / s; const cr = c.getBoundingClientRect(); this.tx = cr.width / 2 - wx * this.k; this.ty = cr.height / 2 - wy * this.k; this.applyView(); };
      this.minimap.addEventListener('pointerdown', e => { e.stopPropagation(); this.minimap.setPointerCapture(e.pointerId); this._mmDrag = true; nav(e); });
      this.minimap.addEventListener('pointermove', e => { if (this._mmDrag) nav(e); });
      this.minimap.addEventListener('pointerup', () => { this._mmDrag = false; });
    }
    const hud = c.querySelector('.mm-hud');
    if (hud) hud.addEventListener('click', e => { const b = e.target.closest('[data-act]'); if (!b) return; e.stopPropagation(); this.hudAction(b.dataset.act); });
  }
  hasFocus() { return this.container.contains(document.activeElement) || document.activeElement === this.container; }
  hudAction(act) {
    if (act === 'zoomIn') this.zoomBy(1.25); else if (act === 'zoomOut') this.zoomBy(0.8);
    else if (act === 'zoom100') { this.k = 1; this.centerOn(this.anchor || this.doc.root.id, true); }
    else if (act === 'fit') this.fit(); else if (act === 'center') this.centerOn(this.doc.root.id, true);
  }
  nodeAt(target) { const el = target && target.closest ? target.closest('.mm-node') : null; return el && this.nodesEl.contains(el) ? el : null; }

  onPointerDown(e) {
    if (e.target.closest('.mm-hud, .mm-minimap')) return;
    this.pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (this.pointers.size === 2) { // pinch start
      const pts = [...this.pointers.values()];
      this.pinch = { d: Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y), cx: (pts[0].x + pts[1].x) / 2, cy: (pts[0].y + pts[1].y) / 2, k: this.k, tx: this.tx, ty: this.ty };
      this.drag = null; this.dropLineEl.classList.add('hidden');
      return;
    }
    const nodeEl = this.nodeAt(e.target);
    const link = e.target.closest && e.target.closest('.mm-rel');
    if (this.editing && !(nodeEl && nodeEl.dataset.id === this.editing)) this.commitEdit();
    if (e.button === 1 || this.spaceDown || (!nodeEl && !link && !e.shiftKey)) {
      if (e.target.closest('.mm-text[contenteditable]')) return;
      this.drag = { kind: 'pan', sx: e.clientX, sy: e.clientY, tx: this.tx, ty: this.ty, moved: false };
      this.container.setPointerCapture(e.pointerId);
      e.preventDefault();
      return;
    }
    if (!nodeEl && e.shiftKey) {
      this.drag = { kind: 'marquee', sx: e.clientX, sy: e.clientY };
      this.container.setPointerCapture(e.pointerId);
      e.preventDefault();
      return;
    }
    if (link) { this.selLink = link.dataset.link; this.sel.clear(); this.anchor = null; this.refreshSelection(); this.container.focus({ preventScroll: true }); e.preventDefault(); return; }
    if (!nodeEl) return;
    const id = nodeEl.dataset.id;
    if (e.target.closest('.mm-fold, .mm-add, .mm-check, .mm-b-url, .mm-b-page, .mm-b-task')) return; // handled on click
    if (this.linkMode) { e.preventDefault(); return; }
    if (this.editing === id) return;
    if (e.shiftKey || e.ctrlKey || e.metaKey) this.select(id, true);
    else if (!this.sel.has(id)) this.select(id);
    this.container.focus({ preventScroll: true });
    if (!this.readOnly && id !== this.doc.root.id) {
      this.drag = { kind: 'node', id, sx: e.clientX, sy: e.clientY, started: false, pointerId: e.pointerId };
      this.container.setPointerCapture(e.pointerId);
    }
    if (e.pointerType === 'touch') { clearTimeout(this._lp); this._lp = setTimeout(() => { if (this.drag && !this.drag.started) { this.drag = null; this.openContextMenu(e.clientX, e.clientY, id); } }, 550); }
    e.preventDefault();
  }
  onPointerMove(e) {
    if (this.pointers.has(e.pointerId)) this.pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (this.pinch && this.pointers.size === 2) {
      const pts = [...this.pointers.values()];
      const d = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y);
      const cx = (pts[0].x + pts[1].x) / 2, cy = (pts[0].y + pts[1].y) / 2;
      const r = this.container.getBoundingClientRect();
      const nk = Math.min(4, Math.max(0.1, this.pinch.k * d / this.pinch.d));
      const px = this.pinch.cx - r.left, py = this.pinch.cy - r.top;
      this.tx = (px - (px - this.pinch.tx) * (nk / this.pinch.k)) + (cx - this.pinch.cx);
      this.ty = (py - (py - this.pinch.ty) * (nk / this.pinch.k)) + (cy - this.pinch.cy);
      this.k = nk; this.applyView();
      return;
    }
    const d = this.drag; if (!d) return;
    if (d.kind === 'pan') {
      this.tx = d.tx + e.clientX - d.sx; this.ty = d.ty + e.clientY - d.sy; d.moved = d.moved || Math.abs(e.clientX - d.sx) + Math.abs(e.clientY - d.sy) > 3;
      this.applyView();
    } else if (d.kind === 'marquee') {
      const r = this.container.getBoundingClientRect();
      const x1 = Math.min(d.sx, e.clientX) - r.left, y1 = Math.min(d.sy, e.clientY) - r.top, w = Math.abs(e.clientX - d.sx), h = Math.abs(e.clientY - d.sy);
      Object.assign(this.marqueeEl.style, { left: x1 + 'px', top: y1 + 'px', width: w + 'px', height: h + 'px' });
      this.marqueeEl.classList.remove('hidden');
      const wx1 = (x1 - this.tx) / this.k, wy1 = (y1 - this.ty) / this.k, wx2 = wx1 + w / this.k, wy2 = wy1 + h / this.k;
      this.sel.clear();
      for (const [id, p] of this.layoutData.pos) if (p.x < wx2 && p.x + p.w > wx1 && p.y < wy2 && p.y + p.h > wy1) this.sel.add(id);
      this.refreshSelection();
    } else if (d.kind === 'node') {
      if (!d.started) {
        if (Math.abs(e.clientX - d.sx) + Math.abs(e.clientY - d.sy) < 7) return;
        d.started = true; clearTimeout(this._lp);
        d.ids = [...this.sel].filter(id => id !== this.doc.root.id);
        if (!d.ids.includes(d.id)) d.ids = [d.id];
        d.sub = new Set(d.ids.flatMap(id => this.subtreeIds(id)));
        const src = this.elems.get(d.id);
        d.ghost = src.cloneNode(true); d.ghost.classList.add('mm-ghost'); d.ghost.style.transform = ''; d.ghost.style.width = src.offsetWidth + 'px';
        d.ghost.style.setProperty('--mm-c', src.style.getPropertyValue('--mm-c'));
        this.container.appendChild(d.ghost);
        d.sub.forEach(id => { const el = this.elems.get(id); if (el) el.classList.add('mm-dragging'); });
        this.container.classList.add('mm-is-dragging');
      }
      const r = this.container.getBoundingClientRect();
      d.ghost.style.left = (e.clientX - r.left + 12) + 'px'; d.ghost.style.top = (e.clientY - r.top + 12) + 'px';
      // auto-pan near edges
      const edge = 28; let px = 0, py = 0;
      if (e.clientX < r.left + edge) px = 12; else if (e.clientX > r.right - edge) px = -12;
      if (e.clientY < r.top + edge) py = 12; else if (e.clientY > r.bottom - edge) py = -12;
      if (px || py) { this.tx += px; this.ty += py; this.applyView(); }
      d.ghost.style.display = 'none';
      const under = document.elementFromPoint(e.clientX, e.clientY);
      d.ghost.style.display = '';
      const tEl = this.nodeAt(under);
      this.nodesEl.querySelectorAll('.mm-drop-child').forEach(x => x.classList.remove('mm-drop-child'));
      this.dropLineEl.classList.add('hidden');
      d.target = null;
      if (!tEl || d.sub.has(tEl.dataset.id)) return;
      const tid = tEl.dataset.id, tr = tEl.getBoundingClientRect();
      const vertical = this.doc.layout === 'down' || this.doc.layout === 'up';
      const rel = vertical ? (e.clientX - tr.left) / tr.width : (e.clientY - tr.top) / tr.height;
      let zone = rel < 0.25 ? 'before' : rel > 0.75 ? 'after' : 'child';
      if (tid === this.doc.root.id) zone = 'child';
      d.target = { id: tid, zone };
      if (zone === 'child') tEl.classList.add('mm-drop-child');
      else {
        const s = this.dropLineEl.style;
        if (vertical) { s.left = ((zone === 'before' ? tr.left : tr.right) - r.left - 1) + 'px'; s.top = (tr.top - r.top) + 'px'; s.width = '3px'; s.height = tr.height + 'px'; }
        else { s.left = (tr.left - r.left) + 'px'; s.top = ((zone === 'before' ? tr.top : tr.bottom) - r.top - 1) + 'px'; s.width = tr.width + 'px'; s.height = '3px'; }
        this.dropLineEl.classList.remove('hidden');
      }
    }
  }
  onPointerUp(e) {
    this.pointers.delete(e.pointerId);
    if (this.pinch) { if (this.pointers.size < 2) this.pinch = null; return; }
    clearTimeout(this._lp);
    const d = this.drag; this.drag = null;
    if (!d) return;
    if (d.kind === 'pan') { if (!d.moved && !e.target.closest('.mm-node, .mm-rel')) { this.select(null); this.selLink = null; this.refreshSelection(); } return; }
    if (d.kind === 'marquee') { this.marqueeEl.classList.add('hidden'); return; }
    if (d.kind === 'node') {
      if (!d.started) { if (!e.shiftKey && !e.ctrlKey && !e.metaKey && this.sel.size > 1) this.select(d.id); return; }
      d.ghost.remove();
      this.nodesEl.querySelectorAll('.mm-dragging, .mm-drop-child').forEach(x => x.classList.remove('mm-dragging', 'mm-drop-child'));
      this.dropLineEl.classList.add('hidden');
      this.container.classList.remove('mm-is-dragging');
      if (d.target) {
        const { id: tid, zone } = d.target;
        let parentId = tid, index = null, side = '';
        if (zone !== 'child') { const tp = this.parent(tid); parentId = tp.id; index = tp.children.findIndex(c => c.id === tid) + (zone === 'after' ? 1 : 0); side = this.node(tid).side || (this.layoutData.pos.get(tid) || {}).side || ''; }
        else if (tid === this.doc.root.id && this.doc.layout === 'balanced') { const rp = this.layoutData.pos.get(tid); side = (e.clientX - this.container.getBoundingClientRect().left - this.tx) / this.k < rp.x + rp.w / 2 ? 'left' : 'right'; }
        // Move every selected top-level node in document order.
        const ordered = this.layoutData.order.filter(id => d.ids.includes(id));
        ordered.reverse().forEach(id => this.moveNode(id, parentId, index, side));
        this.sel = new Set(ordered); this.anchor = d.id; this.refreshSelection();
      } else if (this.doc.layout === 'balanced' && this.parent(d.id) === this.doc.root) {
        // Dropped on empty space: flip the side of a root child.
        const rp = this.layoutData.pos.get(this.doc.root.id);
        const wx = (e.clientX - this.container.getBoundingClientRect().left - this.tx) / this.k;
        const side = wx < rp.x + rp.w / 2 ? 'left' : 'right';
        if (this.node(d.id).side !== side) this.updateNode(d.id, { side });
      }
    }
  }
  onWheel(e) {
    e.preventDefault();
    if (e.ctrlKey || e.metaKey) this.zoomAt(e.clientX, e.clientY, Math.exp(-e.deltaY * 0.0022));
    else { this.tx -= e.shiftKey && !e.deltaX ? e.deltaY : e.deltaX; this.ty -= e.shiftKey && !e.deltaX ? 0 : e.deltaY; this.applyView(); }
  }
  onClick(e) {
    const nodeEl = this.nodeAt(e.target);
    if (!nodeEl) return;
    const id = nodeEl.dataset.id;
    if (this.linkMode) { const from = this.linkMode; this.linkMode = null; this.container.classList.remove('mm-linking'); if (from !== id) this.addLink(from, id); this.select(id); this.updateStatus(); return; }
    if (e.target.closest('.mm-fold')) { e.stopPropagation(); this.toggleFold(id); return; }
    if (e.target.closest('.mm-add')) { e.stopPropagation(); this.select(id); this.addChild(id); return; }
    if (e.target.closest('.mm-check')) { e.stopPropagation(); if (!this.readOnly) this.toggleTask(id); return; }
    const pg = e.target.closest('.mm-b-page'); if (pg) { e.stopPropagation(); if (this.opts.onOpenPage) this.opts.onOpenPage(Number(pg.dataset.page)); return; }
    const tk = e.target.closest('.mm-b-task'); if (tk) { e.stopPropagation(); if (this.opts.onOpenTask) this.opts.onOpenTask(tk.dataset.task); return; }
  }
  onDblClick(e) {
    const nodeEl = this.nodeAt(e.target);
    if (nodeEl) { if (e.target.closest('.mm-fold, .mm-add, .mm-check, .mm-b')) return; this.select(nodeEl.dataset.id); this.edit(nodeEl.dataset.id); return; }
    const link = e.target.closest && e.target.closest('.mm-rel');
    if (link && !this.readOnly) { const l = this.doc.links.find(x => x.id === link.dataset.link); uiPrompt(mmT('mmRelationLabel'), l ? l.label : '').then(v => { if (v !== null) this.setLinkLabel(link.dataset.link, v.trim()); }); return; }
    if (!nodeEl && !link) this.fit();
  }
  onContextMenu(e) {
    e.preventDefault();
    const nodeEl = this.nodeAt(e.target);
    if (nodeEl && !this.sel.has(nodeEl.dataset.id)) this.select(nodeEl.dataset.id);
    this.openContextMenu(e.clientX, e.clientY, nodeEl ? nodeEl.dataset.id : null);
  }
  onKey(e) {
    const k = e.key, ctrl = e.ctrlKey || e.metaKey;
    if (this.editing) {
      if (k === 'Escape') { e.preventDefault(); this.cancelEdit(); }
      else if (k === 'Enter' && !e.shiftKey && !e.altKey) { e.preventDefault(); this.commitEdit(); }
      else if (k === 'Tab') { e.preventDefault(); this.commitEdit(); this.addChild(this.anchor); }
      return;
    }
    if (e.target.closest && e.target.closest('input, textarea, select')) return;
    const ro = this.readOnly;
    const stop = () => { e.preventDefault(); e.stopPropagation(); };
    if (k === 'Escape') { stop(); if (this.linkMode) { this.linkMode = null; this.container.classList.remove('mm-linking'); this.updateStatus(); } else if (this.query && this.opts.onClearSearch) this.opts.onClearSearch(); else { this.select(null); } return; }
    if (k === 'Tab' && !ro) { stop(); this.addChild(this.anchor); return; }
    if (k === 'Enter' && !ro) { stop(); if (ctrl) this.insertParent(this.anchor); else if (e.altKey) this.toggleTask(this.anchor); else if (this.anchor) this.addSibling(this.anchor, e.shiftKey); return; }
    if ((k === 'F2' || k === ' ') && !ro && this.anchor) { stop(); this.edit(this.anchor); return; }
    if (k === ' ') { stop(); this.spaceDown = true; return; }
    if ((k === 'Delete' || k === 'Backspace') && !ro) { stop(); this.selLink ? this.removeLink(this.selLink) : this.deleteSelected(); return; }
    if (k === 'ArrowUp' || k === 'ArrowDown' || k === 'ArrowLeft' || k === 'ArrowRight') {
      stop();
      const dir = k.slice(5).toLowerCase();
      if (e.altKey && !ro && (dir === 'up' || dir === 'down')) this.moveSibling(this.anchor, dir === 'up' ? -1 : 1);
      else if (ctrl || e.shiftKey && !this.anchor) { const s = 60; this.tx += dir === 'left' ? s : dir === 'right' ? -s : 0; this.ty += dir === 'up' ? s : dir === 'down' ? -s : 0; this.applyView(); }
      else this.navigate(dir);
      return;
    }
    if (k === '/' && !ctrl) { stop(); this.toggleFold(this.anchor); return; }
    if (k === '?' || (k === '/' && e.shiftKey)) { stop(); if (this.opts.onHelp) this.opts.onHelp(); return; }
    if (ctrl && (k === 'z' || k === 'Z' || k === 'я' || k === 'Я')) { stop(); e.shiftKey ? this.redo() : this.undo(); return; }
    if (ctrl && (k === 'y' || k === 'Y' || k === 'н')) { stop(); this.redo(); return; }
    if (ctrl && (k === 'a' || k === 'A' || k === 'ф')) { stop(); this.selectAll(); return; }
    if (ctrl && (k === 'd' || k === 'D' || k === 'в') && !ro) { stop(); this.duplicate(); return; }
    if (ctrl && (k === 'f' || k === 'F' || k === 'а')) { stop(); if (this.opts.onFind) this.opts.onFind(); return; }
    if (ctrl && (k === 'i' || k === 'I' || k === 'ш')) { stop(); if (this.opts.onProps) this.opts.onProps(); return; }
    if (ctrl && (k === 'k' || k === 'K' || k === 'л') && !ro) { stop(); if (this.opts.onProps) this.opts.onProps('url'); return; }
    if (ctrl && (k === 's' || k === 'S' || k === 'ы')) { stop(); if (this.opts.onSave) this.opts.onSave(); return; }
    if (ctrl && e.shiftKey && (k === 'L' || k === 'l' || k === 'Д' || k === 'д') && !ro) { stop(); this.startLink(); return; }
    if (ctrl && (k === '=' || k === '+')) { stop(); this.zoomBy(1.25); return; }
    if (ctrl && k === '-') { stop(); this.zoomBy(0.8); return; }
    if (ctrl && k === '0') { stop(); this.k = 1; this.centerOn(this.anchor || this.doc.root.id, true); return; }
    if (e.shiftKey && !ctrl && (e.code === 'Digit1' || e.code === 'Digit2' || e.code === 'Digit0')) {
      stop();
      if (e.code === 'Digit1') this.fit(); else if (e.code === 'Digit2') this.fit([...this.sel]); else { this.k = 1; this.centerOn(this.anchor || this.doc.root.id, true); }
      return;
    }
    if (e.altKey && !ctrl && /^[0-6]$/.test(k) && !ro) { stop(); const n = this.node(this.anchor); if (n) this.updateNode(n.id, { priority: Number(k) }); return; }
    if (k === 'Home') { stop(); this.select(this.doc.root.id); this.centerOn(this.doc.root.id, true); return; }
    // Start typing on a selected node to replace its text.
    if (!ro && this.anchor && k.length === 1 && !ctrl && !e.altKey) { stop(); this.edit(this.anchor, k); return; }
  }
  startLink() { if (this.readOnly || !this.anchor) return; this.linkMode = this.anchor; this.container.classList.add('mm-linking'); this.updateStatus(); this.announce(mmT('mmRelationHint')); }

  // --- context menu ----------------------------------------------------------
  closeMenus() { document.querySelectorAll('.mm-menu').forEach(m => m.remove()); }
  openContextMenu(x, y, id) {
    this.closeMenus();
    const items = [];
    const ro = this.readOnly;
    const n = id ? this.node(id) : null;
    const add = (label, fn, opts) => items.push({ label, fn, ...(opts || {}) });
    if (n) {
      if (!ro) {
        add(mmT('mmNewChild') + '<kbd>Tab</kbd>', () => this.addChild(id));
        if (n !== this.doc.root) add(mmT('mmNewSibling') + '<kbd>Enter</kbd>', () => this.addSibling(id));
        if (n !== this.doc.root) add(mmT('mmInsertParent') + '<kbd>Ctrl+Enter</kbd>', () => this.insertParent(id));
        add(mmT('mmEdit') + '<kbd>F2</kbd>', () => this.edit(id));
        items.push({ sep: true });
        add(mmT('mmCopy') + '<kbd>Ctrl+C</kbd>', () => { const txt = this.copySelection(false); if (txt && navigator.clipboard) navigator.clipboard.writeText(txt).catch(() => {}); });
        if (n !== this.doc.root) add(mmT('mmCut') + '<kbd>Ctrl+X</kbd>', () => { const txt = this.copySelection(true); if (txt && navigator.clipboard) navigator.clipboard.writeText(txt).catch(() => {}); });
        add(mmT('mmPaste') + '<kbd>Ctrl+V</kbd>', () => { if (navigator.clipboard && navigator.clipboard.readText) navigator.clipboard.readText().then(tx => this.pasteText(tx)).catch(() => this.pasteText(null)); else this.pasteText(null); });
        if (n !== this.doc.root) add(mmT('mmDuplicate') + '<kbd>Ctrl+D</kbd>', () => this.duplicate());
        items.push({ sep: true });
        add(mmT('mmTask'), () => this.toggleTask(id), { check: n.done != null });
        add(mmT('mmBoundary'), () => this.updateNode(id, { boundary: !n.boundary }), { check: !!n.boundary });
        add(mmT('mmAddRelation') + '<kbd>Ctrl+Shift+L</kbd>', () => this.startLink());
        add(mmT('mmProperties') + '<kbd>Ctrl+I</kbd>', () => { if (this.opts.onProps) this.opts.onProps(); });
        items.push({ sep: true });
      }
      if (n.children.length) add((n.collapsed ? mmT('mmUnfold') : mmT('mmFold')) + '<kbd>/</kbd>', () => this.toggleFold(id));
      add(mmT('mmCenterNode'), () => this.centerOn(id, true));
      if (n.url) add(mmT('mmOpenLink'), () => window.open(n.url, '_blank', 'noopener'));
      if (n.page_id && this.opts.onOpenPage) add(mmT('mmOpenPage'), () => this.opts.onOpenPage(n.page_id));
      if (n.task_ref && this.opts.onOpenTask) add(mmT('mmOpenTask'), () => this.opts.onOpenTask(n.task_ref));
      if (!ro && n !== this.doc.root) { items.push({ sep: true }); add(mmT('mmDelete') + '<kbd>Del</kbd>', () => this.deleteSelected(), { danger: true }); }
    } else if (this.selLink) {
      if (!ro) { add(mmT('mmRelationLabel'), () => { const l = this.doc.links.find(x => x.id === this.selLink); uiPrompt(mmT('mmRelationLabel'), l ? l.label : '').then(v => { if (v !== null) this.setLinkLabel(l.id, v.trim()); }); }); add(mmT('mmRemoveRelation'), () => this.removeLink(this.selLink), { danger: true }); }
    } else {
      add(mmT('mmFit') + '<kbd>Shift+1</kbd>', () => this.fit());
      add(mmT('mmUnfoldAll'), () => this.foldAll(Infinity));
      add(mmT('mmFoldAll'), () => this.foldAll(1));
      if (!ro) add(mmT('mmPaste') + '<kbd>Ctrl+V</kbd>', () => { if (navigator.clipboard && navigator.clipboard.readText) navigator.clipboard.readText().then(tx => this.pasteText(tx)).catch(() => this.pasteText(null)); else this.pasteText(null); });
    }
    if (!items.length) return;
    const m = document.createElement('div');
    m.className = 'mm-menu';
    m.innerHTML = items.map((it, i) => it.sep ? '<div class="mm-menu-sep"></div>' : `<button class="mm-menu-item${it.danger ? ' danger' : ''}" data-i="${i}">${it.check != null ? `<span class="mm-menu-check">${it.check ? '✓' : ''}</span>` : ''}${it.label}</button>`).join('');
    document.body.appendChild(m);
    const W = m.offsetWidth, H = m.offsetHeight;
    m.style.left = Math.max(4, Math.min(x, window.innerWidth - W - 4)) + 'px';
    m.style.top = Math.max(4, Math.min(y, window.innerHeight - H - 4)) + 'px';
    m.addEventListener('click', e => { const b = e.target.closest('[data-i]'); if (!b) return; this.closeMenus(); items[Number(b.dataset.i)].fn(); });
    setTimeout(() => { const close = ev => { if (!m.contains(ev.target)) { this.closeMenus(); document.removeEventListener('pointerdown', close, true); } }; document.addEventListener('pointerdown', close, true); }, 0);
    document.addEventListener('keydown', function esc(ev) { if (ev.key === 'Escape') { document.querySelectorAll('.mm-menu').forEach(x => x.remove()); document.removeEventListener('keydown', esc, true); } }, true);
  }

  // --- properties panel ------------------------------------------------------
  // renderProps fills `el` with editable fields of the anchor node.
  renderProps(el, focusField) {
    const n = this.node(this.anchor);
    if (!n) { el.innerHTML = `<div class="mm-panel-empty">${esc(mmT('mmRootHint'))}</div>`; return; }
    const ro = this.readOnly;
    const dis = ro ? 'disabled' : '';
    const isRoot = n === this.doc.root;
    const pal = this.paletteColors().colors;
    const swatches = [...new Set([...pal, '#e74c3c', '#e67e22', '#f1c40f', '#2ecc71', '#3498db', '#9b59b6', '#95a5a6', '#2d3436'])].slice(0, 12);
    el.innerHTML = `
      <label class="mm-f">${esc(mmT('mmText'))}<textarea id="mmp-text" rows="2" ${dis}>${esc(n.text)}</textarea></label>
      <label class="mm-f">${esc(mmT('mmNote'))}<textarea id="mmp-note" rows="4" ${dis} placeholder="Markdown">${esc(n.note || '')}</textarea></label>
      ${n.note ? `<div class="mm-note-preview md-content" id="mmp-note-prev"></div>` : ''}
      <label class="mm-f">${esc(mmT('mmLink'))}<input id="mmp-url" type="url" value="${escAttr(n.url || '')}" placeholder="https://" ${dis}></label>
      <label class="mm-f">${esc(mmT('mmWikiPage'))}<div class="mm-pagepick"><input id="mmp-page" placeholder="${escAttr(mmT('mmLinkTo'))}" value="${escAttr(n._page_title || (n.page_id ? '#' + n.page_id : ''))}" ${dis} autocomplete="off">${n.page_id ? `<button class="btn-icon" id="mmp-page-clear" title="×">×</button>` : ''}<div class="mm-pagepick-list hidden" id="mmp-page-list"></div></div></label>
      <label class="mm-f">${esc(mmT('mmTaskRef'))}<input id="mmp-task" value="${escAttr(n.task_ref || '')}" placeholder="KEY-123" ${dis}></label>
      <div class="mm-f"><span>${esc(mmT('mmIcon'))}</span><div class="mm-icons">${MM_ICONS.map(i => `<button class="mm-icon-btn${n.icon === i ? ' on' : ''}" data-icon="${escAttr(i)}" ${dis}>${i}</button>`).join('')}<input id="mmp-icon" value="${escAttr(n.icon || '')}" maxlength="8" placeholder="…" ${dis} style="width:3.2em"></div></div>
      ${isRoot ? '' : `<div class="mm-f"><span>${esc(mmT('mmColor'))}</span><div class="mm-swatches"><button class="mm-swatch mm-swatch-none${!n.color ? ' on' : ''}" data-color="" title="${escAttr(mmT('mmInherit'))}" ${dis}>∅</button>${swatches.map(c => `<button class="mm-swatch${n.color === c ? ' on' : ''}" data-color="${c}" style="background:${c}" ${dis}></button>`).join('')}<input type="color" id="mmp-color" value="${escAttr(n.color || '#888888')}" ${dis}></div></div>`}
      <div class="mm-f mm-f-row"><label>${esc(mmT('mmShape'))}<select id="mmp-shape" ${dis}>${[['', 'mmShapeDefault'], ['rect', 'mmShapeRect'], ['pill', 'mmShapePill'], ['underline', 'mmShapeUnderline']].map(([v, k]) => `<option value="${v}" ${n.shape === v || (!n.shape && !v) ? 'selected' : ''}>${esc(mmT(k))}</option>`).join('')}</select></label>
      ${!isRoot && this.doc.layout === 'balanced' && this.parent(n.id) === this.doc.root ? `<label>${esc(mmT('mmSide'))}<select id="mmp-side" ${dis}>${[['', 'mmSideAuto'], ['left', 'mmSideLeft'], ['right', 'mmSideRight']].map(([v, k]) => `<option value="${v}" ${(n.side || '') === v ? 'selected' : ''}>${esc(mmT(k))}</option>`).join('')}</select></label>` : ''}</div>
      <div class="mm-f"><span>${esc(mmT('mmPriority'))}</span><div class="mm-prio-row"><button class="mm-prio-btn${!n.priority ? ' on' : ''}" data-p="0" ${dis}>${esc(mmT('mmNone'))}</button>${[1, 2, 3, 4, 5, 6].map(p => `<button class="mm-prio-btn mm-prio-${p}${n.priority === p ? ' on' : ''}" data-p="${p}" ${dis}>${p}</button>`).join('')}</div></div>
      <div class="mm-f"><span>${esc(mmT('mmProgress'))}: <b id="mmp-prog-v">${n.progress == null ? esc(mmT('mmNone')) : n.progress + '%'}</b></span><input type="range" id="mmp-prog" min="-25" max="100" step="25" value="${n.progress == null ? -25 : n.progress}" ${dis}></div>
      <label class="mm-f mm-f-check"><input type="checkbox" id="mmp-task-cb" ${n.done != null ? 'checked' : ''} ${dis}> ${esc(mmT('mmTask'))}</label>
      <label class="mm-f mm-f-check"><input type="checkbox" id="mmp-bound" ${n.boundary ? 'checked' : ''} ${dis}> ${esc(mmT('mmBoundary'))}</label>
      <label class="mm-f">${esc(mmT('mmTags'))} <small>${esc(mmT('mmTagsHint'))}</small><input id="mmp-tags" value="${escAttr((n.tags || []).join(', '))}" ${dis}></label>
      <div class="mm-f"><span>${esc(mmT('mmImage'))}</span>${n.image ? `<div class="mm-img-prev"><img src="${escAttr(n.image)}" alt=""><button class="btn btn-ghost btn-sm" id="mmp-img-rm" ${dis}>${esc(mmT('mmRemoveImage'))}</button></div>` : ro ? '' : `<button class="btn btn-ghost btn-sm" id="mmp-img-add">${esc(mmT('mmImage'))}…</button>`}</div>`;
    if (ro) return;
    const q = s => el.querySelector(s);
    const apply = patch => this.updateNode(n.id, patch);
    const onBlurCommit = (sel, key, map) => { const i = q(sel); if (!i) return; i.addEventListener('change', () => apply({ [key]: map ? map(i.value) : i.value.trim() })); };
    q('#mmp-text').addEventListener('change', () => this.setText(n.id, q('#mmp-text').value));
    q('#mmp-text').addEventListener('keydown', e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); e.target.blur(); } });
    onBlurCommit('#mmp-note', 'note');
    onBlurCommit('#mmp-url', 'url');
    onBlurCommit('#mmp-task', 'task_ref', v => v.trim().toUpperCase());
    onBlurCommit('#mmp-icon', 'icon');
    onBlurCommit('#mmp-shape', 'shape');
    if (q('#mmp-side')) onBlurCommit('#mmp-side', 'side');
    onBlurCommit('#mmp-tags', 'tags', v => v.split(',').map(x => x.trim()).filter(Boolean));
    const prev = q('#mmp-note-prev');
    if (prev && typeof wikiMdToHtml === 'function') prev.innerHTML = wikiSanitizePreview(wikiMdToHtml(n.note));
    el.querySelectorAll('.mm-icon-btn').forEach(b => b.addEventListener('click', () => apply({ icon: n.icon === b.dataset.icon ? '' : b.dataset.icon })));
    el.querySelectorAll('.mm-swatch').forEach(b => b.addEventListener('click', () => apply({ color: b.dataset.color })));
    if (q('#mmp-color')) q('#mmp-color').addEventListener('change', () => apply({ color: q('#mmp-color').value }));
    el.querySelectorAll('.mm-prio-btn').forEach(b => b.addEventListener('click', () => apply({ priority: Number(b.dataset.p) })));
    q('#mmp-prog').addEventListener('input', () => { const v = Number(q('#mmp-prog').value); q('#mmp-prog-v').textContent = v < 0 ? mmT('mmNone') : v + '%'; });
    q('#mmp-prog').addEventListener('change', () => { const v = Number(q('#mmp-prog').value); apply({ progress: v < 0 ? null : v }); });
    q('#mmp-task-cb').addEventListener('change', () => apply({ done: q('#mmp-task-cb').checked ? (n.done || false) : null }));
    q('#mmp-bound').addEventListener('change', () => apply({ boundary: q('#mmp-bound').checked }));
    if (q('#mmp-img-rm')) q('#mmp-img-rm').addEventListener('click', () => apply({ image: '' }));
    if (q('#mmp-img-add')) q('#mmp-img-add').addEventListener('click', () => {
      const inp = document.createElement('input'); inp.type = 'file'; inp.accept = 'image/*';
      inp.onchange = async () => { const f = inp.files[0]; if (!f || !this.opts.uploadImage) return; try { const url = await this.opts.uploadImage(f); if (url) apply({ image: url }); } catch (e) { toast(e.message, 'error'); } };
      inp.click();
    });
    if (q('#mmp-page-clear')) q('#mmp-page-clear').addEventListener('click', () => apply({ page_id: 0, _page_title: '' }));
    const pi = q('#mmp-page'), pl = q('#mmp-page-list');
    let timer = null;
    pi.addEventListener('input', () => {
      clearTimeout(timer);
      const v = pi.value.trim();
      if (!v) { pl.classList.add('hidden'); return; }
      timer = setTimeout(async () => {
        let hits = [];
        try { hits = await api('/api/wiki/search?q=' + encodeURIComponent(v)); } catch (e) { hits = []; }
        pl.innerHTML = hits.length ? hits.slice(0, 12).map(h => `<div class="mm-pagepick-item" data-id="${h.page_id}" data-title="${escAttr(h.title)}">${esc(h.space_key)} / ${esc(h.title)}</div>`).join('') : `<div class="mm-pagepick-item muted">${esc(mmT('mmNoResults'))}</div>`;
        pl.classList.remove('hidden');
      }, 250);
    });
    pl.addEventListener('mousedown', e => { const it = e.target.closest('[data-id]'); if (!it) return; e.preventDefault(); n._page_title = it.dataset.title; apply({ page_id: Number(it.dataset.id) }); });
    pi.addEventListener('blur', () => setTimeout(() => pl.classList.add('hidden'), 150));
    if (focusField === 'url') q('#mmp-url').focus();
  }

  // --- outline panel -----------------------------------------------------------
  renderOutline(el) {
    const ro = this.readOnly;
    const list = (nodes, depth) => nodes.map(n => `<div class="mm-ol-item${this.sel.has(n.id) ? ' on' : ''}" data-id="${n.id}" style="--d:${depth}">
        ${n.children.length ? `<span class="mm-ol-tg ${n.collapsed ? 'closed' : ''}" data-tg="${n.id}">${n.collapsed ? '▸' : '▾'}</span>` : '<span class="mm-ol-tg-sp"></span>'}
        ${n.done != null ? `<span class="mm-ol-cb${n.done ? ' on' : ''}"></span>` : ''}${n.icon ? esc(n.icon) + ' ' : ''}<span class="mm-ol-text">${esc(mmFirstLine(n.text)) || '…'}</span>
        ${n.priority ? `<span class="mm-b mm-b-prio mm-prio-${n.priority}">${n.priority}</span>` : ''}
      </div>${n.collapsed ? '' : list(n.children, depth + 1)}`).join('');
    el.innerHTML = `<div class="mm-ol-tools">${ro ? '' : `<button class="btn btn-ghost btn-sm" id="mm-ol-text">${esc(mmT('mmEditAsText'))}</button>`}</div><div class="mm-ol-list">${list([this.doc.root], 0)}</div>`;
    el.querySelector('.mm-ol-list').addEventListener('click', e => {
      const tg = e.target.closest('[data-tg]'); if (tg) { this.toggleFold(tg.dataset.tg); this.renderOutline(el); return; }
      const it = e.target.closest('.mm-ol-item'); if (!it) return;
      this.select(it.dataset.id); this.ensureVisible(it.dataset.id); this.renderOutline(el);
    });
    el.querySelector('.mm-ol-list').addEventListener('dblclick', e => { const it = e.target.closest('.mm-ol-item'); if (it && !ro) { this.select(it.dataset.id); this.centerOn(it.dataset.id, true); this.edit(it.dataset.id); } });
    const tb = el.querySelector('#mm-ol-text');
    if (tb) tb.addEventListener('click', () => this.renderOutlineText(el));
  }
  renderOutlineText(el) {
    const text = '# ' + mmFirstLine(this.doc.root.text) + '\n' + mmOutlineText(this.doc.root.children);
    el.innerHTML = `<div class="mm-ol-hint">${esc(mmT('mmOutlineHint'))}</div><textarea class="mm-ol-ta" spellcheck="false">${esc(text)}</textarea>
      <div class="mm-ol-tools"><button class="btn btn-primary btn-sm" id="mm-ol-apply">${esc(mmT('mmApply'))}</button><button class="btn btn-ghost btn-sm" id="mm-ol-cancel">${esc(mmT('mmCancel'))}</button></div>`;
    const ta = el.querySelector('textarea');
    ta.focus();
    ta.addEventListener('keydown', e => { if (e.key === 'Tab') { e.preventDefault(); const s = ta.selectionStart, en = ta.selectionEnd; if (e.shiftKey) { const ls = ta.value.lastIndexOf('\n', s - 1) + 1; if (ta.value.slice(ls, ls + 2) === '  ') { ta.setRangeText('', ls, ls + 2, 'end'); } } else ta.setRangeText('  ', s, en, 'end'); } });
    el.querySelector('#mm-ol-cancel').addEventListener('click', () => this.renderOutline(el));
    el.querySelector('#mm-ol-apply').addEventListener('click', async () => {
      const kids = mmParseOutline(ta.value);
      let rootText = this.doc.root.text;
      if (kids.length === 1 && /^#\s/.test(ta.value.trim())) { rootText = kids[0].text; const inner = kids[0].children; kids.splice(0, 1, ...inner); }
      if (this.opts.resolvePage) { const walk = async x => { if (x._page) { x.page_id = await this.opts.resolvePage(x._page); } delete x._page; for (const c of x.children) await walk(c); }; for (const k of kids) await walk(k); }
      this.replaceChildren(this.doc.root.id, kids);
      if (rootText !== this.doc.root.text) this.setText(this.doc.root.id, rootText);
      toast(mmT('mmApplied'), 'success');
      this.renderOutline(el);
      this.fit();
    });
  }

  // --- export ------------------------------------------------------------------
  // toSVG renders the current layout as a standalone SVG string (fonts fall
  // back to sans-serif; text is re-wrapped with canvas metrics so it fits).
  toSVG() {
    const L = this.layoutData; if (!L) return '';
    const bg = mmCssVar('--bg', '#ffffff'), fg = mmCssVar('--fg', '#222'), bg2 = mmCssVar('--bg2', '#f4f4f4');
    const pad = 30, b = L.bbox;
    const ctx = document.createElement('canvas').getContext('2d');
    const font = (size, bold) => `${bold ? '600 ' : ''}${size}px Arial, Helvetica, sans-serif`;
    const wrap = (text, maxW, f) => { ctx.font = f; const out = []; for (const para of String(text).split('\n')) { let line = ''; for (const w of para.split(/\s+/)) { const tst = line ? line + ' ' + w : w; if (ctx.measureText(tst).width > maxW && line) { out.push(line); line = w; } else line = tst; } out.push(line); } return out; };
    let s = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${b.x - pad} ${b.y - pad} ${b.w + 2 * pad} ${b.h + 2 * pad}" width="${Math.round(b.w + 2 * pad)}" height="${Math.round(b.h + 2 * pad)}" font-family="Arial, Helvetica, sans-serif">
<rect x="${b.x - pad}" y="${b.y - pad}" width="${b.w + 2 * pad}" height="${b.h + 2 * pad}" fill="${bg}"/>
<defs><marker id="arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0 L10 5 L0 10 z" fill="${fg}"/></marker></defs>\n`;
    s += this.gBounds.innerHTML + '\n';
    for (const id of L.order) { const p = L.pos.get(id); if (!p.parent) continue; const pp = L.pos.get(p.parent); s += `<path d="${mmEdgePath(this.doc.line, this.doc.layout, pp, p)}" stroke="${p.color}" stroke-width="${p.depth === 1 ? 3 : 2}" fill="none" stroke-linecap="round"/>\n`; }
    for (const l of this.doc.links) { const a = L.pos.get(l.from), c = L.pos.get(l.to); if (!a || !c) continue; const r = mmRelPath(a, c); s += `<path d="${r.d}" stroke="${fg}" stroke-opacity=".6" stroke-width="1.5" stroke-dasharray="6 4" fill="none" marker-end="url(#arr)"/>${l.label ? `<text x="${r.mx}" y="${r.my - 4}" text-anchor="middle" font-size="11" fill="${fg}">${esc(l.label)}</text>` : ''}\n`; }
    for (const id of L.order) {
      const p = L.pos.get(id), n = this.index.get(id), isRoot = id === this.doc.root.id;
      const size = isRoot ? 17 : p.depth === 1 ? 14.5 : 13.5, bold = isRoot || p.depth === 1;
      const rx = n.shape === 'rect' ? 2 : n.shape === 'pill' ? p.h / 2 : isRoot ? 12 : 7;
      const fill = isRoot ? p.color : n.shape === 'underline' ? 'none' : bg2;
      const stroke = isRoot ? p.color : p.color;
      s += n.shape === 'underline' ? `<line x1="${p.x}" y1="${p.y + p.h}" x2="${p.x + p.w}" y2="${p.y + p.h}" stroke="${stroke}" stroke-width="2.5"/>` : `<rect x="${p.x}" y="${p.y}" width="${p.w}" height="${p.h}" rx="${rx}" fill="${fill}" stroke="${stroke}" stroke-width="${p.depth === 1 ? 2 : 1.2}"/>`;
      let prefix = '';
      if (n.done != null) prefix += n.done ? '☑ ' : '☐ ';
      if (n.icon) prefix += n.icon + ' ';
      let suffix = '';
      if (n.priority) suffix += ` ⓟ${n.priority}`;
      if (n.progress != null) suffix += ` ${n.progress}%`;
      if (n.url) suffix += ' 🔗';
      const lines = wrap(prefix + n.text + suffix, p.w - 22, font(size, bold));
      const tags = (n.tags || []).length ? '#' + n.tags.join(' #') : '';
      const lh = size * 1.3, total = lines.length * lh + (tags ? 13 : 0);
      let y = p.y + (p.h - total) / 2 + lh * 0.8;
      for (const ln of lines) { s += `<text x="${p.x + p.w / 2}" y="${y}" text-anchor="middle" font-size="${size}" ${bold ? 'font-weight="600"' : ''} fill="${isRoot ? '#fff' : fg}">${esc(ln)}</text>`; y += lh; }
      if (tags) s += `<text x="${p.x + p.w / 2}" y="${y - lh + 11}" text-anchor="middle" font-size="10" fill="${p.color}">${esc(tags)}</text>`;
      s += '\n';
    }
    return s + '</svg>';
  }
  downloadSVG(name) {
    const blob = new Blob([this.toSVG()], { type: 'image/svg+xml;charset=utf-8' });
    mmDownloadBlob(blob, (name || 'mindmap') + '.svg');
  }
  downloadPNG(name) {
    const svg = this.toSVG();
    const img = new Image();
    const url = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml;charset=utf-8' }));
    img.onload = () => {
      const scale = 2, cv = document.createElement('canvas');
      cv.width = img.width * scale; cv.height = img.height * scale;
      const ctx = cv.getContext('2d'); ctx.scale(scale, scale); ctx.drawImage(img, 0, 0);
      URL.revokeObjectURL(url);
      cv.toBlob(bl => mmDownloadBlob(bl, (name || 'mindmap') + '.png'), 'image/png');
    };
    img.onerror = () => { URL.revokeObjectURL(url); toast('PNG export failed', 'error'); };
    img.src = url;
  }
  print() {
    const w = window.open('', '_blank');
    if (!w) return;
    w.document.write(`<!DOCTYPE html><html><head><title>${esc(mmFirstLine(this.doc.root.text))}</title><style>body{margin:0}svg{max-width:100%;height:auto}</style></head><body>${this.toSVG()}</body></html>`);
    w.document.close();
    setTimeout(() => { w.focus(); w.print(); }, 300);
  }
}

function mmDownloadBlob(blob, name) {
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob); a.download = name;
  document.body.appendChild(a); a.click();
  setTimeout(() => { URL.revokeObjectURL(a.href); a.remove(); }, 1000);
}

// === 4. Page glue (wiki integration) ===
let mmPage = null; // the active page controller (one mind map open at a time)

// wikiRenderMindmapPage renders a kind=mindmap page into the wiki main panel:
// header, toolbar, canvas + side panel, status bar and comments below.
function wikiRenderMindmapPage(el) {
  ensureMindmapI18n();
  const p = wikiState.page;
  const canEdit = wikiCanEdit();
  if (mmPage) { mmPage.destroy(); mmPage = null; }
  el.innerHTML = `
    ${wikiBreadcrumbs()}
    <div class="wiki-page-header mm-page-header">
      <h1 class="wiki-title mm-title" ${canEdit ? 'contenteditable="plaintext-only" spellcheck="false"' : ''} title="${canEdit ? escAttr(mmT('mmEdit')) : ''}">${esc(p.title)}</h1>
      <div class="wiki-actions">
        ${canEdit ? `<button class="btn btn-primary btn-sm" id="mm-save-version" title="Ctrl+S">${esc(mmT('mmSaveVersion'))}</button>` : `<span class="mm-ro-badge">${esc(mmT('mmReadOnly'))}</span>`}
        ${wikiState.llmEnabled ? `<button class="btn btn-ghost btn-sm" onclick="wikiSummarize(${p.id})">&#10024; ${t('wikiSummary')}</button>` : ''}
        <button class="btn btn-ghost btn-sm" onclick="navigateTo('/wiki/page/${p.id}/history')">${t('wikiHistory')}</button>
        <div class="wiki-menu-wrap">
          <button class="btn btn-ghost btn-sm" onclick="wikiToggleMenu(event)">&#8943;</button>
          <div class="wiki-menu hidden">
            <a onclick="mmPage.exportAs('svg')">&#11015; ${esc(mmT('mmExportSVG'))}</a>
            <a onclick="mmPage.exportAs('png')">&#11015; ${esc(mmT('mmExportPNG'))}</a>
            <a onclick="mmPage.exportAs('md')">&#11015; ${esc(mmT('mmExportMd'))}</a>
            <a onclick="mmPage.exportAs('opml')">&#11015; ${esc(mmT('mmExportOPML'))}</a>
            <a onclick="mmPage.exportAs('mm')">&#11015; ${esc(mmT('mmExportFreeMind'))}</a>
            <a onclick="mmPage.exportAs('xmind')">&#11015; ${esc(mmT('mmExportXMind'))}</a>
            <a onclick="mmPage.exportAs('mermaid')">&#11015; ${esc(mmT('mmExportMermaid'))}</a>
            <a onclick="mmPage.exportAs('json')">&#11015; ${esc(mmT('mmExportJSON'))}</a>
            <a onclick="mmPage.map.print()">&#128424; ${esc(mmT('mmPrint'))}</a>
            ${canEdit ? `<div class="wiki-menu-sep"></div>
            <a onclick="mmPage.importDialog()">&#11014; ${esc(mmT('mmImport'))}</a>
            <a onclick="wikiNewPage(${p.id})">${t('wikiAddChild')}</a>
            <a onclick="wikiMovePage(${p.id},-1)">&#9650; ${t('wikiMoveUp')}</a>
            <a onclick="wikiMovePage(${p.id},1)">&#9660; ${t('wikiMoveDown')}</a>
            <a onclick="wikiIndentPage(${p.id})">&#8618; ${t('wikiIndent')}</a>
            <a onclick="wikiOutdentPage(${p.id})">&#8617; ${t('wikiOutdent')}</a>
            <a onclick="wikiToggleTemplate(${p.id})">&#128209; ${p.is_template ? t('wikiUnmarkTemplate') : t('wikiMarkTemplate')}</a>
            <a onclick="wikiTogglePin(${p.id},false)">&#128204; ${p.pinned_by_me ? t('wikiUnpin') : t('wikiPin')}</a>
            ${wikiRoleIsAdmin() ? `<a onclick="wikiTogglePin(${p.id},true)">&#128204; ${p.pinned_shared ? t('wikiUnpinShared') : t('wikiPinShared')}</a>` : ''}
            <a onclick="wikiDeletePage(${p.id})" style="color:var(--red)">${t('wikiDeletePage')}</a>` : ''}
          </div>
        </div>
      </div>
    </div>
    <div class="wiki-meta"><a href="/wiki/page/${p.id}/history" onclick="navigateTo('/wiki/page/${p.id}/history');return false;" id="mm-rev-label">v${p.rev_no}</a> &middot; ${t('wikiEditedBy')} ${p.updated_by ? `<a href="/user/${p.updated_by}" onclick="navigateTo('/user/${p.updated_by}');return false;">${esc(displayName(p.updated_by, p.updated_by_name))}</a>` : esc(p.updated_by_name || '?')} &middot; ${fmtDate(p.updated_at)}</div>
    <div id="wiki-summary-panel"></div>
    <div class="mm-wrap" id="mm-wrap">
      <div class="mm-toolbar" id="mm-toolbar"></div>
      <div class="mm-conflict hidden" id="mm-conflict"></div>
      <div class="mm-body">
        <div class="mm-stage" id="mm-stage"></div>
        <aside class="mm-panel hidden" id="mm-panel"><div class="mm-panel-head"><span id="mm-panel-title"></span><button class="btn-icon" id="mm-panel-close">✕</button></div><div class="mm-panel-body" id="mm-panel-body"></div></aside>
      </div>
      <div class="mm-status" id="mm-status"></div>
      <div class="mm-fab hidden" id="mm-fab"></div>
    </div>`;
  mmPage = new MindmapPageController(p, canEdit, el);
  if (typeof wikiEnsureLLM === 'function') wikiEnsureLLM();
  wikiRenderCommentsSection(el, p);
}

class MindmapPageController {
  constructor(page, canEdit, root) {
    this.page = page; this.canEdit = canEdit; this.root = root;
    this.rev = page.rev_no;
    this.dirty = false; this.saving = false; this.saveTimer = null; this.conflictRev = null;
    this.panelMode = null;
    this.stage = root.querySelector('#mm-stage');
    this.map = new MindMap(this.stage, {
      readOnly: !canEdit,
      onChange: () => this.onChange(),
      onSelect: () => this.onSelect(),
      onStatus: () => this.renderStatus(),
      onOpenPage: id => navigateTo('/wiki/page/' + id),
      onOpenTask: ref => this.openTask(ref),
      onFind: () => { const i = root.querySelector('#mm-search'); if (i) { i.focus(); i.select(); } },
      onClearSearch: () => { const i = root.querySelector('#mm-search'); if (i) { i.value = ''; } this.map.search(''); },
      onProps: field => this.showPanel('props', field),
      onHelp: () => this.help(),
      onSave: () => this.saveVersion(),
      onView: v => { try { localStorage.setItem('mm-view-' + page.id, JSON.stringify(v)); } catch (e) { /* ignore */ } },
      uploadImage: f => this.uploadImage(f),
      resolvePage: title => this.resolvePage(title),
    });
    this.renderToolbar();
    this.load(page.mindmap || { root: { text: page.title } });
    this.bindTitle();
    this.bindPanel();
    this.connectSSE();
    this.beforeUnload = e => { if (this.dirty) { this.flush(true); e.preventDefault(); e.returnValue = ''; } };
    window.addEventListener('beforeunload', this.beforeUnload);
    this.checkDraft();
  }
  destroy() {
    this.flush(true);
    window.removeEventListener('beforeunload', this.beforeUnload);
    if (this.es) { this.es.close(); this.es = null; }
    this.map.destroy();
    document.removeEventListener('fullscreenchange', this.onFs);
  }
  load(doc, keepView) {
    this.map.setDoc(doc, keepView);
    if (!keepView) {
      let v = null; try { v = JSON.parse(localStorage.getItem('mm-view-' + this.page.id) || 'null'); } catch (e) { v = null; }
      if (v && v.k) this.map.setView(v); else this.map.fit();
    }
    this.renderStatus();
  }

  // --- toolbar ----------------------------------------------------------------
  renderToolbar() {
    const tb = this.root.querySelector('#mm-toolbar');
    const ro = !this.canEdit;
    const btn = (act, label, title, extra) => `<button class="mm-tb ${extra || ''}" data-act="${act}" title="${escAttr(title)}" ${ro && /^(child|sibling|delete|undo|redo|relation)$/.test(act) ? 'disabled' : ''}>${label}</button>`;
    const sel = (id, title, options, cur) => `<label class="mm-tb-sel" title="${escAttr(title)}"><span>${esc(title)}</span><select id="${id}" ${ro ? 'disabled' : ''}>${options.map(([v, k]) => `<option value="${v}" ${v === cur ? 'selected' : ''}>${esc(mmT(k))}</option>`).join('')}</select></label>`;
    tb.innerHTML = `
      ${btn('child', '＋', mmT('mmNewChild') + ' (Tab)')}${btn('sibling', '⇥', mmT('mmNewSibling') + ' (Enter)')}${btn('delete', '🗑', mmT('mmDelete') + ' (Del)')}
      <span class="mm-tb-sep"></span>
      ${btn('undo', '↶', mmT('mmUndo') + ' (Ctrl+Z)')}${btn('redo', '↷', mmT('mmRedo') + ' (Ctrl+Y)')}
      <span class="mm-tb-sep"></span>
      ${sel('mm-layout', mmT('mmLayout'), [['balanced', 'mmLayoutBalanced'], ['right', 'mmLayoutRight'], ['left', 'mmLayoutLeft'], ['down', 'mmLayoutDown'], ['up', 'mmLayoutUp'], ['tree', 'mmLayoutTree']], 'balanced')}
      ${sel('mm-line', mmT('mmLine'), [['curve', 'mmLineCurve'], ['elbow', 'mmLineElbow'], ['straight', 'mmLineStraight']], 'curve')}
      ${sel('mm-palette', mmT('mmPalette'), [['rainbow', 'mmPaletteRainbow'], ['mono', 'mmPaletteMono'], ['warm', 'mmPaletteWarm'], ['cool', 'mmPaletteCool'], ['pastel', 'mmPalettePastel']], 'rainbow')}
      <span class="mm-tb-sep"></span>
      ${btn('relation', '⤳', mmT('mmAddRelation') + ' (Ctrl+Shift+L)')}
      <label class="mm-tb-sel" title="${escAttr(mmT('mmExpandLevel'))}"><select id="mm-expand"><option value="">${esc(mmT('mmExpandLevel'))}</option><option value="1">1</option><option value="2">2</option><option value="3">3</option><option value="4">4</option><option value="all">${esc(mmT('mmUnfoldAll'))}</option></select></label>
      <span class="mm-tb-sep"></span>
      <span class="mm-search-wrap"><input id="mm-search" class="mm-search" placeholder="${escAttr(mmT('mmSearch'))}" autocomplete="off"><span class="mm-search-count" id="mm-search-count"></span></span>
      <span class="mm-tb-grow"></span>
      ${btn('outline', '☰', mmT('mmOutline') + ' (Ctrl+Shift+O)', 'mm-tb-toggle')}${btn('props', '⚙', mmT('mmProperties') + ' (Ctrl+I)', 'mm-tb-toggle')}
      ${btn('help', '?', mmT('mmHelp') + ' (?)')}${btn('fullscreen', '⛶', mmT('mmFullscreen'))}`;
    tb.addEventListener('click', e => { const b = e.target.closest('[data-act]'); if (b) this.action(b.dataset.act); });
    tb.querySelector('#mm-layout').addEventListener('change', e => this.map.setSettings({ layout: e.target.value }));
    tb.querySelector('#mm-line').addEventListener('change', e => this.map.setSettings({ line: e.target.value }));
    tb.querySelector('#mm-palette').addEventListener('change', e => this.map.setSettings({ palette: e.target.value }));
    tb.querySelector('#mm-expand').addEventListener('change', e => { const v = e.target.value; e.target.value = ''; if (!v) return; this.map.foldAll(v === 'all' ? Infinity : Number(v)); });
    const s = tb.querySelector('#mm-search');
    let st = null;
    s.addEventListener('input', () => { clearTimeout(st); st = setTimeout(() => this.map.search(s.value), 150); });
    s.addEventListener('keydown', e => { if (e.key === 'Enter') { e.preventDefault(); this.map.nextMatch(e.shiftKey ? -1 : 1); } else if (e.key === 'Escape') { s.value = ''; this.map.search(''); this.stage.focus(); } });
    // Mobile floating actions (no Tab key on touch keyboards).
    const fab = this.root.querySelector('#mm-fab');
    if (!ro && window.matchMedia('(pointer: coarse)').matches) {
      fab.innerHTML = `<button data-act="child">＋ ${esc(mmT('mmNewChild'))}</button><button data-act="sibling">⇥ ${esc(mmT('mmNewSibling'))}</button><button data-act="edit">✎</button><button data-act="delete">🗑</button>`;
      fab.addEventListener('click', e => { const b = e.target.closest('[data-act]'); if (b) this.action(b.dataset.act); });
    }
  }
  syncToolbar() {
    const d = this.map.doc, q = s => this.root.querySelector(s);
    if (q('#mm-layout')) q('#mm-layout').value = d.layout;
    if (q('#mm-line')) q('#mm-line').value = d.line;
    if (q('#mm-palette')) q('#mm-palette').value = d.palette;
    const u = q('[data-act=undo]'), r = q('[data-act=redo]');
    if (u) u.disabled = !this.canEdit || !this.map.undoStack.length;
    if (r) r.disabled = !this.canEdit || !this.map.redoStack.length;
    const o = q('[data-act=outline]'), pr = q('[data-act=props]');
    if (o) o.classList.toggle('on', this.panelMode === 'outline');
    if (pr) pr.classList.toggle('on', this.panelMode === 'props');
    const fab = q('#mm-fab'); if (fab && fab.innerHTML) fab.classList.toggle('hidden', !this.map.anchor);
  }
  action(act) {
    const m = this.map;
    switch (act) {
      case 'child': m.addChild(m.anchor || m.doc.root.id); break;
      case 'sibling': m.addSibling(m.anchor); break;
      case 'edit': m.edit(m.anchor); break;
      case 'delete': m.deleteSelected(); break;
      case 'undo': m.undo(); break;
      case 'redo': m.redo(); break;
      case 'relation': m.startLink(); break;
      case 'outline': this.showPanel(this.panelMode === 'outline' ? null : 'outline'); break;
      case 'props': this.showPanel(this.panelMode === 'props' ? null : 'props'); break;
      case 'help': this.help(); break;
      case 'fullscreen': this.toggleFullscreen(); break;
    }
    if (act !== 'outline' && act !== 'props' && act !== 'help') this.stage.focus({ preventScroll: true });
  }
  toggleFullscreen() {
    const w = this.root.querySelector('#mm-wrap');
    if (document.fullscreenElement) { document.exitFullscreen(); return; }
    if (w.requestFullscreen) { w.requestFullscreen().catch(() => w.classList.toggle('mm-fs')); }
    else w.classList.toggle('mm-fs');
    this.onFs = () => { w.classList.toggle('mm-fs', !!document.fullscreenElement); setTimeout(() => this.map.fit(), 100); };
    document.addEventListener('fullscreenchange', this.onFs);
  }

  // --- side panel ---------------------------------------------------------------
  bindPanel() {
    this.root.querySelector('#mm-panel-close').addEventListener('click', () => this.showPanel(null));
    this.stage.addEventListener('keydown', e => { if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 'O' || e.key === 'o' || e.key === 'Щ' || e.key === 'щ')) { e.preventDefault(); this.showPanel(this.panelMode === 'outline' ? null : 'outline'); } });
  }
  showPanel(mode, field) {
    this.panelMode = mode;
    const panel = this.root.querySelector('#mm-panel');
    panel.classList.toggle('hidden', !mode);
    if (mode) {
      this.root.querySelector('#mm-panel-title').textContent = mode === 'props' ? mmT('mmProperties') : mmT('mmOutline');
      this.renderPanel(field);
    }
    this.syncToolbar();
    setTimeout(() => this.map.drawMinimap(), 50);
  }
  renderPanel(field) {
    const body = this.root.querySelector('#mm-panel-body');
    if (!this.panelMode) return;
    if (this.panelMode === 'props') this.map.renderProps(body, field); else this.map.renderOutline(body);
  }
  onSelect() { if (this.panelMode === 'props') this.renderPanel(); else if (this.panelMode === 'outline') { const b = this.root.querySelector('#mm-panel-body'); b.querySelectorAll('.mm-ol-item').forEach(i => i.classList.toggle('on', this.map.sel.has(i.dataset.id))); } this.syncToolbar(); }

  // --- status / saving ---------------------------------------------------------
  renderStatus() {
    const st = this.root.querySelector('#mm-status'); if (!st) return;
    const m = this.map;
    const save = !this.canEdit ? '' : this.saving ? `<span class="mm-st-saving">${esc(mmT('mmSaving'))}</span>` : this.saveError ? `<span class="mm-st-err">${esc(mmT('mmSaveError'))}: ${esc(this.saveError)}</span>` : this.dirty ? `<span class="mm-st-dirty">● ${esc(mmT('mmUnsaved'))}</span>` : `<span class="mm-st-ok">✓ ${esc(mmT('mmSaved'))}</span>`;
    const selN = m.sel.size > 1 ? ` · ${esc(mmT('mmSelectedN', { n: m.sel.size }))}` : '';
    const link = m.linkMode ? ` · <b>${esc(mmT('mmRelationHint'))}</b>` : '';
    const search = m.query ? ` · ${m.matches.length ? esc(mmT('mmMatchOf', { i: m.matchIdx + 1, n: m.matches.length })) : esc(mmT('mmNoMatches'))}` : '';
    st.innerHTML = `<span>${m.count()} ${esc(mmT('mmNodes'))}${selN}${link}${search}</span><span>${save} · ${esc(mmT('mmVersion', { n: this.rev }))}</span>`;
    const sc = this.root.querySelector('#mm-search-count'); if (sc) sc.textContent = m.query ? (m.matches.length ? `${m.matchIdx + 1}/${m.matches.length}` : '0') : '';
    const rl = this.root.querySelector('#mm-rev-label'); if (rl) rl.textContent = 'v' + this.rev;
    this.syncToolbar();
  }
  onChange() {
    if (!this.canEdit) return;
    this.dirty = true; this.saveError = null;
    this.saveDraft();
    this.renderStatus();
    if (this.panelMode === 'outline') this.renderPanel();
    clearTimeout(this.saveTimer);
    this.saveTimer = setTimeout(() => this.flush(false), 1200);
  }
  draftKey() { return 'mm-draft-' + this.page.id; }
  saveDraft() { try { localStorage.setItem(this.draftKey(), JSON.stringify({ rev: this.rev, title: this.title(), doc: this.map.doc, at: Date.now() })); } catch (e) { /* quota */ } }
  clearDraft() { try { localStorage.removeItem(this.draftKey()); } catch (e) { /* ignore */ } }
  async checkDraft() {
    let d = null; try { d = JSON.parse(localStorage.getItem(this.draftKey()) || 'null'); } catch (e) { d = null; }
    if (!d || !this.canEdit || d.rev !== this.rev || JSON.stringify(d.doc) === JSON.stringify(this.map.doc)) { if (d && d.rev !== this.rev) this.clearDraft(); return; }
    const ok = await uiChoice(mmT('mmDraftFound'), [{ label: mmT('mmRestore'), value: true, primary: true }, { label: mmT('mmDiscard'), value: false }]);
    if (ok) { this.map.setDoc(d.doc, true); if (d.title) this.setTitle(d.title); this.map.begin(); this.map.commit({ animate: false }); }
    else this.clearDraft();
  }
  title() { const h = this.root.querySelector('.mm-title'); return h ? h.textContent.trim() : this.page.title; }
  setTitle(v) { const h = this.root.querySelector('.mm-title'); if (h) h.textContent = v; }
  bindTitle() {
    const h = this.root.querySelector('.mm-title');
    if (!h || !this.canEdit) return;
    h.addEventListener('keydown', e => { if (e.key === 'Enter') { e.preventDefault(); h.blur(); } if (e.key === 'Escape') { h.textContent = this.page.title; h.blur(); } });
    h.addEventListener('blur', () => { const v = h.textContent.trim(); if (!v) { h.textContent = this.page.title; return; } if (v !== this.page.title) { this.page.title = v; wikiState.tree.forEach(n => { if (n.id === this.page.id) n.title = v; }); if (typeof wikiRenderTree === 'function') wikiRenderTree(); this.onChange(); } });
    h.addEventListener('paste', e => { e.preventDefault(); document.execCommand('insertText', false, (e.clipboardData.getData('text/plain') || '').replace(/\n/g, ' ')); });
  }
  // flush saves now (autosave=true → squashed revision). keepalive lets the
  // request finish while the page unloads.
  async flush(unloading, comment) {
    if (!this.canEdit || (!this.dirty && !comment) || this.conflictRev) return;
    clearTimeout(this.saveTimer);
    if (this.saving) { this.pendingFlush = true; return; }
    this.saving = true; this.renderStatus();
    const body = JSON.stringify(Object.assign({}, this.map.getDoc(), { title: this.title(), base_rev: this.rev, comment: comment || '', autosave: !comment }));
    try {
      const r = await fetch(`/api/wiki/pages/${this.page.id}/mindmap`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body, keepalive: !!unloading });
      if (r.status === 409) {
        const j = await r.json().catch(() => ({}));
        this.conflictRev = j.current_rev || (this.rev + 1);
        this.showConflict();
      } else if (!r.ok) {
        this.saveError = (await r.text()).slice(0, 200);
      } else {
        const j = await r.json();
        this.rev = j.rev_no; this.dirty = false; this.saveError = null; this.clearDraft();
      }
    } catch (e) { this.saveError = e.message; }
    this.saving = false;
    this.renderStatus();
    if (this.pendingFlush) { this.pendingFlush = false; if (this.dirty) this.flush(false); }
  }
  async saveVersion() {
    if (!this.canEdit) return;
    const c = await uiPrompt(mmT('mmVersionComment'), '');
    if (c === null) return;
    this.dirty = true;
    await this.flush(false, c.trim() || '·');
    if (!this.saveError && !this.conflictRev) toast(mmT('mmSaved'), 'success');
  }
  showConflict() {
    const box = this.root.querySelector('#mm-conflict');
    box.innerHTML = `<span>${esc(mmT('mmConflict', { n: this.conflictRev }))}</span><button class="btn btn-primary btn-sm" id="mm-c-reload">${esc(mmT('mmReload'))}</button><button class="btn btn-ghost btn-sm" id="mm-c-keep">${esc(mmT('mmKeepMine'))}</button>`;
    box.classList.remove('hidden');
    box.querySelector('#mm-c-reload').onclick = () => { this.conflictRev = null; box.classList.add('hidden'); this.clearDraft(); this.dirty = false; this.reload(); };
    box.querySelector('#mm-c-keep').onclick = async () => { this.rev = this.conflictRev; this.conflictRev = null; box.classList.add('hidden'); this.dirty = true; await this.flush(false); };
  }
  async reload(keepView) {
    try {
      const d = await api(`/api/wiki/pages/${this.page.id}/mindmap`);
      this.rev = d.rev_no;
      if (d.title && d.title !== this.title()) { this.page.title = d.title; this.setTitle(d.title); }
      this.load({ root: d.root, links: d.links, layout: d.layout, line: d.line, palette: d.palette }, keepView !== false);
    } catch (e) { toast(e.message, 'error'); }
  }
  connectSSE() {
    if (typeof EventSource === 'undefined') return;
    try {
      const es = new EventSource('/api/wiki/events');
      es.onmessage = ev => {
        let d; try { d = JSON.parse(ev.data); } catch (e) { return; }
        if (d.type !== 'wiki_page' || d.page_id !== this.page.id || !currentUser || d.user_id === currentUser.id || d.rev_no <= this.rev) return;
        if (this.dirty || this.saving) { this.conflictRev = d.rev_no; this.showConflict(); return; }
        this.reload(true).then(() => toast(mmT('mmUpdatedBy', { u: typeof displayName === 'function' ? displayName(d.user_id, '') || '#' + d.user_id : '#' + d.user_id }), 'info'));
      };
      es.onerror = () => { es.close(); this.es = null; };
      this.es = es;
    } catch (e) { this.es = null; }
  }

  // --- helpers --------------------------------------------------------------------
  async uploadImage(file) {
    if (typeof MAX_UPLOAD_SIZE !== 'undefined' && file.size > MAX_UPLOAD_SIZE) throw new Error(t('fileTooLarge'));
    const upload = typeof compressImage === 'function' && file.type !== 'image/gif' ? await compressImage(file) : file;
    const base64 = await new Promise((res, rej) => { const r = new FileReader(); r.onload = () => res(r.result.split(',')[1]); r.onerror = rej; r.readAsDataURL(upload); });
    const resp = await api('/api/files', 'POST', { data: base64, filename: file.name, mime: upload.type || file.type });
    return resp.url;
  }
  async resolvePage(title) {
    try { const hits = await api('/api/wiki/search?q=' + encodeURIComponent(title)); const h = hits.find(x => x.title.toLowerCase() === title.toLowerCase()) || null; return h ? h.page_id : 0; } catch (e) { return 0; }
  }
  openTask(ref) {
    const m = /^([A-Za-z][A-Za-z0-9_]*)-(\d+)$/.exec(ref);
    if (!m) return;
    api('/api/search/global?q=' + encodeURIComponent(ref) + '&scope=kanban').then(hits => { const h = (hits || []).find(x => x.url && /^\/task\//.test(x.url) && x.title.toUpperCase().includes(ref.toUpperCase())); if (h) navigateTo(h.url); else toast(ref, 'info'); }).catch(() => {});
  }
  exportAs(format) {
    const name = (this.page.title || 'mindmap').replace(/[\\/:*?"<>|]/g, '_');
    if (format === 'svg') return this.map.downloadSVG(name);
    if (format === 'png') return this.map.downloadPNG(name);
    window.open(`/api/wiki/pages/${this.page.id}/mindmap/export?format=${format}`, '_blank');
  }
  help() {
    const rows = [['Tab', 'mmKeyChild'], ['Enter / Shift+Enter', 'mmKeySibling'], ['Ctrl+Enter', 'mmKeyParent'], ['F2 / Space / 2×click', 'mmKeyEdit'], ['Shift+Enter', 'mmKeyNewline'], ['Delete / Backspace', 'mmKeyDelete'], ['← ↑ → ↓', 'mmKeyNav'], ['/', 'mmKeyFold'], ['Alt+↑ / Alt+↓', 'mmKeyMove'], ['Ctrl+Z / Ctrl+Y', 'mmKeyUndo'], ['Ctrl+C / X / V', 'mmKeyCopy'], ['Ctrl+D', 'mmKeyDup'], ['Ctrl+A', 'mmKeyAll'], ['Ctrl+wheel, Ctrl+± / Ctrl+0', 'mmKeyZoom'], ['Shift+1 / Shift+2 / Shift+0', 'mmKeyFit'], ['Ctrl+F', 'mmKeyFind'], ['Shift+click / Shift+drag', 'mmKeyMulti'], ['Drag / Space+drag', 'mmKeyPan'], ['Alt+1…6 / Alt+0', 'mmKeyPriority'], ['Alt+Enter', 'mmKeyTask'], ['Ctrl+I / Ctrl+K', 'mmKeyProps'], ['Ctrl+Shift+L', 'mmAddRelation'], ['Ctrl+S', 'mmKeySave'], ['?', 'mmKeyHelp']];
    const ov = document.createElement('div');
    ov.className = 'modal-overlay'; ov.style.zIndex = '600';
    ov.innerHTML = `<div class="modal"><h3>${esc(mmT('mmShortcuts'))}</h3><table class="mm-help">${rows.map(([k, d]) => `<tr><td><kbd>${esc(k)}</kbd></td><td>${esc(mmT(d))}</td></tr>`).join('')}</table></div>`;
    ov.addEventListener('mousedown', e => { if (e.target === ov) ov.remove(); });
    document.addEventListener('keydown', function esc_(e) { if (e.key === 'Escape') { ov.remove(); document.removeEventListener('keydown', esc_, true); } }, true);
    document.body.appendChild(ov);
  }
  importDialog() { mmImportDialog({ pageId: this.page.id, onDone: () => { this.clearDraft(); this.dirty = false; this.reload(false); } }); }
}

// mmImportDialog: file / pasted outline → either into an existing page
// ({pageId}) or as a new page ({spaceId, parentId}). Calls onDone(result).
function mmImportDialog(opts) {
  ensureMindmapI18n();
  const ov = document.createElement('div');
  ov.className = 'modal-overlay'; ov.style.zIndex = '600';
  ov.innerHTML = `<div class="modal mm-import">
    <h3>${esc(mmT('mmImportTitle'))}</h3>
    <p class="mm-import-hint">${esc(mmT('mmImportHint'))}</p>
    <div class="mm-import-row"><button class="btn btn-ghost btn-sm" id="mmi-file">${esc(mmT('mmImportFile'))}</button><span id="mmi-fname" class="mm-import-fname"></span></div>
    <p class="mm-import-hint">${esc(mmT('mmImportPaste'))}</p>
    <textarea id="mmi-text" rows="8" placeholder="# Root&#10;- child&#10;  - grandchild"></textarea>
    ${opts.pageId ? `<label class="mm-f">${esc(mmT('mmImportMode'))}<select id="mmi-mode"><option value="replace">${esc(mmT('mmImportReplace'))}</option><option value="append">${esc(mmT('mmImportAppend'))}</option></select></label>` : ''}
    <div class="ui-dialog-foot"><button class="btn btn-ghost btn-sm" id="mmi-cancel">${esc(t('cancel'))}</button><button class="btn btn-primary btn-sm" id="mmi-ok">${esc(mmT('mmImport'))}</button></div>
  </div>`;
  document.body.appendChild(ov);
  let file = null;
  const q = s => ov.querySelector(s);
  q('#mmi-file').onclick = () => { const i = document.createElement('input'); i.type = 'file'; i.accept = '.md,.markdown,.txt,.opml,.xml,.mm,.xmind,.json,.mmd,.mermaid'; i.onchange = () => { file = i.files[0] || null; q('#mmi-fname').textContent = file ? file.name : ''; }; i.click(); };
  q('#mmi-cancel').onclick = () => ov.remove();
  ov.addEventListener('mousedown', e => { if (e.target === ov) ov.remove(); });
  q('#mmi-ok').onclick = async () => {
    const body = { mode: opts.pageId ? q('#mmi-mode').value : undefined, space_id: opts.spaceId, parent_id: opts.parentId || null, title: opts.title || '' };
    if (file) {
      body.filename = file.name;
      if (/\.xmind$/i.test(file.name)) body.data_base64 = await new Promise((res, rej) => { const r = new FileReader(); r.onload = () => res(r.result.split(',')[1]); r.onerror = rej; r.readAsDataURL(file); });
      else body.data = await file.text();
    } else {
      body.data = q('#mmi-text').value;
      if (!body.data.trim()) return;
    }
    q('#mmi-ok').disabled = true;
    try {
      const url = opts.pageId ? `/api/wiki/pages/${opts.pageId}/mindmap/import` : '/api/wiki/mindmaps/import';
      const r = await api(url, 'POST', body);
      ov.remove();
      toast(mmT('mmImportDone', { n: r.nodes }), 'success');
      if (opts.onDone) opts.onDone(r);
    } catch (e) { q('#mmi-ok').disabled = false; toast(e.message, 'error'); }
  };
}

// mmRenderEmbed renders a read-only map (for [[mindmap:ID]] tokens in regular
// pages and for revision previews) into `el`.
async function mmRenderEmbed(el, pageId, doc) {
  ensureMindmapI18n();
  try {
    if (!doc) { const d = await api(`/api/wiki/pages/${pageId}/mindmap`); doc = { root: d.root, links: d.links, layout: d.layout, line: d.line, palette: d.palette }; el.dataset.title = d.title; }
  } catch (e) { el.innerHTML = `<div class="wiki-empty">${esc(e.message)}</div>`; return; }
  el.innerHTML = `<div class="mm-embed-bar"><span>🧠 ${esc(el.dataset.title || '')}</span>${pageId ? `<a href="/wiki/page/${pageId}" onclick="navigateTo('/wiki/page/${pageId}');return false;">${esc(mmT('mmEmbedOpen'))} →</a>` : ''}</div><div class="mm-embed-stage"></div>`;
  const map = new MindMap(el.querySelector('.mm-embed-stage'), { readOnly: true, embedded: true, onOpenPage: id => navigateTo('/wiki/page/' + id) });
  map.setDoc(doc);
  el._mm = map;
  return map;
}

// mmNewPageKind asks which kind of page to create; returns 'page' | 'mindmap' | 'import' | undefined.
function mmNewPageKind() {
  ensureMindmapI18n();
  return uiChoice(mmT('mmPageType'), [
    { label: mmT('mmPageTypePage'), value: 'page', primary: true },
    { label: mmT('mmPageTypeMindmap'), value: 'mindmap' },
    { label: mmT('mmPageTypeImport'), value: 'import' },
    { label: t('cancel'), value: undefined },
  ]);
}
