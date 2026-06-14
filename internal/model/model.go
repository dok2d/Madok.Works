package model

import "time"

type Column struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

type Epic struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

type Tag struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Sprint struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	StartDate string `json:"start_date"` // ISO date YYYY-MM-DD
	EndDate   string `json:"end_date"`   // ISO date YYYY-MM-DD
	Status    string `json:"status"`     // "planning", "active", "completed"
}

type TaskDep struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	ColumnID int64  `json:"column_id"`
}

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Todo        string    `json:"todo"`
	ProjectURL  string    `json:"project_url"`
	ColumnID    int64     `json:"column_id"`
	EpicID      *int64    `json:"epic_id"`
	SprintID    *int64    `json:"sprint_id"`
	AssigneeID  *int64    `json:"assignee_id"`
	Position    int       `json:"position"`
	Priority    int       `json:"priority"` // 0=none,1=low,2=med,3=high,4=critical
	Deadline    string    `json:"deadline"` // ISO datetime or empty
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tags        []Tag     `json:"tags"`
	Epic        *Epic     `json:"epic,omitempty"`
	Sprint      *Sprint   `json:"sprint,omitempty"`
	Assignee    *User     `json:"assignee,omitempty"`
	Comments    []Comment `json:"comments,omitempty"`
	DependsOn   []TaskDep `json:"depends_on,omitempty"`
	Dependents  []TaskDep `json:"dependents,omitempty"`
}

type Comment struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	ParentID  *int64    `json:"parent_id"`
	AuthorID  *int64    `json:"author_id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Author    *User     `json:"author,omitempty"`
	Replies   []Comment `json:"replies,omitempty"`
}

// Attachment stored in the files table
type Attachment struct {
	ID       int64  `json:"id"`
	Filename string `json:"filename"`
	Mime     string `json:"mime"`
	Size     int    `json:"size"`
	URL      string `json:"url"`
}

// User represents an authenticated user
type User struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	Role       string `json:"role"` // "admin", "regular", "readonly"
	IsAdmin    bool   `json:"is_admin"`
	CreatedAt  string `json:"created_at"`
	TelegramID int64  `json:"telegram_id,omitempty"`
	LinkHash   string `json:"link_hash,omitempty"`
}

// Session represents an active user session
type Session struct {
	Token     string
	UserID    int64
	ExpiresAt string
}

// Notification for user
type Notification struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Type      string    `json:"type"` // "mention", "assigned", "subscribed", "comment"
	Text      string    `json:"text"`
	TaskID    *int64    `json:"task_id,omitempty"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

// TaskSubscription tracks who is subscribed to task updates
type TaskSubscription struct {
	TaskID int64 `json:"task_id"`
	UserID int64 `json:"user_id"`
}

// ActivityEntry represents a user activity log entry
type ActivityEntry struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Action    string    `json:"action"` // "create_task", "edit_task", "comment", "move_task", "assign", etc.
	TaskID    *int64    `json:"task_id,omitempty"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

// === Wiki (ecosystem service) ===

// WikiSpace is a documentation space — a tree of pages with its own access list.
type WikiSpace struct {
	ID          int64  `json:"id"`
	Key         string `json:"key"` // short uppercase identifier, e.g. "DOCS"
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"` // "open" (all users) or "restricted" (members only)
	CreatedBy   *int64 `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	PageCount   int    `json:"page_count"`
	MyRole      string `json:"my_role"` // effective role of the requesting user: "admin", "editor", "viewer", ""
}

// WikiSpaceMember is an entry in a restricted space's access list.
type WikiSpaceMember struct {
	SpaceID  int64  `json:"space_id"`
	UserID   int64  `json:"user_id"`
	Role     string `json:"role"` // "admin", "editor", "viewer"
	Username string `json:"username"`
}

// WikiPage holds the current revision of a page.
type WikiPage struct {
	ID            int64  `json:"id"`
	SpaceID       int64  `json:"space_id"`
	ParentID      *int64 `json:"parent_id"`
	Title         string `json:"title"`
	Content       string `json:"content,omitempty"` // sanitized HTML
	Position      int    `json:"position"`
	RevNo         int    `json:"rev_no"`
	CreatedBy     *int64 `json:"created_by"`
	CreatedAt     string `json:"created_at"`
	UpdatedBy     *int64 `json:"updated_by"`
	UpdatedAt     string `json:"updated_at"`
	CreatedByName string `json:"created_by_name,omitempty"`
	UpdatedByName string `json:"updated_by_name,omitempty"`
}

// WikiRevision is a historical version of a page (content loaded on demand).
type WikiRevision struct {
	ID         int64  `json:"id"`
	PageID     int64  `json:"page_id"`
	RevNo      int    `json:"rev_no"`
	Title      string `json:"title"`
	Content    string `json:"content,omitempty"`
	AuthorID   *int64 `json:"author_id"`
	AuthorName string `json:"author_name,omitempty"`
	Comment    string `json:"comment"`
	CreatedAt  string `json:"created_at"`
}

// WikiSearchHit is one result of a wiki full-text search.
type WikiSearchHit struct {
	PageID   int64  `json:"page_id"`
	SpaceID  int64  `json:"space_id"`
	SpaceKey string `json:"space_key"`
	Title    string `json:"title"`
	Snippet  string `json:"snippet"`
}

// === Chat (ecosystem service) ===

// ChatChannel is a public channel, private group or direct-message pair.
type ChatChannel struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Topic       string `json:"topic"`
	Type        string `json:"type"` // "channel" (public), "group" (private), "dm"
	CreatedBy   *int64 `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	MemberCount int    `json:"member_count"`
	Unread      int    `json:"unread"`
	LastMsgAt   string `json:"last_msg_at,omitempty"`
	IsMember    bool   `json:"is_member"`
	// For DM channels: the other participant
	PeerID   *int64 `json:"peer_id,omitempty"`
	PeerName string `json:"peer_name,omitempty"`
}

// ChatAttachment links an uploaded file to a chat message.
type ChatAttachment struct {
	ID       int64  `json:"id"`
	FileID   int64  `json:"file_id"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

// ChatMessage is one message in a channel.
type ChatMessage struct {
	ID          int64            `json:"id"`
	ChannelID   int64            `json:"channel_id"`
	AuthorID    *int64           `json:"author_id"`
	AuthorName  string           `json:"author_name"`
	Text        string           `json:"text"`
	CreatedAt   string           `json:"created_at"`
	EditedAt    string           `json:"edited_at,omitempty"`
	Deleted     bool             `json:"deleted"`
	Attachments []ChatAttachment `json:"attachments,omitempty"`
}

// ChatSearchHit is one full-text search result across visible channels.
type ChatSearchHit struct {
	MessageID   int64  `json:"message_id"`
	ChannelID   int64  `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	AuthorName  string `json:"author_name"`
	Text        string `json:"text"`
	CreatedAt   string `json:"created_at"`
}

// === Calendar (ecosystem service) ===

// Calendar groups events. "private" calendars are visible only to the owner;
// "shared" calendars are visible to all users (editable by owner/admin).
type Calendar struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	OwnerID    *int64 `json:"owner_id"`
	Visibility string `json:"visibility"` // "private" | "shared"
	CreatedAt  string `json:"created_at"`
	OwnerName  string `json:"owner_name,omitempty"`
	CanEdit    bool   `json:"can_edit"`
}

// CalendarEvent is one event or, when expanded from an RRULE, one occurrence.
// Times are stored and transported in UTC ("YYYY-MM-DD HH:MM:SS"); the client
// renders them in the user's timezone.
type CalendarEvent struct {
	ID          int64  `json:"id"`
	CalendarID  int64  `json:"calendar_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Start       string `json:"start"` // UTC
	End         string `json:"end"`   // UTC
	AllDay      bool   `json:"all_day"`
	RRule       string `json:"rrule,omitempty"` // e.g. "FREQ=WEEKLY;INTERVAL=1;COUNT=10"
	CreatedBy   *int64 `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	// Rendering helpers / virtual events from other services
	Color      string `json:"color,omitempty"`
	CalName    string `json:"calendar_name,omitempty"`
	Source     string `json:"source,omitempty"`     // "" (real) | "task" | "sprint"
	RefID      int64  `json:"ref_id,omitempty"`     // task/sprint id for virtual events
	Occurrence string `json:"occurrence,omitempty"` // set on expanded recurring instances
}

// === Mail (ecosystem service) ===

// MailAttachment links an uploaded file to a mail message.
type MailAttachment struct {
	ID       int64  `json:"id"`
	FileID   int64  `json:"file_id"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

// MailMessage is one email. When listed from a mailbox, the per-user box fields
// (BoxID/Folder/Seen/Flagged) are populated.
type MailMessage struct {
	ID          int64            `json:"id"`
	ThreadID    int64            `json:"thread_id"`
	FromUserID  *int64           `json:"from_user_id"`
	FromName    string           `json:"from_name"`
	FromAddr    string           `json:"from_addr"`
	ToAddrs     string           `json:"to_addrs"`
	CcAddrs     string           `json:"cc_addrs"`
	Subject     string           `json:"subject"`
	BodyText    string           `json:"body_text,omitempty"`
	BodyHTML    string           `json:"body_html,omitempty"`
	InReplyTo   string           `json:"in_reply_to,omitempty"`
	CreatedAt   string           `json:"created_at"`
	Attachments []MailAttachment `json:"attachments,omitempty"`
	// per-mailbox fields
	BoxID   int64  `json:"box_id,omitempty"`
	Folder  string `json:"folder,omitempty"`
	Seen    bool   `json:"seen"`
	Flagged bool   `json:"flagged"`
	Snippet string `json:"snippet,omitempty"`
}

// MailFolderCount is unread/total per folder for the sidebar.
type MailFolderCount struct {
	Folder string `json:"folder"`
	Unread int    `json:"unread"`
	Total  int    `json:"total"`
}

// MailFolder is a user-defined custom folder.
type MailFolder struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// MailRuleCondition / MailRuleAction / MailRule define a user's inbound filter.
type MailRuleCondition struct {
	Field string `json:"field"` // from | to | subject | body
	Op    string `json:"op"`    // contains | equals | starts_with | regex
	Value string `json:"value"`
}

type MailRuleAction struct {
	Type string `json:"type"` // move_to_folder | flag | mark_read | delete
	Arg  string `json:"arg"`
}

type MailRule struct {
	ID         int64               `json:"id"`
	UserID     int64               `json:"user_id"`
	Name       string              `json:"name"`
	Enabled    bool                `json:"enabled"`
	MatchType  string              `json:"match_type"` // all | any
	Conditions []MailRuleCondition `json:"conditions"`
	Actions    []MailRuleAction    `json:"actions"`
	Position   int                 `json:"position"`
}

// MailAudit is one security/attack-investigation event for the mail subsystem.
type MailAudit struct {
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	Event     string `json:"event"` // inbound_accept|inbound_reject|spam|rule_trigger|mass_send|relay_attempt
	SourceIP  string `json:"source_ip"`
	MailFrom  string `json:"mail_from"`
	Rcpt      string `json:"rcpt"`
	Subject   string `json:"subject"`
	UserID    *int64 `json:"user_id"`
	Score     int    `json:"score"`
	Detail    string `json:"detail"`
}

// ExportUser contains user data for export (password hash omitted by default for security)
type ExportUser struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash,omitempty"`
	Role         string `json:"role"`
	IsAdmin      bool   `json:"is_admin"`
	TelegramID   int64  `json:"telegram_id,omitempty"`
}

// ExportSetting stores app settings for export
type ExportSetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ExportDependency stores task dependency for export
type ExportDependency struct {
	TaskID      int64 `json:"task_id"`
	DependsOnID int64 `json:"depends_on_id"`
}

// ExportSubscription stores task subscription for export
type ExportSubscription struct {
	TaskID int64 `json:"task_id"`
	UserID int64 `json:"user_id"`
}

// ExportFile stores file/image data for export
type ExportFile struct {
	ID       int64  `json:"id"`
	Filename string `json:"filename"`
	Mime     string `json:"mime"`
	Data     []byte `json:"data"`
}

// ExportData is the full board export structure
type ExportData struct {
	Columns       []Column             `json:"columns"`
	Epics         []Epic               `json:"epics"`
	Sprints       []Sprint             `json:"sprints,omitempty"`
	Tags          []Tag                `json:"tags"`
	Tasks         []Task               `json:"tasks"`
	Comments      []Comment            `json:"comments"`
	Users         []ExportUser         `json:"users,omitempty"`
	Settings      []ExportSetting      `json:"settings,omitempty"`
	Dependencies  []ExportDependency   `json:"dependencies,omitempty"`
	Subscriptions []ExportSubscription `json:"subscriptions,omitempty"`
	Files         []ExportFile         `json:"files,omitempty"`
	Images        []ExportFile         `json:"images,omitempty"`
	// Ecosystem holds generic per-table row dumps for the wiki/chat/calendar
	// services (keyed by table name) so JSON export/import is lossless for them.
	Ecosystem map[string][]map[string]any `json:"ecosystem,omitempty"`
}
