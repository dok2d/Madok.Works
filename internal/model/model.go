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
	Deadline    string    `json:"deadline"`  // ISO datetime or empty
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
	Key         string `json:"key"`  // short uppercase identifier, e.g. "DOCS"
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

// ChatMessage is one message in a channel.
type ChatMessage struct {
	ID         int64  `json:"id"`
	ChannelID  int64  `json:"channel_id"`
	AuthorID   *int64 `json:"author_id"`
	AuthorName string `json:"author_name"`
	Text       string `json:"text"`
	CreatedAt  string `json:"created_at"`
	EditedAt   string `json:"edited_at,omitempty"`
	Deleted    bool   `json:"deleted"`
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
}
