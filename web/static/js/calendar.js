// Calendar service frontend. Relies on shared helpers from the inline script
// (api, t, esc, escAttr, navigateTo, getTimezone, langLocale, currentUser, state).
// Times travel in UTC ("YYYY-MM-DD HH:MM:SS"); inputs use the browser timezone.

let calState = {
  calendars: [],
  events: [],
  view: 'month', // month | week | agenda
  cursor: null, // a Date within the visible period (browser-local)
  hidden: {}, // calendarId -> true when unchecked (subscriptions use -subId)
  hideKanban: false, // hide task/sprint overlay
  subscriptions: [], // external ICS subscriptions
  es: null, // EventSource for live updates
  dragId: null, // event id being dragged
};

const CAL_VIEW_KEY = 'kanban-cal-view';
let calReminders = []; // draft reminder rows {value,unit,channel} for the open editor
const CAL_REM_UNIT = { min: 1, hour: 60, day: 1440 };

function calRemToDraft(r) {
  let value = r.lead_min, unit = 'min';
  if (r.lead_min > 0 && r.lead_min % 1440 === 0) { value = r.lead_min / 1440; unit = 'day'; }
  else if (r.lead_min > 0 && r.lead_min % 60 === 0) { value = r.lead_min / 60; unit = 'hour'; }
  return { value, unit, channel: r.channel || 'both' };
}
function calAddReminder() { calReminders.push({ value: 15, unit: 'min', channel: 'both' }); calRenderReminders(); }
function calRenderReminders() {
  const el = document.getElementById('cal-reminders');
  if (!el) return;
  const unitOpts = [['min', 'calRemMin'], ['hour', 'calRemHour'], ['day', 'calRemDay']];
  const chOpts = [['both', 'calRemBoth'], ['telegram', 'calRemTg'], ['inapp', 'calRemInApp']];
  el.innerHTML = calReminders.map((r, i) => `<div class="cal-rem-row">
    <input type="number" min="0" value="${r.value}" oninput="calReminders[${i}].value=Number(this.value)">
    <select onchange="calReminders[${i}].unit=this.value">${unitOpts.map(([u, k]) => `<option value="${u}" ${r.unit === u ? 'selected' : ''}>${t(k)}</option>`).join('')}</select>
    <span class="cal-rem-before">${t('calRemBefore')}</span>
    <select onchange="calReminders[${i}].channel=this.value">${chOpts.map(([c, k]) => `<option value="${c}" ${r.channel === c ? 'selected' : ''}>${t(k)}</option>`).join('')}</select>
    <button class="btn-icon btn-sm cal-danger" onclick="calReminders.splice(${i},1);calRenderReminders()">&#10005;</button>
  </div>`).join('');
}

// === i18n (ru/en; other languages fall back to en until the Phase D sweep) ===
const CAL_I18N = {
  en: {
    calNewEvent: 'New event', calNewCalendar: 'New calendar', calToday: 'Today',
    calMonth: 'Month', calWeek: 'Week', calDay: 'Day', calAgenda: 'Agenda', calCalendars: 'Calendars',
    calTitle: 'Title', calCalendar: 'Calendar', calStart: 'Start', calEnd: 'End',
    calAllDay: 'All day', calLocation: 'Location', calDescription: 'Description',
    calRepeat: 'Repeat', calRepeatNone: 'Does not repeat', calRepeatDaily: 'Daily',
    calRepeatWeekly: 'Weekly', calRepeatMonthly: 'Monthly', calRepeatYearly: 'Yearly',
    calRepeatCount: 'Occurrences (optional)', calSave: 'Save', calCancel: 'Cancel',
    calDelete: 'Delete', calDeleteConfirm: 'Delete this event?', calNoCalendars: 'No calendars yet',
    calNoEvents: 'No events in this period', calTitleRequired: 'Title is required',
    calNameRequired: 'Name is required', calVisibility: 'Visibility', calPrivate: 'Private',
    calShared: 'Shared', calColor: 'Color', calDeleteCalendar: 'Delete calendar',
    calDeleteCalendarConfirm: 'Delete this calendar and all its events?', calName: 'Name',
    calEditEvent: 'Edit event', calEditCalendar: 'Calendar settings', calKanban: 'Tasks & sprints', calReminders: 'Reminders', calReminderAdd: 'Add reminder', calRemBefore: 'before', calRemMin: 'minutes', calRemHour: 'hours', calRemDay: 'days', calRemTg: 'Telegram', calRemInApp: 'In-app', calRemBoth: 'Both',
    calAddIcsSub: 'Google Calendar (ICS)', calIcsUrl: 'iCal/ICS address (secret URL)', calIcsName: 'Name', calSyncNow: 'Sync now', calSubDelete: 'Remove subscription', calNoTitle: '(no title)',
    calIcsHint: 'Google Calendar → Settings → your calendar → "Secret address in iCal format". Read-only.',
  },
  ru: {
    calNewEvent: 'Новое событие', calNewCalendar: 'Новый календарь', calToday: 'Сегодня',
    calMonth: 'Месяц', calWeek: 'Неделя', calDay: 'День', calAgenda: 'Повестка', calCalendars: 'Календари',
    calTitle: 'Название', calCalendar: 'Календарь', calStart: 'Начало', calEnd: 'Конец',
    calAllDay: 'Весь день', calLocation: 'Место', calDescription: 'Описание',
    calRepeat: 'Повтор', calRepeatNone: 'Без повтора', calRepeatDaily: 'Ежедневно',
    calRepeatWeekly: 'Еженедельно', calRepeatMonthly: 'Ежемесячно', calRepeatYearly: 'Ежегодно',
    calRepeatCount: 'Число повторений (необяз.)', calSave: 'Сохранить', calCancel: 'Отмена',
    calDelete: 'Удалить', calDeleteConfirm: 'Удалить событие?', calNoCalendars: 'Пока нет календарей',
    calNoEvents: 'Нет событий в этом периоде', calTitleRequired: 'Введите название',
    calNameRequired: 'Введите название', calVisibility: 'Видимость', calPrivate: 'Личный',
    calShared: 'Общий', calColor: 'Цвет', calDeleteCalendar: 'Удалить календарь',
    calDeleteCalendarConfirm: 'Удалить календарь и все его события?', calName: 'Название',
    calEditEvent: 'Редактировать событие', calEditCalendar: 'Настройки календаря', calKanban: 'Задачи и спринты', calReminders: 'Напоминания', calReminderAdd: 'Добавить напоминание', calRemBefore: 'до начала', calRemMin: 'минут', calRemHour: 'часов', calRemDay: 'дней', calRemTg: 'Telegram', calRemInApp: 'В приложении', calRemBoth: 'Оба',
    calAddIcsSub: 'Google Календарь (ICS)', calIcsUrl: 'Адрес iCal/ICS (секретный URL)', calIcsName: 'Название', calSyncNow: 'Синхронизировать', calSubDelete: 'Удалить подписку', calNoTitle: '(без названия)',
    calIcsHint: 'Google Календарь → Настройки → ваш календарь → «Секретный адрес в формате iCal». Только чтение.',
  },
  es: {
    calNewEvent: "Nuevo evento", calNewCalendar: "Nuevo calendario", calToday: "Hoy",
    calMonth: "Mes", calWeek: "Semana", calDay: "Día", calAgenda: "Agenda", calCalendars: "Calendarios",
    calTitle: "Título", calCalendar: "Calendario", calStart: "Inicio", calEnd: "Fin",
    calAllDay: "Todo el día", calLocation: "Ubicación", calDescription: "Descripción",
    calRepeat: "Repetir", calRepeatNone: "No se repite", calRepeatDaily: "Diariamente",
    calRepeatWeekly: "Semanalmente", calRepeatMonthly: "Mensualmente", calRepeatYearly: "Anualmente",
    calRepeatCount: "Repeticiones (opcional)", calSave: "Guardar", calCancel: "Cancelar",
    calDelete: "Eliminar", calDeleteConfirm: "¿Eliminar este evento?", calNoCalendars: "Aún no hay calendarios",
    calNoEvents: "Sin eventos en este período", calTitleRequired: "El título es obligatorio",
    calNameRequired: "El nombre es obligatorio", calVisibility: "Visibilidad", calPrivate: "Privado",
    calShared: "Compartido", calColor: "Color", calDeleteCalendar: "Eliminar calendario",
    calDeleteCalendarConfirm: "¿Eliminar este calendario y todos sus eventos?", calName: "Nombre",
    calEditEvent: "Editar evento", calEditCalendar: "Ajustes del calendario", calKanban: "Tareas y sprints",
  },
  fr: {
    calNewEvent: "Nouvel événement", calNewCalendar: "Nouveau calendrier", calToday: "Aujourd'hui",
    calMonth: "Mois", calWeek: "Semaine", calDay: "Jour", calAgenda: "Agenda", calCalendars: "Calendriers",
    calTitle: "Titre", calCalendar: "Calendrier", calStart: "Début", calEnd: "Fin",
    calAllDay: "Toute la journée", calLocation: "Lieu", calDescription: "Description",
    calRepeat: "Répéter", calRepeatNone: "Ne se répète pas", calRepeatDaily: "Tous les jours",
    calRepeatWeekly: "Toutes les semaines", calRepeatMonthly: "Tous les mois", calRepeatYearly: "Tous les ans",
    calRepeatCount: "Occurrences (facultatif)", calSave: "Enregistrer", calCancel: "Annuler",
    calDelete: "Supprimer", calDeleteConfirm: "Supprimer cet événement ?", calNoCalendars: "Aucun calendrier",
    calNoEvents: "Aucun événement sur cette période", calTitleRequired: "Le titre est obligatoire",
    calNameRequired: "Le nom est obligatoire", calVisibility: "Visibilité", calPrivate: "Privé",
    calShared: "Partagé", calColor: "Couleur", calDeleteCalendar: "Supprimer le calendrier",
    calDeleteCalendarConfirm: "Supprimer ce calendrier et tous ses événements ?", calName: "Nom",
    calEditEvent: "Modifier l'événement", calEditCalendar: "Paramètres du calendrier", calKanban: "Tâches et sprints",
  },
  de: {
    calNewEvent: "Neuer Termin", calNewCalendar: "Neuer Kalender", calToday: "Heute",
    calMonth: "Monat", calWeek: "Woche", calDay: "Tag", calAgenda: "Agenda", calCalendars: "Kalender",
    calTitle: "Titel", calCalendar: "Kalender", calStart: "Beginn", calEnd: "Ende",
    calAllDay: "Ganztägig", calLocation: "Ort", calDescription: "Beschreibung",
    calRepeat: "Wiederholen", calRepeatNone: "Keine Wiederholung", calRepeatDaily: "Täglich",
    calRepeatWeekly: "Wöchentlich", calRepeatMonthly: "Monatlich", calRepeatYearly: "Jährlich",
    calRepeatCount: "Wiederholungen (optional)", calSave: "Speichern", calCancel: "Abbrechen",
    calDelete: "Löschen", calDeleteConfirm: "Diesen Termin löschen?", calNoCalendars: "Noch keine Kalender",
    calNoEvents: "Keine Termine in diesem Zeitraum", calTitleRequired: "Titel ist erforderlich",
    calNameRequired: "Name ist erforderlich", calVisibility: "Sichtbarkeit", calPrivate: "Privat",
    calShared: "Geteilt", calColor: "Farbe", calDeleteCalendar: "Kalender löschen",
    calDeleteCalendarConfirm: "Diesen Kalender und alle Termine löschen?", calName: "Name",
    calEditEvent: "Termin bearbeiten", calEditCalendar: "Kalendereinstellungen", calKanban: "Aufgaben und Sprints",
  },
  pt: {
    calNewEvent: "Novo evento", calNewCalendar: "Novo calendário", calToday: "Hoje",
    calMonth: "Mês", calWeek: "Semana", calDay: "Dia", calAgenda: "Agenda", calCalendars: "Calendários",
    calTitle: "Título", calCalendar: "Calendário", calStart: "Início", calEnd: "Fim",
    calAllDay: "Dia inteiro", calLocation: "Local", calDescription: "Descrição",
    calRepeat: "Repetir", calRepeatNone: "Não se repete", calRepeatDaily: "Diariamente",
    calRepeatWeekly: "Semanalmente", calRepeatMonthly: "Mensalmente", calRepeatYearly: "Anualmente",
    calRepeatCount: "Ocorrências (opcional)", calSave: "Salvar", calCancel: "Cancelar",
    calDelete: "Excluir", calDeleteConfirm: "Excluir este evento?", calNoCalendars: "Ainda não há calendários",
    calNoEvents: "Sem eventos neste período", calTitleRequired: "O título é obrigatório",
    calNameRequired: "O nome é obrigatório", calVisibility: "Visibilidade", calPrivate: "Privado",
    calShared: "Compartilhado", calColor: "Cor", calDeleteCalendar: "Excluir calendário",
    calDeleteCalendarConfirm: "Excluir este calendário e todos os eventos?", calName: "Nome",
    calEditEvent: "Editar evento", calEditCalendar: "Configurações do calendário", calKanban: "Tarefas e sprints",
  },
};
let calI18nMerged = false;
function ensureCalI18n() {
  if (calI18nMerged || typeof LANGS === 'undefined') return;
  for (const lang of ['ru', 'en', 'zh', 'es', 'fr', 'de', 'pt', 'ja', 'ko', 'ar']) {
    if (!LANGS[lang]) continue;
    const dict = Object.assign({}, CAL_I18N.en, CAL_I18N[lang] || {});
    for (const k in dict) if (!(k in LANGS[lang])) LANGS[lang][k] = dict[k];
  }
  calI18nMerged = true;
}

// === Time helpers ===
function calParseUTC(s) { return new Date(s && s.includes('T') ? s : (s || '').replace(' ', 'T') + 'Z'); }
function calPad(n) { return String(n).padStart(2, '0'); }
function calDayKeyLocal(d) { return d.getFullYear() + '-' + calPad(d.getMonth() + 1) + '-' + calPad(d.getDate()); }
function calEventDayKey(e) { return calDayKeyLocal(calParseUTC(e.start)); }
// datetime-local input value (browser-local) -> UTC storage string
function calInputToUTC(v) {
  if (!v) return '';
  const d = new Date(v);
  if (isNaN(d.getTime())) return '';
  return d.getUTCFullYear() + '-' + calPad(d.getUTCMonth() + 1) + '-' + calPad(d.getUTCDate()) + ' ' +
    calPad(d.getUTCHours()) + ':' + calPad(d.getUTCMinutes()) + ':00';
}
function calUTCToInput(s) {
  const d = calParseUTC(s);
  return d.getFullYear() + '-' + calPad(d.getMonth() + 1) + '-' + calPad(d.getDate()) + 'T' +
    calPad(d.getHours()) + ':' + calPad(d.getMinutes());
}
function calLocale() { return (typeof langLocale === 'function') ? langLocale() : undefined; }
function calFmtTime(s) {
  try { return new Intl.DateTimeFormat(calLocale(), { timeZone: getTimezone(), hour: '2-digit', minute: '2-digit', hour12: false }).format(calParseUTC(s)); }
  catch (e) { return ''; }
}
function calFmtDay(s) {
  try { return new Intl.DateTimeFormat(calLocale(), { timeZone: getTimezone(), year: 'numeric', month: 'short', day: 'numeric' }).format(calParseUTC(s)); }
  catch (e) { return ''; }
}
function calFmtDateTime(s) {
  try { return new Intl.DateTimeFormat(calLocale(), { timeZone: getTimezone(), month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }).format(calParseUTC(s)); }
  catch (e) { return ''; }
}

// === Entry point ===
async function renderCalendarView() {
  ensureCalI18n();
  calState.view = localStorage.getItem(CAL_VIEW_KEY) || 'month';
  if (!calState.cursor) calState.cursor = new Date();
  try {
    calState.calendars = await api('/api/calendar/calendars');
  } catch (e) {
    document.getElementById('app').innerHTML = `<div class="cal-empty cal-empty-lg">${esc(e.message)}</div>`;
    return;
  }
  try { calState.subscriptions = await api('/api/calendar/subscriptions'); }
  catch (e) { calState.subscriptions = []; }
  calRenderLayout();
  calConnectSSE();
  await calLoadAndRender();
}

// Live updates: reload the visible range when any calendar the user can see changes.
function calConnectSSE() {
  if (calState.es || typeof EventSource === 'undefined') return;
  try {
    const es = new EventSource('/api/calendar/stream');
    calState.es = es;
    let tmo = null;
    es.onmessage = () => { clearTimeout(tmo); tmo = setTimeout(() => { calLoadAndRender(); calUpdateBadge(); }, 250); };
    es.onerror = () => { /* browser auto-reconnects */ };
  } catch (e) { /* SSE unavailable */ }
}

// calInitGlobal runs once at startup so the rail badge shows upcoming events
// even before the calendar view is opened.
async function calInitGlobal() {
  if (typeof isServiceEnabled === 'function' && !isServiceEnabled('calendar')) return;
  calUpdateBadge();
}
async function calUpdateBadge() {
  try {
    const r = await api('/api/calendar/upcoming');
    if (typeof setRailBadge === 'function') setRailBadge('calendar', r.upcoming || 0);
  } catch (e) { /* ignore */ }
}

function calRenderLayout() {
  const canWrite = currentUser && currentUser.role !== 'readonly';
  calRenderHeader();
  document.getElementById('app').innerHTML = `
    <main class="cal-main cal-main-full">
      <div class="cal-toolbar">
        <div class="cal-nav">
          <button class="btn btn-ghost btn-sm" onclick="calMove(-1)">&#8249;</button>
          <button class="btn btn-ghost btn-sm" onclick="calToday()">${t('calToday')}</button>
          <button class="btn btn-ghost btn-sm" onclick="calMove(1)">&#8250;</button>
          <span class="cal-period" id="cal-period"></span>
        </div>
        ${canWrite ? `<button class="btn btn-primary btn-sm" onclick="calNewEventDialog()">+ ${t('calNewEvent')}</button>` : ''}
      </div>
      <div class="cal-body" id="cal-body"></div>
    </main>`;
}

// calRenderHeader injects the view switcher + calendars visibility selector into
// the app header (#cal-hdr) as kanban-style dropdowns, shown only in calendar view.
function calRenderHeader() {
  const el = document.getElementById('cal-hdr');
  if (!el) return;
  const canWrite = currentUser && currentUser.role !== 'readonly';
  const views = ['month', 'week', 'day', 'agenda'];
  const vlabel = v => t('cal' + v[0].toUpperCase() + v.slice(1));
  const viewItems = views.map(v => `<div class="sfd-item ${calState.view === v ? 'active' : ''}" onclick="calSetView('${v}')">${esc(vlabel(v))}</div>`).join('');
  let calItems = calState.calendars.map(c => `<label class="sfd-item cal-dd-row">
      <input type="checkbox" ${calState.hidden[c.id] ? '' : 'checked'} onchange="calToggleCal(${c.id}, this.checked)">
      <span class="cal-dot" style="background:${escAttr(c.color)}"></span>
      <span class="cal-cl-name">${esc(c.name)}</span>
      ${c.can_edit ? `<button class="cal-cl-edit" title="${escAttr(t('calEditCalendar'))}" onclick="event.preventDefault();event.stopPropagation();calEditCalendarDialog(${c.id})">&#9881;</button>` : ''}
    </label>`).join('');
  calItems += `<label class="sfd-item cal-dd-row">
      <input type="checkbox" ${calState.hideKanban ? '' : 'checked'} onchange="calToggleKanban(this.checked)">
      <span class="cal-dot" style="background:#ef4444"></span>
      <span class="cal-cl-name">${esc(t('calKanban'))}</span>
    </label>`;
  // External ICS subscriptions (read-only), toggled by -subId.
  for (const sub of (calState.subscriptions || [])) {
    const key = -sub.id;
    calItems += `<label class="sfd-item cal-dd-row">
      <input type="checkbox" ${calState.hidden[key] ? '' : 'checked'} onchange="calToggleCal(${key}, this.checked)">
      <span class="cal-dot" style="background:${escAttr(sub.color)}"></span>
      <span class="cal-cl-name">${esc(sub.name)}</span>
      ${sub.last_error ? `<span class="cal-sub-err" title="${escAttr(sub.last_error)}">&#9888;</span>` : ''}
      ${canWrite ? `<button class="cal-cl-edit" title="${escAttr(t('calSubDelete'))}" onclick="event.preventDefault();event.stopPropagation();calDeleteSubscription(${sub.id})">&#10005;</button>` : ''}
    </label>`;
  }
  if (canWrite) calItems += `<div class="sfd-item cal-dd-new" onclick="calNewCalendarDialog()">+ ${esc(t('calNewCalendar'))}</div>`;
  if (canWrite) calItems += `<div class="sfd-item cal-dd-new" onclick="calSubscriptionDialog()">+ ${esc(t('calAddIcsSub'))}</div>`;
  el.innerHTML = `<span class="hdr-sep">/</span>
    <span class="hdr-filter">
      <span class="hdr-filter-name" id="cal-view-label" onclick="hdrDropToggle(event,'cal-view-dd')">${esc(vlabel(calState.view))}</span>
      <div class="hdr-filter-dropdown hidden" id="cal-view-dd">${viewItems}</div>
    </span>
    <span class="hdr-sep">/</span>
    <span class="hdr-filter">
      <span class="hdr-filter-name" id="cal-cals-label" onclick="hdrDropToggle(event,'cal-cals-dd')">${esc(t('calCalendars'))}</span>
      <div class="hdr-filter-dropdown hidden" id="cal-cals-dd" onclick="event.stopPropagation()">${calItems}</div>
    </span>`;
}

// Back-compat alias: callers that refreshed the calendar list now refresh the header.
function calRenderCalList() { calRenderHeader(); }

function calToggleKanban(checked) { calState.hideKanban = !checked; calRenderBody(); }

function calToggleCal(id, checked) {
  if (checked) delete calState.hidden[id]; else calState.hidden[id] = true;
  calRenderBody();
}

function calVisibleEvents() {
  return calState.events.filter(e => {
    if (e.source === 'task' || e.source === 'sprint') return !calState.hideKanban;
    return !calState.hidden[e.calendar_id]; // ICS events use calendar_id = -subId
  });
}

// === Range + load ===
function calPeriodRange() {
  const c = calState.cursor;
  let start, end;
  if (calState.view === 'day') {
    start = new Date(c); start.setHours(0, 0, 0, 0);
    end = new Date(start);
  } else if (calState.view === 'week') {
    start = new Date(c); start.setDate(c.getDate() - ((c.getDay() + 6) % 7)); start.setHours(0, 0, 0, 0);
    end = new Date(start); end.setDate(start.getDate() + 6);
  } else { // month & agenda use the month grid
    const first = new Date(c.getFullYear(), c.getMonth(), 1);
    start = new Date(first); start.setDate(1 - ((first.getDay() + 6) % 7));
    end = new Date(start); end.setDate(start.getDate() + 41);
  }
  return { start, end };
}

async function calLoadAndRender() {
  const { start, end } = calPeriodRange();
  // ±1 day buffer so timezone offsets never drop boundary events
  const from = new Date(start); from.setDate(from.getDate() - 1);
  const to = new Date(end); to.setDate(to.getDate() + 1);
  const fromS = calDayKeyLocal(from) + ' 00:00:00';
  const toS = calDayKeyLocal(to) + ' 23:59:59';
  try {
    calState.events = await api(`/api/calendar/events?from=${encodeURIComponent(fromS)}&to=${encodeURIComponent(toS)}`);
  } catch (e) { calState.events = []; }
  calRenderPeriodLabel();
  calRenderBody();
}

function calRenderPeriodLabel() {
  const el = document.getElementById('cal-period');
  if (!el) return;
  const c = calState.cursor;
  if (calState.view === 'day') {
    el.textContent = new Intl.DateTimeFormat(calLocale(), { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' }).format(c);
  } else if (calState.view === 'week') {
    const { start, end } = calPeriodRange();
    const f = new Intl.DateTimeFormat(calLocale(), { day: 'numeric', month: 'short' });
    el.textContent = f.format(start) + ' – ' + f.format(end) + ' ' + start.getFullYear();
  } else {
    el.textContent = new Intl.DateTimeFormat(calLocale(), { month: 'long', year: 'numeric' }).format(c);
  }
}

function calSetView(v) { calState.view = v; localStorage.setItem(CAL_VIEW_KEY, v); calRenderLayout(); calLoadAndRender(); }
function calToday() { calState.cursor = new Date(); calLoadAndRender(); }
function calMove(dir) {
  const c = calState.cursor;
  if (calState.view === 'day') c.setDate(c.getDate() + dir);
  else if (calState.view === 'week') c.setDate(c.getDate() + 7 * dir);
  else c.setMonth(c.getMonth() + dir);
  calState.cursor = new Date(c);
  calLoadAndRender();
}

// === Body rendering ===
function calRenderBody() {
  const el = document.getElementById('cal-body');
  if (!el) return;
  if (calState.view === 'agenda') { calRenderAgenda(el); return; }
  if (calState.view === 'day') { calRenderDay(el); return; }
  calRenderGrid(el);
}

function calAgendaRow(e) {
  return `<div class="cal-agenda-row" onclick="calEventClick(${e.id}, '${esc(e.source || '')}', ${e.ref_id || 0})">
    <span class="cal-dot" style="background:${escAttr(e.color || '#6366f1')}"></span>
    <span class="cal-agenda-time">${e.all_day ? t('calAllDay') : esc(calFmtTime(e.start))}</span>
    <span class="cal-agenda-title">${esc(e.title)}</span>
    ${e.location ? `<span class="cal-agenda-loc">${esc(e.location)}</span>` : ''}
  </div>`;
}

function calRenderDay(el) {
  const key = calDayKeyLocal(calState.cursor);
  const evs = calVisibleEvents().filter(e => calEventDayKey(e) === key).sort((a, b) => a.start.localeCompare(b.start));
  if (!evs.length) { el.innerHTML = `<div class="cal-empty cal-empty-lg">${t('calNoEvents')}</div>`; return; }
  el.innerHTML = `<div class="cal-agenda"><div class="cal-agenda-day">${evs.map(calAgendaRow).join('')}</div></div>`;
}

// === Drag to reschedule (whole-day moves; single, editable events only) ===
function calDragStart(ev, id) { calState.dragId = id; }
function calDayDelta(fromKey, toKey) {
  return Math.round((new Date(toKey + 'T00:00') - new Date(fromKey + 'T00:00')) / 86400000);
}
function calShiftUTCDays(s, days) {
  const d = calParseUTC(s); d.setUTCDate(d.getUTCDate() + days);
  return d.getUTCFullYear() + '-' + calPad(d.getUTCMonth() + 1) + '-' + calPad(d.getUTCDate()) + ' ' +
    calPad(d.getUTCHours()) + ':' + calPad(d.getUTCMinutes()) + ':' + calPad(d.getUTCSeconds());
}
async function calDrop(ev, toKey) {
  ev.preventDefault();
  const id = calState.dragId; calState.dragId = null;
  if (!id) return;
  const e = calState.events.find(x => x.id === id && !x.source);
  if (!e) return;
  const cal = calState.calendars.find(c => c.id === e.calendar_id);
  if (!cal || !cal.can_edit) return;
  const delta = calDayDelta(calEventDayKey(e), toKey);
  if (!delta) return;
  try {
    await api('/api/calendar/events/' + id, 'PUT', {
      calendar_id: e.calendar_id, title: e.title, location: e.location, description: e.description,
      all_day: e.all_day, rrule: e.rrule || '',
      start: calShiftUTCDays(e.start, delta), end: calShiftUTCDays(e.end, delta),
    });
    await calLoadAndRender();
  } catch (err) { alert(err.message); }
}

function calWeekdayNames() {
  // Monday-first short weekday names in the active locale
  const base = new Date(2024, 0, 1); // a Monday
  const f = new Intl.DateTimeFormat(calLocale(), { weekday: 'short' });
  const out = [];
  for (let i = 0; i < 7; i++) { const d = new Date(base); d.setDate(base.getDate() + i); out.push(f.format(d)); }
  return out;
}

function calEventsByDay() {
  const map = {};
  for (const e of calVisibleEvents()) { const k = calEventDayKey(e); (map[k] = map[k] || []).push(e); }
  for (const k in map) map[k].sort((a, b) => a.start.localeCompare(b.start));
  return map;
}

function calRenderGrid(el) {
  const { start } = calPeriodRange();
  const days = calState.view === 'week' ? 7 : 42;
  const byDay = calEventsByDay();
  const todayKey = calDayKeyLocal(new Date());
  const curMonth = calState.cursor.getMonth();
  let html = `<div class="cal-grid cal-grid-${calState.view}">`;
  calWeekdayNames().forEach((w, i) => { html += `<div class="cal-wd ${i >= 5 ? 'cal-wd-weekend' : ''}">${esc(w)}</div>`; });
  for (let i = 0; i < days; i++) {
    const d = new Date(start); d.setDate(start.getDate() + i);
    const key = calDayKeyLocal(d);
    const out = calState.view === 'month' && d.getMonth() !== curMonth ? 'cal-out' : '';
    const today = key === todayKey ? 'cal-today' : '';
    const weekend = (d.getDay() === 0 || d.getDay() === 6) ? 'cal-weekend' : '';
    const evs = byDay[key] || [];
    html += `<div class="cal-cell ${out} ${today} ${weekend}" onclick="calCellClick('${key}')" ondragover="event.preventDefault()" ondrop="calDrop(event,'${key}')">
      <div class="cal-cell-date">${d.getDate()}</div>
      <div class="cal-cell-events">
        ${evs.map(e => calEventChip(e)).join('')}
      </div>
    </div>`;
  }
  html += `</div>`;
  el.innerHTML = html;
}

function calEventChip(e) {
  const color = e.color || '#6366f1';
  const time = e.all_day ? '' : calFmtTime(e.start) + ' ';
  const cal = calState.calendars.find(c => c.id === e.calendar_id);
  const drag = !e.source && !e.rrule && cal && cal.can_edit;
  return `<div class="cal-chip" style="--c:${escAttr(color)}" ${drag ? `draggable="true" ondragstart="calDragStart(event, ${e.id})"` : ''} onclick="event.stopPropagation();calEventClick(${e.id}, '${esc(e.source || '')}', ${e.ref_id || 0})" title="${escAttr(e.title)}">
    <span class="cal-chip-dot"></span>${esc(time)}${esc(e.title)}</div>`;
}

function calRenderAgenda(el) {
  const evs = calVisibleEvents().slice().sort((a, b) => a.start.localeCompare(b.start));
  if (!evs.length) { el.innerHTML = `<div class="cal-empty cal-empty-lg">${t('calNoEvents')}</div>`; return; }
  const byDay = {};
  for (const e of evs) { const k = calEventDayKey(e); (byDay[k] = byDay[k] || []).push(e); }
  const fmt = new Intl.DateTimeFormat(calLocale(), { weekday: 'long', day: 'numeric', month: 'long' });
  let html = '<div class="cal-agenda">';
  for (const k of Object.keys(byDay).sort()) {
    html += `<div class="cal-agenda-day"><div class="cal-agenda-date">${esc(fmt.format(calParseUTC(k + ' 12:00:00')))}</div>`;
    for (const e of byDay[k]) html += calAgendaRow(e);
    html += `</div>`;
  }
  html += '</div>';
  el.innerHTML = html;
}

// === Interactions ===
function calCellClick(dayKey) {
  if (!currentUser || currentUser.role === 'readonly') return;
  calNewEventDialog(dayKey);
}
function calEventClick(id, source, refId) {
  if (source === 'task') { navigateTo('/task/' + refId); return; }
  if (source === 'sprint') { navigateTo('/sprint/' + refId); return; }
  if (source === 'ics') {
    const e = calState.events.find(x => x.source === 'ics' && x.id === id);
    if (e) calIcsDialog(e);
    return;
  }
  const e = calState.events.find(x => x.id === id && !x.source);
  if (e) calEventDialog(e);
}

// calIcsDialog shows a read-only external (subscription) event.
function calIcsDialog(e) {
  const when = e.all_day ? calFmtDay(e.start) : (calFmtDateTime(e.start) + ' – ' + calFmtDateTime(e.end));
  let body = `<div class="cal-ics-when">${esc(when)}</div>`;
  if (e.location) body += `<div class="cal-ics-row">📍 ${esc(e.location)}</div>`;
  if (e.description) body += `<div class="cal-ics-row cal-ics-desc">${esc(e.description)}</div>`;
  if (e.calendar_name) body += `<div class="cal-ics-row cal-ics-src">${esc(e.calendar_name)}</div>`;
  calModal(e.title || t('calNoTitle') || '—', body, null);
}

// === ICS subscriptions ===
function calSubscriptionDialog() {
  document.querySelectorAll('.modal-overlay').forEach(m => m.remove());
  const subs = calState.subscriptions || [];
  const list = subs.map(s => `<div class="cal-sub-row">
      <span class="cal-dot" style="background:${escAttr(s.color)}"></span>
      <span class="cal-sub-name">${esc(s.name)}${s.last_error ? ` <span class="cal-sub-err" title="${escAttr(s.last_error)}">&#9888;</span>` : ''}</span>
      <button class="btn btn-ghost btn-sm" onclick="calSyncSubscription(${s.id})">${t('calSyncNow')}</button>
      <button class="btn-icon btn-sm cal-danger" title="${escAttr(t('calSubDelete'))}" onclick="calDeleteSubscription(${s.id})">&#10005;</button>
    </div>`).join('') || `<div class="cal-empty">—</div>`;
  const body = `
    <div class="cal-sub-list">${list}</div>
    <div class="cal-sub-add">
      <input id="cal-sub-name" placeholder="${escAttr(t('calIcsName') || 'Name')}">
      <input id="cal-sub-url" placeholder="${escAttr(t('calIcsUrl'))}">
      <input id="cal-sub-color" type="color" value="#0ea5e9">
    </div>
    <div class="cal-ics-hint">${esc(t('calIcsHint'))}</div>`;
  calModal(t('calAddIcsSub'), body, async ov => {
    const url = ov.querySelector('#cal-sub-url').value.trim();
    if (!url) return;
    const name = ov.querySelector('#cal-sub-name').value.trim();
    const color = ov.querySelector('#cal-sub-color').value;
    try {
      await api('/api/calendar/subscriptions', 'POST', { name, url, color });
      calState.subscriptions = await api('/api/calendar/subscriptions');
      ov.remove();
      calRenderHeader();
    } catch (e) { alert(e.message); }
  });
}

async function calDeleteSubscription(id) {
  if (!await uiConfirm(t('calSubDelete') + '?')) return;
  try {
    await api('/api/calendar/subscriptions/' + id, 'DELETE');
    calState.subscriptions = await api('/api/calendar/subscriptions');
    document.querySelectorAll('.modal-overlay').forEach(m => m.remove());
    calRenderHeader();
    await calLoadAndRender();
  } catch (e) { alert(e.message); }
}

async function calSyncSubscription(id) {
  try {
    await api('/api/calendar/subscriptions/' + id + '/sync', 'POST', {});
    // Server syncs in the background and pushes a calendar SSE event when done.
  } catch (e) { alert(e.message); }
}

// === Dialogs ===
function calModal(title, bodyHtml, onSave) {
  const ov = document.createElement('div');
  ov.className = 'modal-overlay';
  ov.innerHTML = `<div class="modal cal-modal">
    <h3>${esc(title)}</h3>
    <div class="cal-modal-body">${bodyHtml}</div>
    <div class="cal-modal-foot">
      <button class="btn btn-ghost btn-sm" onclick="this.closest('.modal-overlay').remove()">${t('calCancel')}</button>
      ${onSave ? `<button class="btn btn-primary btn-sm" id="cal-modal-save">${t('calSave')}</button>` : ''}
    </div>
  </div>`;
  document.body.appendChild(ov);
  ov.addEventListener('click', e => { if (e.target === ov) ov.remove(); });
  if (onSave) ov.querySelector('#cal-modal-save').addEventListener('click', () => onSave(ov));
  return ov;
}

function calEditableCalendars() { return calState.calendars.filter(c => c.can_edit); }

function calEventForm(ev) {
  const cals = calEditableCalendars();
  const rr = ev ? calParseRRuleFreq(ev.rrule) : { freq: '', count: '' };
  const startV = ev ? calUTCToInput(ev.start) : calDefaultStart();
  const endV = ev ? calUTCToInput(ev.end) : calDefaultEnd();
  return `
    <label class="cal-fld">${t('calTitle')}<input id="cal-f-title" maxlength="200" value="${ev ? escAttr(ev.title) : ''}"></label>
    <label class="cal-fld">${t('calCalendar')}
      <select id="cal-f-cal">${cals.map(c => `<option value="${c.id}" ${ev && ev.calendar_id === c.id ? 'selected' : ''}>${esc(c.name)}</option>`).join('')}</select>
    </label>
    <label class="cal-fld cal-fld-check"><input type="checkbox" id="cal-f-allday" ${ev && ev.all_day ? 'checked' : ''}> ${t('calAllDay')}</label>
    <div class="cal-fld-row">
      <label class="cal-fld">${t('calStart')}<input type="datetime-local" id="cal-f-start" value="${startV}"></label>
      <label class="cal-fld">${t('calEnd')}<input type="datetime-local" id="cal-f-end" value="${endV}"></label>
    </div>
    <label class="cal-fld">${t('calLocation')}<input id="cal-f-loc" maxlength="100" value="${ev ? escAttr(ev.location) : ''}"></label>
    <div class="cal-fld-row">
      <label class="cal-fld">${t('calRepeat')}
        <select id="cal-f-freq">
          ${[['', 'calRepeatNone'], ['DAILY', 'calRepeatDaily'], ['WEEKLY', 'calRepeatWeekly'], ['MONTHLY', 'calRepeatMonthly'], ['YEARLY', 'calRepeatYearly']]
      .map(([v, k]) => `<option value="${v}" ${rr.freq === v ? 'selected' : ''}>${t(k)}</option>`).join('')}
        </select>
      </label>
      <label class="cal-fld">${t('calRepeatCount')}<input type="number" min="1" max="999" id="cal-f-count" value="${rr.count}"></label>
    </div>
    <label class="cal-fld">${t('calDescription')}<textarea id="cal-f-desc" maxlength="4000" rows="3">${ev ? esc(ev.description) : ''}</textarea></label>
    <div class="cal-fld"><div class="cal-rem-head">${t('calReminders')}</div>
      <div id="cal-reminders"></div>
      <button type="button" class="btn btn-ghost btn-sm" onclick="calAddReminder()">+ ${t('calReminderAdd')}</button>
    </div>
    ${ev ? `<button class="btn btn-ghost btn-sm cal-danger cal-del-btn" onclick="calDeleteEvent(${ev.id})">${t('calDelete')}</button>` : ''}`;
}

function calDefaultStart() {
  const d = calState.cursor && calState.view !== 'agenda' ? new Date(calState.cursor) : new Date();
  d.setHours(new Date().getHours() + 1, 0, 0, 0);
  return calDayKeyLocal(d) + 'T' + calPad(d.getHours()) + ':00';
}
function calDefaultEnd() {
  const s = calDefaultStart();
  const d = new Date(s); d.setHours(d.getHours() + 1);
  return calUTCToInput(calInputToUTC(calDayKeyLocal(d) + 'T' + calPad(d.getHours()) + ':' + calPad(d.getMinutes())));
}

function calParseRRuleFreq(rrule) {
  if (!rrule) return { freq: '', count: '' };
  const m = {}; rrule.split(';').forEach(p => { const kv = p.split('='); if (kv.length === 2) m[kv[0].toUpperCase()] = kv[1]; });
  return { freq: (m.FREQ || '').toUpperCase(), count: m.COUNT || '' };
}
function calBuildRRule() {
  const freq = document.getElementById('cal-f-freq').value;
  if (!freq) return '';
  const count = parseInt(document.getElementById('cal-f-count').value, 10);
  return 'FREQ=' + freq + (count > 0 ? ';COUNT=' + count : '');
}

function calCollectEvent(ov) {
  const allDay = document.getElementById('cal-f-allday').checked;
  let start = document.getElementById('cal-f-start').value;
  let end = document.getElementById('cal-f-end').value;
  if (allDay) {
    const sd = start.slice(0, 10) || calDayKeyLocal(new Date());
    const ed = (end.slice(0, 10) || sd);
    start = sd + 'T00:00'; end = ed + 'T23:59';
  }
  return {
    calendar_id: Number(document.getElementById('cal-f-cal').value),
    title: document.getElementById('cal-f-title').value.trim(),
    location: document.getElementById('cal-f-loc').value.trim(),
    description: document.getElementById('cal-f-desc').value,
    all_day: allDay,
    start: calInputToUTC(start),
    end: calInputToUTC(end),
    rrule: calBuildRRule(),
    reminders: calReminders.map(r => ({ lead_min: Math.max(0, (r.value || 0) * (CAL_REM_UNIT[r.unit] || 1)), channel: r.channel })),
  };
}

function calNewEventDialog(dayKey) {
  if (!calEditableCalendars().length) { alert(t('calNoCalendars')); calNewCalendarDialog(); return; }
  calReminders = [];
  const ov = calModal(t('calNewEvent'), calEventForm(null), async ov => {
    const ev = calCollectEvent(ov);
    if (!ev.title) { alert(t('calTitleRequired')); return; }
    try { await api('/api/calendar/events', 'POST', ev); ov.remove(); await calLoadAndRender(); }
    catch (e) { alert(e.message); }
  });
  if (dayKey) {
    const si = ov.querySelector('#cal-f-start'), ei = ov.querySelector('#cal-f-end');
    si.value = dayKey + 'T09:00'; ei.value = dayKey + 'T10:00';
  }
  calRenderReminders();
}

async function calEventDialog(ev) {
  const canEdit = (calState.calendars.find(c => c.id === ev.calendar_id) || {}).can_edit;
  if (!canEdit) { // view-only
    calModal(ev.title, `<div class="cal-view">
      <div>${esc(ev.all_day ? t('calAllDay') : calFmtTime(ev.start) + ' – ' + calFmtTime(ev.end))}</div>
      ${ev.location ? `<div class="cal-view-loc">${esc(ev.location)}</div>` : ''}
      ${ev.description ? `<div class="cal-view-desc">${esc(ev.description)}</div>` : ''}
    </div>`, null);
    return;
  }
  // Fetch the full event to get its reminder definitions.
  let full = ev;
  try { full = await api('/api/calendar/events/' + ev.id); } catch (e) { /* use list copy */ }
  calReminders = (full.reminders || []).map(calRemToDraft);
  calModal(t('calEditEvent'), calEventForm(full), async ov => {
    const upd = calCollectEvent(ov);
    if (!upd.title) { alert(t('calTitleRequired')); return; }
    try { await api('/api/calendar/events/' + ev.id, 'PUT', upd); ov.remove(); await calLoadAndRender(); }
    catch (e) { alert(e.message); }
  });
  calRenderReminders();
}

async function calDeleteEvent(id) {
  if (!await uiConfirm(t('calDeleteConfirm'))) return;
  try {
    await api('/api/calendar/events/' + id, 'DELETE');
    document.querySelectorAll('.modal-overlay').forEach(m => m.remove());
    await calLoadAndRender();
  } catch (e) { alert(e.message); }
}

// === Calendar management ===
function calCalendarForm(c) {
  return `
    <label class="cal-fld">${t('calName')}<input id="cal-c-name" maxlength="100" value="${c ? escAttr(c.name) : ''}"></label>
    <label class="cal-fld">${t('calColor')}<input type="color" id="cal-c-color" value="${c ? escAttr(c.color) : '#6366f1'}"></label>
    <label class="cal-fld">${t('calVisibility')}
      <select id="cal-c-vis">
        <option value="private" ${c && c.visibility === 'private' ? 'selected' : ''}>${t('calPrivate')}</option>
        <option value="shared" ${c && c.visibility === 'shared' ? 'selected' : ''}>${t('calShared')}</option>
      </select>
    </label>
    ${c ? `<button class="btn btn-ghost btn-sm cal-danger cal-del-btn" onclick="calDeleteCalendar(${c.id})">${t('calDeleteCalendar')}</button>` : ''}`;
}

function calNewCalendarDialog() {
  calModal(t('calNewCalendar'), calCalendarForm(null), async ov => {
    const name = ov.querySelector('#cal-c-name').value.trim();
    if (!name) { alert(t('calNameRequired')); return; }
    const body = { name, color: ov.querySelector('#cal-c-color').value, visibility: ov.querySelector('#cal-c-vis').value };
    try { await api('/api/calendar/calendars', 'POST', body); ov.remove(); calState.calendars = await api('/api/calendar/calendars'); calRenderCalList(); await calLoadAndRender(); }
    catch (e) { alert(e.message); }
  });
}

function calEditCalendarDialog(id) {
  const c = calState.calendars.find(x => x.id === id);
  if (!c) return;
  calModal(t('calEditCalendar'), calCalendarForm(c), async ov => {
    const name = ov.querySelector('#cal-c-name').value.trim();
    if (!name) { alert(t('calNameRequired')); return; }
    const body = { name, color: ov.querySelector('#cal-c-color').value, visibility: ov.querySelector('#cal-c-vis').value };
    try { await api('/api/calendar/calendars/' + id, 'PUT', body); ov.remove(); calState.calendars = await api('/api/calendar/calendars'); calRenderCalList(); await calLoadAndRender(); }
    catch (e) { alert(e.message); }
  });
}

async function calDeleteCalendar(id) {
  if (!await uiConfirm(t('calDeleteCalendarConfirm'))) return;
  try {
    await api('/api/calendar/calendars/' + id, 'DELETE');
    document.querySelectorAll('.modal-overlay').forEach(m => m.remove());
    calState.calendars = await api('/api/calendar/calendars');
    calRenderCalList();
    await calLoadAndRender();
  } catch (e) { alert(e.message); }
}
