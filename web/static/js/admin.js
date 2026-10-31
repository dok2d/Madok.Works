// Admin page. Reached from the bottom app-rail gear (admins only) at /admin.
// Renders a tabbed page in a persistent top-level #admin-page element and relocates
// the existing admin-only sidebar sections into its panels (ids preserved, so all
// existing populate functions keep working). Phase 7 adds Mail server / Mailboxes.

const ADMIN_I18N = {
  en: { adminTitle: 'Administration', settingsTitle: 'Settings', adminTabGeneral: 'My settings', adminTabAccount: 'Account', accProfile: 'Open profile', accNotifTitle: 'Notifications', accSound: 'New message sound', accSoundHint: 'Plays a short sound for incoming chat messages (muted channels stay silent).', adminTabSite: 'Site', adminAppName: 'Application name', adminAppNameHint: 'Shown in the page title, header and backup names.', adminDefaultLang: 'Default language', adminDefaultLangHint: 'Interface language for new users (each user can change their own afterwards).', adminWap: 'Enable WAP interface (legacy phones)', adminWapHint: 'Exposes a minimal WML interface at /wap/ for very old devices. Off by default.', adminUploadAny: 'Allow any file type for attachments', adminUploadExts: 'Allowed file extensions', adminUploadHint: 'Applies to all attachments (wiki/chat/tasks/mail). Empty = built-in safe defaults; or list extensions like "pdf, png, docx, zip"; or tick "allow any". Files are always served as downloads.', obReplayTitle: 'First-run wizard', obReplay: 'Show setup wizard again', obReplayHint: 'Re-open the one-time service-setup wizard (clears the completed flag).', adminTabTelegram: 'Telegram', adminTabMail: 'Mail', adminTabUsers: 'Users', adminTabBoard: 'Board', adminTabProjects: 'Projects', adminProjNew: 'New project', adminProjVisibility: 'Visibility', adminProjMembers: 'Members', adminProjManage: 'Manage', adminProjNoneSel: 'Select a project to manage its members and settings.', adminTabServices: 'Services', adminTabIntegrations: 'Integrations', adminTabData: 'Data', adminTabMailServer: 'Mail server', adminTabMailboxes: 'Mailboxes', adminMailDomain: 'Sending domain', adminDkimSelector: 'DKIM selector', adminDkimKey: 'DKIM private key (PEM)', adminDkimKeyHint: 'leave blank to keep current', adminDkimTxt: 'Publish this DNS TXT record', adminSave: 'Save', adminAudit: 'Security audit', adminAlias: 'Alias', adminPrimary: 'Primary', adminAdd: 'Add', adminUser: 'User', adminNoData: 'No data', adminMailDisabled: 'Outbound mail is off until a sending domain is set.', adminClear: 'Clear key', callsTitle: 'Calls (WebRTC)', callsHint: 'Voice/video calls use peer-to-peer media. STUN is built-in; add a self-hosted TURN server for reliable calls across NAT/firewalls.', callsTurnUrl: 'TURN URL', callsTurnUser: 'TURN username', callsTurnCred: 'TURN credential',
gitTitle: 'Git repositories', gitHint: 'Connect GitHub/GitLab/Gitea: incoming webhooks link commits/branches/MRs to tasks by KEY-<n> / #<n>; a merged MR can auto-move the task. Add the shown webhook URL in your repo settings (with the secret).', gitAddRepo: 'Add repository', gitWebhookUrl: 'Webhook URL', gitKind: 'Provider', gitLabel: 'Label', gitRepo: 'Repository (owner/name)', gitSecret: 'Webhook secret', gitMoveCol: 'On merge → move to column', gitActivity: 'Git activity',
llmTitle: 'AI summaries (LLM)', dbTitle: 'Database', llmHint: 'Connect an OpenAI-compatible API (e.g. llama.cpp). When enabled: a "Summary" button on wiki pages and "✨ Generate" (description + TODO) in the task form.', llmEnabledOn: 'Enabled', llmBaseUrl: 'API base URL', llmModel: 'Model', llmApiKey: 'API key', llmApiKeyOptional: 'optional', llmEnable: 'Enable (tests connection)', llmTest: 'Test connection', llmTesting: 'Testing…', llmOk: 'Connection OK', llmNoThink: 'Disable reasoning ("thinking")', llmNoThinkHint: 'Faster answers — appends /no_think (Qwen3-style models). Harmless for other models.', gcalTitle: 'Google Calendar (two-way)', gcalHint: 'Register an OAuth client in Google Cloud (APIs & Services → Credentials → OAuth client ID → Web application), enable the Google Calendar API, and add the redirect URI below. Users then connect their own Google account from the Calendar.', gcalConfigured: 'Configured', gcalRedirect: 'Authorized redirect URI', gcalClientId: 'OAuth client ID', gcalSecret: 'OAuth client secret', gcalEnable: 'Enable Google Calendar sync', dbHost: 'Host', dbPort: 'Port', dbName: 'Database', dbUser: 'User', dbPath: 'File', adminTabAudit: 'Audit log', auditTime: 'Time', auditUser: 'User', auditAction: 'Action', auditProject: 'Project', auditDetails: 'Details', auditAllProjects: 'All projects', adminTabSecurity: 'Security', twofaTitle: 'Two-factor authentication', twofaHint: 'Add a time-based one-time code (TOTP) from an authenticator app as a second login step.', twofaOn: 'Two-factor authentication is enabled.', twofaEnable: 'Enable 2FA', twofaDisable: 'Disable 2FA', twofaCode: 'Code', twofaScan: 'Add this secret to your authenticator app (or scan the otpauth URL), then enter the current code:', twofaConfirm: 'Confirm', twofaEnabled: '2FA enabled.', twofaRecovery: 'Save these one-time recovery codes somewhere safe:' },
  ru: { adminTitle: 'Администрирование', settingsTitle: 'Настройки', adminTabGeneral: 'Мои настройки', adminTabAccount: 'Аккаунт', accProfile: 'Открыть профиль', accNotifTitle: 'Уведомления', accSound: 'Звук нового сообщения', accSoundHint: 'Короткий звук при входящих сообщениях чата (заглушённые каналы остаются тихими).', adminTabSite: 'Сайт', adminAppName: 'Название приложения', adminAppNameHint: 'Показывается в заголовке вкладки, шапке и именах бэкапов.', adminDefaultLang: 'Язык по умолчанию', adminDefaultLangHint: 'Язык интерфейса для новых пользователей (каждый может потом сменить свой).', adminWap: 'Включить WAP-интерфейс (старые телефоны)', adminWapHint: 'Открывает минимальный WML-интерфейс по адресу /wap/ для очень старых устройств. По умолчанию выключен.', adminUploadAny: 'Разрешить любые форматы файлов для вложений', adminUploadExts: 'Разрешённые расширения файлов', adminUploadHint: 'Действует на все вложения (вики/чат/задачи/почта). Пусто = безопасные значения по умолчанию; либо список расширений вроде «pdf, png, docx, zip»; либо галочка «разрешить любые». Файлы всегда отдаются на скачивание.', obReplayTitle: 'Визард первого запуска', obReplay: 'Показать визард заново', obReplayHint: 'Заново открыть одноразовый визард настройки сервисов (сбрасывает флаг завершения).', adminTabTelegram: 'Telegram', adminTabMail: 'Почта', adminTabUsers: 'Пользователи', adminTabBoard: 'Доска', adminTabProjects: 'Проекты', adminProjNew: 'Новый проект', adminProjVisibility: 'Видимость', adminProjMembers: 'Участники', adminProjManage: 'Управлять', adminProjNoneSel: 'Выберите проект, чтобы управлять его участниками и настройками.', adminTabServices: 'Сервисы', adminTabIntegrations: 'Интеграции', adminTabData: 'Данные', adminTabMailServer: 'Почтовый сервер', adminTabMailboxes: 'Ящики', adminMailDomain: 'Домен отправки', adminDkimSelector: 'DKIM селектор', adminDkimKey: 'Приватный ключ DKIM (PEM)', adminDkimKeyHint: 'пусто — оставить текущий', adminDkimTxt: 'Опубликуйте эту DNS TXT-запись', adminSave: 'Сохранить', adminAudit: 'Аудит безопасности', adminAlias: 'Алиас', adminPrimary: 'Основной', adminAdd: 'Добавить', adminUser: 'Пользователь', adminNoData: 'Нет данных', adminMailDisabled: 'Исходящая почта выключена, пока не задан домен отправки.', adminClear: 'Очистить ключ', callsTitle: 'Звонки (WebRTC)', callsHint: 'Голосовые/видеозвонки используют P2P-медиа. STUN встроен; добавьте свой TURN-сервер для надёжных звонков через NAT/файрволы.', callsTurnUrl: 'TURN URL', callsTurnUser: 'TURN логин', callsTurnCred: 'TURN ключ',
gitTitle: 'Git-репозитории', gitHint: 'Подключите GitHub/GitLab/Gitea: входящие вебхуки связывают коммиты/ветки/MR с задачами по KEY-<n> / #<n>; смёрженный MR может авто-переместить задачу. Добавьте показанный webhook URL в настройках репозитория (с секретом).', gitAddRepo: 'Добавить репозиторий', gitWebhookUrl: 'Webhook URL', gitKind: 'Провайдер', gitLabel: 'Метка', gitRepo: 'Репозиторий (owner/name)', gitSecret: 'Секрет вебхука', gitMoveCol: 'При мёрже → в колонку', gitActivity: 'Git-активность',
llmTitle: 'ИИ-саммари (LLM)', dbTitle: 'База данных', llmHint: 'Подключите OpenAI-совместимый API (напр. llama.cpp). После включения появятся: кнопка «Саммари» на wiki-страницах и «✨ Сгенерировать» (описание + TODO) в форме задачи.', llmEnabledOn: 'Включено', llmBaseUrl: 'Базовый URL API', llmModel: 'Модель', llmApiKey: 'API-ключ', llmApiKeyOptional: 'необязательно', llmEnable: 'Включить (с проверкой подключения)', llmTest: 'Проверить подключение', llmTesting: 'Проверка…', llmOk: 'Подключение успешно', llmNoThink: 'Отключить «думанье» (reasoning)', llmNoThinkHint: 'Быстрее отвечает — добавляет /no_think (модели типа Qwen3). Для других моделей безвредно.', gcalTitle: 'Google Календарь (двусторонний)', gcalHint: 'Зарегистрируйте OAuth-клиент в Google Cloud (APIs & Services → Credentials → OAuth client ID → Web application), включите Google Calendar API и добавьте redirect URI ниже. После этого пользователи подключают свой аккаунт Google прямо в Календаре.', gcalConfigured: 'Настроено', gcalRedirect: 'Разрешённый redirect URI', gcalClientId: 'OAuth client ID', gcalSecret: 'OAuth client secret', gcalEnable: 'Включить синхронизацию Google Календаря', dbHost: 'Хост', dbPort: 'Порт', dbName: 'База', dbUser: 'Пользователь', dbPath: 'Файл', adminTabAudit: 'Журнал аудита', auditTime: 'Время', auditUser: 'Пользователь', auditAction: 'Действие', auditProject: 'Проект', auditDetails: 'Детали', auditAllProjects: 'Все проекты', adminTabSecurity: 'Безопасность', twofaTitle: 'Двухфакторная аутентификация', twofaHint: 'Добавьте одноразовый код (TOTP) из приложения-аутентификатора как второй шаг входа.', twofaOn: 'Двухфакторная аутентификация включена.', twofaEnable: 'Включить 2FA', twofaDisable: 'Отключить 2FA', twofaCode: 'Код', twofaScan: 'Добавьте этот секрет в приложение-аутентификатор (или отсканируйте otpauth-ссылку), затем введите текущий код:', twofaConfirm: 'Подтвердить', twofaEnabled: '2FA включена.', twofaRecovery: 'Сохраните эти одноразовые коды восстановления в надёжном месте:' },
  es: { adminTitle: 'Administración', adminTabUsers: 'Usuarios', adminTabBoard: 'Tablero', adminTabServices: 'Servicios', adminTabIntegrations: 'Integraciones', adminTabData: 'Datos', adminTabMailServer: 'Servidor de correo', adminTabMailboxes: 'Buzones' },
  fr: { adminTitle: 'Administration', adminTabUsers: 'Utilisateurs', adminTabBoard: 'Tableau', adminTabServices: 'Services', adminTabIntegrations: 'Intégrations', adminTabData: 'Données', adminTabMailServer: 'Serveur de messagerie', adminTabMailboxes: 'Boîtes' },
  de: { adminTitle: 'Verwaltung', adminTabUsers: 'Benutzer', adminTabBoard: 'Board', adminTabServices: 'Dienste', adminTabIntegrations: 'Integrationen', adminTabData: 'Daten', adminTabMailServer: 'Mailserver', adminTabMailboxes: 'Postfächer' },
  pt: { adminTitle: 'Administração', adminTabUsers: 'Usuários', adminTabBoard: 'Quadro', adminTabServices: 'Serviços', adminTabIntegrations: 'Integrações', adminTabData: 'Dados', adminTabMailServer: 'Servidor de e-mail', adminTabMailboxes: 'Caixas' },
};
// API tokens tab strings, merged into ADMIN_I18N below (all 10 UI languages).
const TOKEN_I18N = {
  en: { tokTitle: "API tokens", tokHint: "Personal tokens for scripts and integrations: send \"Authorization: Bearer <token>\" to /api/*. A token acts as you, limited to its role (never above your own) and, optionally, to some services. Password, 2FA, Telegram and tokens themselves stay browser-only.", tokName: "Name", tokRole: "Role", tokServices: "Services", tokAllServices: "All services", tokSvcHint: "None ticked = all services.", tokExpires: "Expires", tokNever: "Never", tokDays: "days", tokCreate: "Create token", tokPrefix: "Token", tokLastUsed: "Last used", tokCreated: "Created", tokRevoke: "Revoke", tokRevokeConfirm: "Revoke this token? Anything using it stops working immediately.", tokNew: "Copy the token now — it will not be shown again:", tokCopy: "Copy", tokCopied: "Token copied", tokNone: "No tokens yet", tokNameRequired: "Enter a token name", tokAllTitle: "All users' tokens", tokOwner: "User", tokExpired: "expired" },
  ru: { tokTitle: "API-токены", tokHint: "Личные токены для скриптов и интеграций: передавайте «Authorization: Bearer <токен>» в запросах к /api/*. Токен действует от вашего имени, но в рамках своей роли (не выше вашей) и, при желании, только в выбранных сервисах. Пароль, 2FA, Telegram и сами токены меняются только из браузера.", tokName: "Название", tokRole: "Роль", tokServices: "Сервисы", tokAllServices: "Все сервисы", tokSvcHint: "Ничего не отмечено — все сервисы.", tokExpires: "Срок действия", tokNever: "Бессрочно", tokDays: "дней", tokCreate: "Создать токен", tokPrefix: "Токен", tokLastUsed: "Использован", tokCreated: "Создан", tokRevoke: "Отозвать", tokRevokeConfirm: "Отозвать токен? Всё, что его использует, сразу перестанет работать.", tokNew: "Скопируйте токен сейчас — больше он показан не будет:", tokCopy: "Копировать", tokCopied: "Токен скопирован", tokNone: "Токенов пока нет", tokNameRequired: "Введите название токена", tokAllTitle: "Токены всех пользователей", tokOwner: "Пользователь", tokExpired: "истёк" },
  zh: { tokTitle: "API 令牌", tokHint: "用于脚本和集成的个人令牌：在请求 /api/* 时发送 \"Authorization: Bearer <令牌>\"。令牌以您的身份操作，但受其角色限制（不会高于您自己的角色），并可限定于部分服务。密码、两步验证、Telegram 和令牌本身只能在浏览器中管理。", tokName: "名称", tokRole: "角色", tokServices: "服务", tokAllServices: "所有服务", tokSvcHint: "未勾选 = 所有服务。", tokExpires: "有效期", tokNever: "永不过期", tokDays: "天", tokCreate: "创建令牌", tokPrefix: "令牌", tokLastUsed: "最近使用", tokCreated: "创建时间", tokRevoke: "撤销", tokRevokeConfirm: "撤销此令牌？使用它的一切将立即失效。", tokNew: "请立即复制令牌——它不会再次显示：", tokCopy: "复制", tokCopied: "令牌已复制", tokNone: "暂无令牌", tokNameRequired: "请输入令牌名称", tokAllTitle: "所有用户的令牌", tokOwner: "用户", tokExpired: "已过期" },
  es: { tokTitle: "Tokens de API", tokHint: "Tokens personales para scripts e integraciones: envía \"Authorization: Bearer <token>\" a /api/*. Un token actúa en tu nombre, limitado a su rol (nunca superior al tuyo) y, opcionalmente, a algunos servicios. La contraseña, la 2FA, Telegram y los propios tokens solo se gestionan desde el navegador.", tokName: "Nombre", tokRole: "Rol", tokServices: "Servicios", tokAllServices: "Todos los servicios", tokSvcHint: "Ninguno marcado = todos los servicios.", tokExpires: "Caduca", tokNever: "Nunca", tokDays: "días", tokCreate: "Crear token", tokPrefix: "Token", tokLastUsed: "Último uso", tokCreated: "Creado", tokRevoke: "Revocar", tokRevokeConfirm: "¿Revocar este token? Todo lo que lo use dejará de funcionar de inmediato.", tokNew: "Copia el token ahora: no se volverá a mostrar:", tokCopy: "Copiar", tokCopied: "Token copiado", tokNone: "Aún no hay tokens", tokNameRequired: "Introduce un nombre para el token", tokAllTitle: "Tokens de todos los usuarios", tokOwner: "Usuario", tokExpired: "caducado" },
  fr: { tokTitle: "Jetons d'API", tokHint: "Jetons personnels pour scripts et intégrations : envoyez « Authorization: Bearer <jeton> » vers /api/*. Un jeton agit en votre nom, limité à son rôle (jamais au-dessus du vôtre) et, si vous le souhaitez, à certains services. Mot de passe, 2FA, Telegram et jetons eux-mêmes se gèrent uniquement depuis le navigateur.", tokName: "Nom", tokRole: "Rôle", tokServices: "Services", tokAllServices: "Tous les services", tokSvcHint: "Aucune case cochée = tous les services.", tokExpires: "Expiration", tokNever: "Jamais", tokDays: "jours", tokCreate: "Créer un jeton", tokPrefix: "Jeton", tokLastUsed: "Dernière utilisation", tokCreated: "Créé", tokRevoke: "Révoquer", tokRevokeConfirm: "Révoquer ce jeton ? Tout ce qui l'utilise cessera immédiatement de fonctionner.", tokNew: "Copiez le jeton maintenant — il ne sera plus affiché :", tokCopy: "Copier", tokCopied: "Jeton copié", tokNone: "Aucun jeton pour l'instant", tokNameRequired: "Saisissez un nom de jeton", tokAllTitle: "Jetons de tous les utilisateurs", tokOwner: "Utilisateur", tokExpired: "expiré" },
  de: { tokTitle: "API-Tokens", tokHint: "Persönliche Tokens für Skripte und Integrationen: „Authorization: Bearer <Token>“ an /api/* senden. Ein Token handelt in Ihrem Namen, begrenzt auf seine Rolle (nie höher als Ihre eigene) und optional auf einzelne Dienste. Passwort, 2FA, Telegram und die Tokens selbst lassen sich nur im Browser verwalten.", tokName: "Name", tokRole: "Rolle", tokServices: "Dienste", tokAllServices: "Alle Dienste", tokSvcHint: "Nichts angehakt = alle Dienste.", tokExpires: "Gültig bis", tokNever: "Unbegrenzt", tokDays: "Tage", tokCreate: "Token erstellen", tokPrefix: "Token", tokLastUsed: "Zuletzt verwendet", tokCreated: "Erstellt", tokRevoke: "Widerrufen", tokRevokeConfirm: "Diesen Token widerrufen? Alles, was ihn nutzt, funktioniert sofort nicht mehr.", tokNew: "Kopieren Sie den Token jetzt – er wird nicht noch einmal angezeigt:", tokCopy: "Kopieren", tokCopied: "Token kopiert", tokNone: "Noch keine Tokens", tokNameRequired: "Geben Sie einen Token-Namen ein", tokAllTitle: "Tokens aller Benutzer", tokOwner: "Benutzer", tokExpired: "abgelaufen" },
  pt: { tokTitle: "Tokens de API", tokHint: "Tokens pessoais para scripts e integrações: envie \"Authorization: Bearer <token>\" para /api/*. Um token age em seu nome, limitado à sua função (nunca acima da sua) e, opcionalmente, a alguns serviços. Senha, 2FA, Telegram e os próprios tokens só podem ser geridos pelo navegador.", tokName: "Nome", tokRole: "Função", tokServices: "Serviços", tokAllServices: "Todos os serviços", tokSvcHint: "Nenhum marcado = todos os serviços.", tokExpires: "Expira", tokNever: "Nunca", tokDays: "dias", tokCreate: "Criar token", tokPrefix: "Token", tokLastUsed: "Último uso", tokCreated: "Criado", tokRevoke: "Revogar", tokRevokeConfirm: "Revogar este token? Tudo que o usa deixará de funcionar imediatamente.", tokNew: "Copie o token agora — ele não será exibido novamente:", tokCopy: "Copiar", tokCopied: "Token copiado", tokNone: "Nenhum token ainda", tokNameRequired: "Informe um nome para o token", tokAllTitle: "Tokens de todos os usuários", tokOwner: "Usuário", tokExpired: "expirado" },
  ja: { tokTitle: "API トークン", tokHint: "スクリプトや連携用の個人トークンです。/api/* へのリクエストで「Authorization: Bearer <トークン>」を送信します。トークンはあなたとして動作しますが、そのロール（あなた自身のロールを超えない）と、必要に応じて選択したサービスに限定されます。パスワード、2FA、Telegram、トークン自体はブラウザからのみ管理できます。", tokName: "名前", tokRole: "ロール", tokServices: "サービス", tokAllServices: "すべてのサービス", tokSvcHint: "未選択 = すべてのサービス。", tokExpires: "有効期限", tokNever: "無期限", tokDays: "日", tokCreate: "トークンを作成", tokPrefix: "トークン", tokLastUsed: "最終使用", tokCreated: "作成日時", tokRevoke: "取り消す", tokRevokeConfirm: "このトークンを取り消しますか？これを使用しているものはすぐに動作しなくなります。", tokNew: "今すぐトークンをコピーしてください。再表示されません：", tokCopy: "コピー", tokCopied: "トークンをコピーしました", tokNone: "トークンはまだありません", tokNameRequired: "トークン名を入力してください", tokAllTitle: "全ユーザーのトークン", tokOwner: "ユーザー", tokExpired: "期限切れ" },
  ko: { tokTitle: "API 토큰", tokHint: "스크립트와 연동을 위한 개인 토큰입니다. /api/* 요청에 \"Authorization: Bearer <토큰>\"을 보내세요. 토큰은 사용자 본인으로 동작하지만 해당 역할(본인의 역할을 넘지 않음)과 선택한 서비스로 제한됩니다. 비밀번호, 2FA, Telegram 및 토큰 자체는 브라우저에서만 관리할 수 있습니다.", tokName: "이름", tokRole: "역할", tokServices: "서비스", tokAllServices: "모든 서비스", tokSvcHint: "선택 안 함 = 모든 서비스.", tokExpires: "만료", tokNever: "만료 없음", tokDays: "일", tokCreate: "토큰 만들기", tokPrefix: "토큰", tokLastUsed: "마지막 사용", tokCreated: "생성", tokRevoke: "폐기", tokRevokeConfirm: "이 토큰을 폐기할까요? 이 토큰을 사용하는 모든 것이 즉시 작동을 멈춥니다.", tokNew: "지금 토큰을 복사하세요. 다시 표시되지 않습니다:", tokCopy: "복사", tokCopied: "토큰이 복사되었습니다", tokNone: "아직 토큰이 없습니다", tokNameRequired: "토큰 이름을 입력하세요", tokAllTitle: "모든 사용자의 토큰", tokOwner: "사용자", tokExpired: "만료됨" },
  ar: { tokTitle: "رموز API", tokHint: "رموز شخصية للبرامج النصية والتكاملات: أرسل \"Authorization: Bearer <الرمز>\" إلى /api/*. يعمل الرمز باسمك، لكنه مقيد بدوره (لا يتجاوز دورك أبدًا) وبخدمات محددة إن شئت. كلمة المرور والمصادقة الثنائية وTelegram والرموز نفسها تُدار من المتصفح فقط.", tokName: "الاسم", tokRole: "الدور", tokServices: "الخدمات", tokAllServices: "كل الخدمات", tokSvcHint: "عدم التحديد = كل الخدمات.", tokExpires: "تنتهي", tokNever: "أبدًا", tokDays: "يوم", tokCreate: "إنشاء رمز", tokPrefix: "الرمز", tokLastUsed: "آخر استخدام", tokCreated: "تاريخ الإنشاء", tokRevoke: "إلغاء", tokRevokeConfirm: "إلغاء هذا الرمز؟ سيتوقف كل ما يستخدمه عن العمل فورًا.", tokNew: "انسخ الرمز الآن — لن يُعرض مرة أخرى:", tokCopy: "نسخ", tokCopied: "تم نسخ الرمز", tokNone: "لا توجد رموز بعد", tokNameRequired: "أدخل اسمًا للرمز", tokAllTitle: "رموز جميع المستخدمين", tokOwner: "المستخدم", tokExpired: "منتهي" },
};
for (const l in TOKEN_I18N) ADMIN_I18N[l] = Object.assign(ADMIN_I18N[l] || {}, TOKEN_I18N[l]);
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
  // User-facing first. Account also hosts the personal display settings (theme/
  // language/font/timezone), relocated into its panel.
  { key: 'account', i: 'adminTabAccount', render: renderAdminAccount, sections: ['theme-section', 'lang-section', 'fontsize-section', 'timezone-section'] },
  { key: 'mail', i: 'adminTabMail', render: renderAdminMail, show: () => typeof isServiceEnabled === 'function' && isServiceEnabled('mail') && currentUser && currentUser.role !== 'readonly' },
  { key: 'mailserver', i: 'adminTabMailServer', render: renderAdminMailServer, admin: true, show: () => typeof isServiceEnabled === 'function' && isServiceEnabled('mail') && isAdmin() },
  // Admin-only service config (bot token, SSO, LLM, DB). Personal Telegram
  // linking lives in the Account tab instead.
  { key: 'integrations', i: 'adminTabIntegrations', sections: ['telegram-section', 'sso-section', 'llm-section', 'google-section', 'db-section', 'git-section', 'calls-section'], admin: true },
  // Admin configuration
  { key: 'board', i: 'adminTabBoard', sections: ['columns-section', 'epics-section', 'sprints-section', 'tags-section'], admin: true },
  { key: 'projects', i: 'adminTabProjects', render: renderAdminProjects, admin: true },
  { key: 'users', i: 'adminTabUsers', render: renderAdminUsersTokens, sections: ['users-section'], admin: true },
  { key: 'services', i: 'adminTabServices', sections: ['services-section'], admin: true },
  { key: 'site', i: 'adminTabSite', render: renderAdminSite, admin: true },
  { key: 'auditlog', i: 'adminTabAudit', render: renderAdminAuditLog, admin: true },
  { key: 'data', i: 'adminTabData', sections: ['export-import-section', 'backup-section'], admin: true },
];

// Top-level layout grouping leaf tabs under collapsible sections. A group shows
// a secondary tab bar of its children; a leaf is shown directly. Keys reference
// ADMIN_TABS entries. Order here defines the top-level tab order.
const ADMIN_LAYOUT = [
  { leaf: 'account' },
  { group: 'mailgrp', i: 'adminTabMail', children: ['mail', 'mailserver'] },
  { group: 'boardgrp', i: 'adminTabBoard', children: ['board', 'projects'] },
  { leaf: 'users' },
  { group: 'sitegrp', i: 'adminTabSite', children: ['site', 'services', 'integrations', 'data', 'auditlog'] },
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
  // The personal display sections (theme/language/font/timezone) are relocated
  // into this panel by ensureAdminPage; render account content into a host above
  // them and the logout into a foot below them, without wiping those sections.
  let host = panel.querySelector('#acc-render');
  if (!host) { host = document.createElement('div'); host.id = 'acc-render'; panel.insertBefore(host, panel.firstChild); }
  let foot = panel.querySelector('#acc-foot');
  if (!foot) { foot = document.createElement('div'); foot.id = 'acc-foot'; panel.appendChild(foot); }
  host.innerHTML = `<h2 class="admin-h">${t('adminTabAccount')}</h2>
    <div class="admin-form">
      <div class="admin-note">${esc(currentUser.username)} &middot; ${esc(t(currentUser.role) || currentUser.role)}</div>
      <div class="admin-row">
        <button class="btn btn-ghost btn-sm" onclick="navigateTo('/user/' + currentUser.id)">${t('accProfile')}</button>
        <button class="btn btn-ghost btn-sm" onclick="changeOwnPassword()">${t('changePassword')}</button>
      </div>
    </div>
    ${tg}
    <div class="admin-form" style="margin-top:20px">
      <h3 style="margin:0 0 8px">${t('accNotifTitle')}</h3>
      <label class="admin-fld" style="flex-direction:row;align-items:center;gap:8px">
        <input type="checkbox" id="acc-sound" ${(typeof localStorage !== 'undefined' && localStorage.getItem('chat-sound') === 'off') ? '' : 'checked'} onchange="accToggleSound(this.checked)" style="width:auto;margin:0"> ${t('accSound')}</label>
      <div class="admin-note">${t('accSoundHint')}</div>
    </div>
    <div id="acc-2fa" style="margin-top:20px"></div>
    <div id="acc-tokens" style="margin:20px 0"></div>`;
  foot.innerHTML = `<div class="admin-form" style="margin-top:20px">
      <div class="admin-row"><button class="btn btn-ghost btn-sm admin-danger" onclick="logout()">${t('logout')}</button></div>
    </div>`;
  renderAdmin2FA(host.querySelector('#acc-2fa'));
  renderAccountTokens(host.querySelector('#acc-tokens'));
}

// accToggleSound flips the global new-message sound preference (per-user,
// localStorage); per-channel muting stays in the chat header.
function accToggleSound(on) {
  try { localStorage.setItem('chat-sound', on ? 'on' : 'off'); } catch (e) { /* ignore */ }
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

// renderAccountTokens: the user's own API tokens (create/list/revoke) in the
// Account tab, next to 2FA. The secret is shown once, on creation.
function renderAccountTokens(panel) {
  if (!panel) return;
  const rank = { readonly: 1, regular: 2, admin: 3 };
  const mine = isAdmin() ? 3 : (rank[currentUser.role] || 2);
  const roles = ['readonly', 'regular', 'admin'].filter(r => rank[r] <= mine);
  const svcs = (typeof services !== 'undefined' ? services : []).filter(s => s.enabled).map(s => s.name);
  panel.innerHTML = `<h3 style="margin:0 0 8px">${t('tokTitle')}</h3>
    <div class="admin-note">${esc(t('tokHint'))}</div>
    <div class="admin-form">
      <label class="admin-fld">${t('tokName')}<input id="tok-name" maxlength="100"></label>
      <label class="admin-fld">${t('tokRole')}<select id="tok-role">${roles.map(r => `<option value="${r}">${esc(t(r))}</option>`).join('')}</select></label>
      <div class="admin-fld">${t('tokServices')}
        <div class="tok-svcs">${svcs.map(s => `<label><input type="checkbox" class="tok-svc" value="${escAttr(s)}"> ${esc(t('svc_' + s))}</label>`).join('')}</div>
        <div class="admin-note" style="padding:0">${t('tokSvcHint')}</div>
      </div>
      <label class="admin-fld">${t('tokExpires')}<select id="tok-exp">
        <option value="30">30 ${t('tokDays')}</option><option value="90" selected>90 ${t('tokDays')}</option>
        <option value="365">365 ${t('tokDays')}</option><option value="0">${t('tokNever')}</option></select></label>
      <div class="admin-row"><button class="btn btn-primary btn-sm" onclick="tokCreate()">${t('tokCreate')}</button></div>
      <div id="tok-new"></div>
    </div>
    <div id="tok-mine" class="audit-table" style="margin-top:16px"></div>`;
  tokLoad();
}

// renderAdminUsersTokens adds every user's API tokens (with revoke) below the
// relocated users section, without wiping it.
function renderAdminUsersTokens(panel) {
  if (!panel) return;
  let host = panel.querySelector('#users-tokens');
  if (!host) { host = document.createElement('div'); host.id = 'users-tokens'; host.style.marginTop = '24px'; panel.appendChild(host); }
  host.innerHTML = `<h3 style="margin:0 0 8px">${t('tokAllTitle')}</h3><div id="tok-all" class="audit-table"></div>`;
  tokLoad();
}

function tokTable(rows, withOwner, admin) {
  if (!rows.length) return `<div class="admin-empty">${t('tokNone')}</div>`;
  const now = Date.now() / 1000;
  const when = sec => sec ? fmtDate(new Date(sec * 1000).toISOString()) : '';
  return `<table class="audit-tbl"><thead><tr>
      ${withOwner ? `<th>${t('tokOwner')}</th>` : ''}<th>${t('tokName')}</th><th>${t('tokPrefix')}</th><th>${t('tokRole')}</th>
      <th>${t('tokServices')}</th><th>${t('tokExpires')}</th><th>${t('tokLastUsed')}</th>${withOwner ? '' : `<th>${t('tokCreated')}</th>`}<th></th>
    </tr></thead><tbody>${rows.map(r => `<tr>
      ${withOwner ? `<td>${esc(r.username)}</td>` : ''}
      <td>${esc(r.name)}</td><td><code>${esc(r.prefix)}…</code></td><td>${esc(t(r.role) || r.role)}</td>
      <td>${r.services.length ? r.services.map(s => esc(t('svc_' + s))).join(', ') : esc(t('tokAllServices'))}</td>
      <td>${!r.expires_at ? esc(t('tokNever')) : r.expires_at <= now ? `<span style="color:var(--red)">${esc(t('tokExpired'))}</span>` : esc(when(r.expires_at))}</td>
      <td>${esc(when(r.last_used_at))}</td>${withOwner ? '' : `<td>${esc(fmtDate(r.created_at))}</td>`}
      <td><button class="btn btn-ghost btn-sm admin-danger" onclick="tokRevoke(${r.id}, ${admin})">${t('tokRevoke')}</button></td>
    </tr>`).join('')}</tbody></table>`;
}

async function tokLoad() {
  const mine = document.getElementById('tok-mine');
  if (mine) {
    try { mine.innerHTML = tokTable(await api('/api/user/tokens'), false, false); }
    catch (e) { mine.innerHTML = `<div class="admin-empty">${esc(e.message)}</div>`; }
  }
  const all = document.getElementById('tok-all');
  if (all) {
    try { all.innerHTML = tokTable(await api('/api/admin/tokens'), true, true); }
    catch (e) { all.innerHTML = `<div class="admin-empty">${esc(e.message)}</div>`; }
  }
}

async function tokCreate() {
  const name = document.getElementById('tok-name').value.trim();
  if (!name) { toast(t('tokNameRequired'), 'error'); return; }
  const body = {
    name,
    role: document.getElementById('tok-role').value,
    services: [...document.querySelectorAll('.tok-svc:checked')].map(c => c.value),
    expires_days: parseInt(document.getElementById('tok-exp').value, 10) || 0,
  };
  let r;
  try { r = await api('/api/user/tokens', 'POST', body); } catch (e) { toast(e.message, 'error'); return; }
  document.getElementById('tok-name').value = '';
  document.getElementById('tok-new').innerHTML = `<p>${t('tokNew')}</p>
    <div class="admin-row" style="align-items:center;flex-wrap:wrap">
      <div class="twofa-secret" id="tok-secret" style="font-size:.85rem;letter-spacing:0;word-break:break-all">${esc(r.token)}</div>
      <button class="btn btn-ghost btn-sm" onclick="tokCopy()">${t('tokCopy')}</button></div>`;
  tokLoad();
}

async function tokCopy() {
  const el = document.getElementById('tok-secret');
  if (!el) return;
  try { await navigator.clipboard.writeText(el.textContent); toast(t('tokCopied'), 'success'); }
  catch (e) { window.getSelection().selectAllChildren(el); }
}

async function tokRevoke(id, admin) {
  if (!(await uiConfirm(t('tokRevokeConfirm')))) return;
  try { await api((admin ? '/api/admin/tokens/' : '/api/user/tokens/') + id, 'DELETE'); } catch (e) { toast(e.message, 'error'); return; }
  tokLoad();
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
let adminTab = 'account';

function ensureAdminPage() {
  if (adminBuilt) return;
  ensureAdminI18n();
  const page = document.createElement('div');
  page.id = 'admin-page';
  page.innerHTML = `<div class="admin-tabs" id="admin-tabs"></div><div class="admin-subtabs" id="admin-subtabs"></div><div class="admin-body" id="admin-body"></div>`;
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

function adminLeafVisible(key) {
  const td = ADMIN_TABS.find(t => t.key === key);
  return td ? adminTabVisible(td) : false;
}
function adminGroupChildren(entry) { return (entry.children || []).filter(adminLeafVisible); }
function adminTopLevelVisible() {
  return ADMIN_LAYOUT.filter(e => e.leaf ? adminLeafVisible(e.leaf) : adminGroupChildren(e).length > 0);
}
// Layout entry that owns the currently active leaf (leaf itself or its group).
function adminActiveTop() {
  return ADMIN_LAYOUT.find(e => e.leaf === adminTab || (e.children && e.children.includes(adminTab)));
}

function renderAdminTabs() {
  const bar = document.getElementById('admin-tabs');
  if (!bar) return;
  const active = adminActiveTop();
  bar.innerHTML = adminTopLevelVisible().map(e => {
    const key = e.leaf || e.group;
    const on = e === active;
    return `<button class="admin-tab ${on ? 'active' : ''}" onclick="adminSetTop('${key}')">${esc(t(e.i || ADMIN_TABS.find(t2 => t2.key === e.leaf).i))}</button>`;
  }).join('');
  // Secondary bar for the active group.
  const sub = document.getElementById('admin-subtabs');
  if (sub) {
    const kidsAll = active && active.group ? adminGroupChildren(active) : [];
    if (kidsAll.length > 1) {
      const kids = kidsAll;
      sub.innerHTML = kids.map(k => {
        const td = ADMIN_TABS.find(t2 => t2.key === k);
        return `<button class="admin-subtab ${adminTab === k ? 'active' : ''}" onclick="adminSetTab('${k}')">${esc(t(td.i))}</button>`;
      }).join('');
      sub.style.display = '';
    } else {
      sub.innerHTML = '';
      sub.style.display = 'none';
    }
  }
  document.querySelectorAll('#admin-page .admin-panel').forEach(p => p.classList.toggle('active', p.id === 'admin-panel-' + adminTab));
}

// adminSetTop handles a top-level click: a leaf selects directly; a group opens
// its first visible child.
function adminSetTop(key) {
  const entry = ADMIN_LAYOUT.find(e => e.leaf === key || e.group === key);
  if (!entry) return adminSetTab(key);
  if (entry.leaf) return adminSetTab(entry.leaf);
  const kids = adminGroupChildren(entry);
  if (kids.length) adminSetTab(kids[0]);
}

function adminSetTab(k) {
  adminTab = k;
  renderAdminTabs();
  const td = ADMIN_TABS.find(t => t.key === k);
  if (td && typeof td.render === 'function') td.render(document.getElementById('admin-panel-' + k));
  if (k === 'integrations') {
    if (typeof gitLoadRepos === 'function') gitLoadRepos();
    if (typeof callsLoadTurn === 'function') callsLoadTurn();
  }
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
  let wap = false;
  let uploadAllowed = '';
  try { const r = await api('/api/settings/app'); name = r.name || name; if (r.default_lang) defLang = r.default_lang; wap = !!r.wap_enabled; uploadAllowed = r.upload_allowed || ''; } catch (e) { /* ignore */ }
  const upAny = uploadAllowed.trim() === '*';
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
      <label class="admin-fld" style="flex-direction:row;align-items:center;gap:8px">
        <input type="checkbox" id="site-wap" ${wap ? 'checked' : ''} style="width:auto;margin:0"> ${t('adminWap')}</label>
      <label class="admin-fld" style="flex-direction:row;align-items:center;gap:8px">
        <input type="checkbox" id="site-upany" ${upAny ? 'checked' : ''} onchange="document.getElementById('site-upexts').disabled=this.checked" style="width:auto;margin:0"> ${t('adminUploadAny')}</label>
      <label class="admin-fld">${t('adminUploadExts')}
        <input id="site-upexts" value="${escAttr(upAny ? '' : uploadAllowed)}" ${upAny ? 'disabled' : ''} placeholder="pdf, png, docx, xlsx, zip"></label>
      <div class="admin-row"><button class="btn btn-primary btn-sm" onclick="adminSaveAppName()">${t('adminSave')}</button></div>
      <div class="admin-note">${t('adminAppNameHint')}</div>
      <div class="admin-note">${t('adminDefaultLangHint')}</div>
      <div class="admin-note">${t('adminWapHint')}</div>
      <div class="admin-note">${t('adminUploadHint')}</div>
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

// === Projects tab (admin moderation: create / edit / members / delete) ===
async function renderAdminProjects(panel) {
  if (!panel) return;
  if (typeof loadProjects === 'function') { try { await loadProjects(); } catch (e) { /* ignore */ } }
  const list = (typeof projects !== 'undefined' && projects) ? projects : [];
  const visLabel = v => v === 'open' ? t('projectOpen') : t('projectRestricted');
  const rows = list.map(p => `<tr>
      <td>${esc(p.name)}</td>
      <td>${esc(visLabel(p.visibility))}</td>
      <td style="text-align:right">${p.my_role === 'admin'
        ? `<button class="btn btn-ghost btn-sm" onclick="openProjectSettings(${p.id})">${t('adminProjManage')}</button>`
        : `<span class="admin-note">${esc(t('wikiRole_' + (p.my_role || '')) || p.my_role || '')}</span>`}</td>
    </tr>`).join('') || `<tr><td colspan="3"><div class="admin-note">${t('adminNoData')}</div></td></tr>`;
  panel.innerHTML = `<h2 class="admin-h">${t('adminTabProjects')}</h2>
    <div class="admin-form">
      <div class="admin-row"><button class="btn btn-primary btn-sm" onclick="newProjectDialog()">${t('adminProjNew')}</button></div>
      <div class="admin-note">${t('adminProjNoneSel')}</div>
    </div>
    <table class="admin-table"><thead><tr><th>${t('projectName')}</th><th>${t('adminProjVisibility')}</th><th></th></tr></thead>
    <tbody>${rows}</tbody></table>`;
}

async function adminSaveAppName() {
  const name = document.getElementById('site-appname').value.trim();
  const ls = document.getElementById('site-deflang');
  const lang = ls ? ls.value : '';
  const wapEl = document.getElementById('site-wap');
  const body = { name, lang };
  if (wapEl) body.wap = wapEl.checked;
  const upAnyEl = document.getElementById('site-upany');
  const upExtsEl = document.getElementById('site-upexts');
  if (upAnyEl) body.upload_allowed = upAnyEl.checked ? '*' : (upExtsEl ? upExtsEl.value.trim() : '');
  try {
    const r = await api('/api/settings/app', 'POST', body);
    if (r.default_lang) window.DEFAULT_LANG = r.default_lang;
    if (typeof applyAppName === 'function') applyAppName(r.name || name || 'Madok.Works');
    if (typeof toast === 'function') toast(t('settingsSaved') || 'OK', 'success');
  } catch (e) { alert(e.message); }
}

// === Git integration (Settings → Integrations) ===
async function gitLoadRepos() {
  const el = document.getElementById('git-repos-list');
  if (!el) return;
  const pid = (typeof currentProjectId !== 'undefined' && currentProjectId) || 1;
  let repos = [];
  try { repos = await api('/api/integrations/git/repos?project=' + pid); } catch (e) { el.innerHTML = ''; return; }
  const origin = location.origin;
  el.innerHTML = repos.map(r => `<div class="git-repo-row">
      <div><b>${esc(r.label || r.repo || r.kind)}</b> <span class="admin-note">${esc(r.kind)}${r.repo ? ' · ' + esc(r.repo) : ''}</span></div>
      <div class="admin-note">${t('gitWebhookUrl')}: <code>${origin}/api/integrations/git/webhook/${r.id}</code></div>
      <div class="admin-row">
        <button class="btn btn-ghost btn-sm" onclick="gitEditRepo(${r.id})">${t('edit')}</button>
        <button class="btn btn-ghost btn-sm admin-danger" onclick="gitDeleteRepo(${r.id})">${t('delete')}</button>
      </div>
    </div>`).join('') || `<div class="admin-note">${t('adminNoData')}</div>`;
}

function gitRepoModal(repo) {
  const pid = (typeof currentProjectId !== 'undefined' && currentProjectId) || 1;
  const cols = (typeof state !== 'undefined' && state.columns) ? state.columns : [];
  const kinds = ['github', 'gitlab', 'gitea'];
  const kindOpts = kinds.map(k => `<option value="${k}" ${repo && repo.kind === k ? 'selected' : ''}>${k}</option>`).join('');
  const colOpts = `<option value="">—</option>` + cols.map(c => `<option value="${c.id}" ${repo && repo.move_column_id === c.id ? 'selected' : ''}>${esc(c.name)}</option>`).join('');
  const ov = document.createElement('div');
  ov.className = 'modal-overlay';
  ov.innerHTML = `<div class="modal"><div class="modal-header"><h2>${t('gitTitle')}</h2>
      <button class="btn-icon" onclick="this.closest('.modal-overlay').remove()">&#10005;</button></div>
    <div class="modal-body">
      <div class="field"><label>${t('gitKind')}</label><select id="git-kind">${kindOpts}</select></div>
      <div class="field"><label>${t('gitLabel')}</label><input id="git-label" maxlength="60" value="${repo ? escAttr(repo.label) : ''}"></div>
      <div class="field"><label>${t('gitRepo')}</label><input id="git-repo" maxlength="200" placeholder="owner/name" value="${repo ? escAttr(repo.repo) : ''}"></div>
      <div class="field"><label>${t('gitSecret')}</label><input id="git-secret" maxlength="200" placeholder="${repo && repo.secret === 'set' ? '••••••' : ''}" value=""></div>
      <div class="field"><label>${t('gitMoveCol')}</label><select id="git-move">${colOpts}</select></div>
    </div>
    <div class="modal-footer">
      <button class="btn btn-ghost" onclick="this.closest('.modal-overlay').remove()">${t('cancel')}</button>
      <button class="btn btn-primary" onclick="gitSaveRepo(${repo ? repo.id : 0})">${t('adminSave')}</button>
    </div></div>`;
  document.body.appendChild(ov);
}

function gitAddRepo() { gitRepoModal(null); }
async function gitEditRepo(id) {
  const pid = (typeof currentProjectId !== 'undefined' && currentProjectId) || 1;
  let repos = [];
  try { repos = await api('/api/integrations/git/repos?project=' + pid); } catch (e) { return; }
  const repo = repos.find(r => r.id === id);
  if (repo) gitRepoModal(repo);
}
async function gitSaveRepo(id) {
  const pid = (typeof currentProjectId !== 'undefined' && currentProjectId) || 1;
  const mv = document.getElementById('git-move').value;
  const body = {
    kind: document.getElementById('git-kind').value,
    label: document.getElementById('git-label').value.trim(),
    repo: document.getElementById('git-repo').value.trim(),
    secret: document.getElementById('git-secret').value,
    move_column_id: mv ? Number(mv) : null,
  };
  // Empty secret on edit → keep current (server treats "set" sentinel specially).
  if (id && body.secret === '') body.secret = 'set';
  try {
    if (id) await api('/api/integrations/git/repos/' + id, 'PUT', body);
    else await api('/api/integrations/git/repos?project=' + pid, 'POST', body);
    document.querySelectorAll('.modal-overlay').forEach(o => o.remove());
    gitLoadRepos();
  } catch (e) { alert(e.message); }
}
async function gitDeleteRepo(id) {
  if (!await uiConfirm(t('delete') + '?')) return;
  try { await api('/api/integrations/git/repos/' + id, 'DELETE'); gitLoadRepos(); } catch (e) { alert(e.message); }
}

// === Calls: TURN server config ===
async function callsLoadTurn() {
  const el = document.getElementById('calls-turn-form');
  if (!el) return;
  let c = {};
  try { c = await api('/api/calls/turn'); } catch (e) { el.innerHTML = ''; return; }
  el.innerHTML = `
    <label class="admin-fld">${t('callsTurnUrl')}<input id="turn-url" maxlength="200" placeholder="turn:host:3478" value="${escAttr(c.turn_url || '')}"></label>
    <label class="admin-fld">${t('callsTurnUser')}<input id="turn-user" maxlength="100" value="${escAttr(c.turn_user || '')}"></label>
    <label class="admin-fld">${t('callsTurnCred')}<input id="turn-cred" maxlength="200" value="${escAttr(c.turn_cred || '')}"></label>
    <div class="admin-row"><button class="btn btn-primary btn-sm" onclick="callsSaveTurn()">${t('adminSave')}</button></div>`;
}
async function callsSaveTurn() {
  const body = {
    url: document.getElementById('turn-url').value.trim(),
    user: document.getElementById('turn-user').value.trim(),
    cred: document.getElementById('turn-cred').value.trim(),
  };
  try { await api('/api/calls/turn', 'POST', body); if (typeof toast === 'function') toast(t('settingsSaved') || 'OK', 'success'); }
  catch (e) { alert(e.message); }
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
