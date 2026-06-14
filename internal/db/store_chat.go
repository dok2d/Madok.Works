package db

import (
	"database/sql"
	"fmt"
	"strings"

	"kanban/internal/model"
)

// === Channels ===

// ListChatChannels returns channels visible to the user: all public channels
// plus private groups and DMs the user belongs to. Unread counts are computed
// from chat_members.last_read_id (0 for public channels never joined).
func (s *Store) ListChatChannels(userID int64) ([]model.ChatChannel, error) {
	rows, err := s.db.Query(`SELECT c.id, c.name, c.topic, c.type, c.created_by, c.created_at,
		(SELECT COUNT(*) FROM chat_members m2 WHERE m2.channel_id = c.id),
		COALESCE(m.last_read_id, -1),
		COALESCE((SELECT MAX(id) FROM chat_messages msg WHERE msg.channel_id = c.id AND msg.deleted = 0), 0),
		COALESCE((SELECT MAX(created_at) FROM chat_messages msg WHERE msg.channel_id = c.id AND msg.deleted = 0), '')
		FROM chat_channels c
		LEFT JOIN chat_members m ON m.channel_id = c.id AND m.user_id = ?
		WHERE c.type = 'channel' OR m.user_id IS NOT NULL
		ORDER BY c.type, c.name COLLATE NOCASE`, userID)
	if err != nil {
		return nil, err
	}
	// With SetMaxOpenConns(1) the open cursor holds the only connection, so
	// read everything first and only then run follow-up queries.
	var out []model.ChatChannel
	var lastReads []int64
	for rows.Next() {
		var c model.ChatChannel
		var lastRead, maxID int64
		if err := rows.Scan(&c.ID, &c.Name, &c.Topic, &c.Type, &c.CreatedBy, &c.CreatedAt,
			&c.MemberCount, &lastRead, &maxID, &c.LastMsgAt); err != nil {
			rows.Close()
			return nil, err
		}
		c.IsMember = lastRead >= 0
		out = append(out, c)
		lastReads = append(lastReads, lastRead)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	for i := range out {
		if out[i].IsMember {
			var unread int
			s.db.QueryRow(`SELECT COUNT(*) FROM chat_messages WHERE channel_id=? AND id>? AND deleted=0 AND COALESCE(author_id,0)!=?`,
				out[i].ID, lastReads[i], userID).Scan(&unread)
			out[i].Unread = unread
		}
		if out[i].Type == "dm" {
			var peerID int64
			var peerName string
			err := s.db.QueryRow(`SELECT u.id, u.username FROM chat_members m JOIN users u ON u.id = m.user_id
				WHERE m.channel_id = ? AND m.user_id != ? LIMIT 1`, out[i].ID, userID).Scan(&peerID, &peerName)
			if err == nil {
				out[i].PeerID = &peerID
				out[i].PeerName = peerName
			}
		}
	}
	if out == nil {
		out = []model.ChatChannel{}
	}
	return out, nil
}

func (s *Store) GetChatChannel(id int64) (*model.ChatChannel, error) {
	var c model.ChatChannel
	err := s.db.QueryRow(`SELECT id, name, topic, type, created_by, created_at,
		(SELECT COUNT(*) FROM chat_members m WHERE m.channel_id = chat_channels.id)
		FROM chat_channels WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Topic, &c.Type, &c.CreatedBy, &c.CreatedAt, &c.MemberCount)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// IsChatMember reports membership; public channels are open to everyone.
func (s *Store) IsChatMember(channelID, userID int64) (bool, error) {
	var cnt int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM chat_members WHERE channel_id=? AND user_id=?`, channelID, userID).Scan(&cnt)
	return cnt > 0, err
}

func (s *Store) CreateChatChannel(name, topic, chType string, createdBy int64, memberIDs []int64) (int64, error) {
	switch chType {
	case "channel", "group":
	default:
		return 0, fmt.Errorf("invalid channel type %q", chType)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("name required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO chat_channels(name, topic, type, created_by) VALUES(?,?,?,?)`,
		name, topic, chType, createdBy)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	seen := map[int64]bool{createdBy: true}
	if _, err := tx.Exec(`INSERT INTO chat_members(channel_id, user_id) VALUES(?,?)`, id, createdBy); err != nil {
		return 0, err
	}
	for _, uid := range memberIDs {
		if seen[uid] {
			continue
		}
		seen[uid] = true
		if _, err := tx.Exec(`INSERT OR IGNORE INTO chat_members(channel_id, user_id) VALUES(?,?)`, id, uid); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// GetOrCreateDM returns the DM channel between two users, creating it on
// first use.
func (s *Store) GetOrCreateDM(userA, userB int64) (int64, error) {
	if userA == userB {
		return 0, fmt.Errorf("cannot DM yourself")
	}
	var id int64
	err := s.db.QueryRow(`SELECT c.id FROM chat_channels c
		JOIN chat_members m1 ON m1.channel_id = c.id AND m1.user_id = ?
		JOIN chat_members m2 ON m2.channel_id = c.id AND m2.user_id = ?
		WHERE c.type = 'dm' LIMIT 1`, userA, userB).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO chat_channels(name, type, created_by) VALUES('', 'dm', ?)`, userA)
	if err != nil {
		return 0, err
	}
	id, _ = res.LastInsertId()
	if _, err := tx.Exec(`INSERT INTO chat_members(channel_id, user_id) VALUES(?,?),(?,?)`, id, userA, id, userB); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (s *Store) UpdateChatChannel(id int64, name, topic string) error {
	_, err := s.db.Exec(`UPDATE chat_channels SET name=?, topic=? WHERE id=?`, strings.TrimSpace(name), topic, id)
	return err
}

func (s *Store) DeleteChatChannel(id int64) error {
	_, err := s.db.Exec(`DELETE FROM chat_channels WHERE id=?`, id)
	return err
}

// === Membership ===

func (s *Store) ListChatMembers(channelID int64) ([]model.WikiSpaceMember, error) {
	// reuse the simple member struct shape (space_id field carries channel id)
	rows, err := s.db.Query(`SELECT m.channel_id, m.user_id, '', u.username
		FROM chat_members m JOIN users u ON u.id = m.user_id
		WHERE m.channel_id = ? ORDER BY u.username COLLATE NOCASE`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.WikiSpaceMember
	for rows.Next() {
		var m model.WikiSpaceMember
		if err := rows.Scan(&m.SpaceID, &m.UserID, &m.Role, &m.Username); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if out == nil {
		out = []model.WikiSpaceMember{}
	}
	return out, rows.Err()
}

func (s *Store) AddChatMember(channelID, userID int64) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO chat_members(channel_id, user_id) VALUES(?,?)`, channelID, userID)
	return err
}

func (s *Store) RemoveChatMember(channelID, userID int64) error {
	_, err := s.db.Exec(`DELETE FROM chat_members WHERE channel_id=? AND user_id=?`, channelID, userID)
	return err
}

// MarkChatRead stores the highest message id the user has seen, joining the
// channel implicitly for public channels.
func (s *Store) MarkChatRead(channelID, userID, lastID int64) error {
	_, err := s.db.Exec(`INSERT INTO chat_members(channel_id, user_id, last_read_id) VALUES(?,?,?)
		ON CONFLICT(channel_id, user_id) DO UPDATE SET last_read_id = MAX(last_read_id, excluded.last_read_id)`,
		channelID, userID, lastID)
	return err
}

// === Messages ===

// ListChatMessages returns up to limit messages. With beforeID > 0 it pages
// backwards (older); with afterID > 0 it returns newer messages (polling/SSE
// catch-up). Results are always in ascending id order.
func (s *Store) ListChatMessages(channelID int64, beforeID, afterID int64, limit int) ([]model.ChatMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	switch {
	case afterID > 0:
		rows, err = s.db.Query(`SELECT m.id, m.channel_id, m.author_id, COALESCE(u.username,''), m.text, m.created_at,
			COALESCE(m.edited_at,''), m.deleted
			FROM chat_messages m LEFT JOIN users u ON u.id = m.author_id
			WHERE m.channel_id = ? AND m.id > ? ORDER BY m.id LIMIT ?`, channelID, afterID, limit)
	case beforeID > 0:
		rows, err = s.db.Query(`SELECT * FROM (
			SELECT m.id, m.channel_id, m.author_id, COALESCE(u.username,''), m.text, m.created_at,
			COALESCE(m.edited_at,''), m.deleted
			FROM chat_messages m LEFT JOIN users u ON u.id = m.author_id
			WHERE m.channel_id = ? AND m.id < ? ORDER BY m.id DESC LIMIT ?
		) ORDER BY id`, channelID, beforeID, limit)
	default:
		rows, err = s.db.Query(`SELECT * FROM (
			SELECT m.id, m.channel_id, m.author_id, COALESCE(u.username,''), m.text, m.created_at,
			COALESCE(m.edited_at,''), m.deleted
			FROM chat_messages m LEFT JOIN users u ON u.id = m.author_id
			WHERE m.channel_id = ? ORDER BY m.id DESC LIMIT ?
		) ORDER BY id`, channelID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ChatMessage
	for rows.Next() {
		var m model.ChatMessage
		var deleted int
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.AuthorID, &m.AuthorName, &m.Text, &m.CreatedAt, &m.EditedAt, &deleted); err != nil {
			return nil, err
		}
		m.Deleted = deleted == 1
		if m.Deleted {
			m.Text = ""
		}
		out = append(out, m)
	}
	if out == nil {
		out = []model.ChatMessage{}
	}
	return out, rows.Err()
}

func (s *Store) GetChatMessage(id int64) (*model.ChatMessage, error) {
	var m model.ChatMessage
	var deleted int
	err := s.db.QueryRow(`SELECT m.id, m.channel_id, m.author_id, COALESCE(u.username,''), m.text, m.created_at,
		COALESCE(m.edited_at,''), m.deleted
		FROM chat_messages m LEFT JOIN users u ON u.id = m.author_id WHERE m.id = ?`, id).
		Scan(&m.ID, &m.ChannelID, &m.AuthorID, &m.AuthorName, &m.Text, &m.CreatedAt, &m.EditedAt, &deleted)
	if err != nil {
		return nil, err
	}
	m.Deleted = deleted == 1
	return &m, nil
}

func (s *Store) CreateChatMessage(channelID, authorID int64, text string) (*model.ChatMessage, error) {
	res, err := s.db.Exec(`INSERT INTO chat_messages(channel_id, author_id, text) VALUES(?,?,?)`,
		channelID, authorID, text)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	// author has obviously read their own message
	s.MarkChatRead(channelID, authorID, id)
	return s.GetChatMessage(id)
}

func (s *Store) UpdateChatMessage(id int64, text string) (*model.ChatMessage, error) {
	_, err := s.db.Exec(`UPDATE chat_messages SET text=?, edited_at=CURRENT_TIMESTAMP WHERE id=? AND deleted=0`, text, id)
	if err != nil {
		return nil, err
	}
	return s.GetChatMessage(id)
}

// DeleteChatMessage soft-deletes so the timeline keeps its shape.
func (s *Store) DeleteChatMessage(id int64) error {
	_, err := s.db.Exec(`UPDATE chat_messages SET deleted=1, text='' WHERE id=?`, id)
	return err
}

// === Search ===

func (s *Store) SearchChatMessages(userID int64, query string, limit int) ([]model.ChatSearchHit, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	like := "%" + escapeLike(query) + "%"
	rows, err := s.db.Query(`SELECT m.id, m.channel_id,
		CASE WHEN c.type='dm' THEN COALESCE((SELECT u2.username FROM chat_members mm JOIN users u2 ON u2.id=mm.user_id
			WHERE mm.channel_id=c.id AND mm.user_id != ? LIMIT 1), 'DM') ELSE c.name END,
		COALESCE(u.username,''), m.text, m.created_at
		FROM chat_messages m
		JOIN chat_channels c ON c.id = m.channel_id
		LEFT JOIN users u ON u.id = m.author_id
		WHERE m.deleted = 0 AND m.text LIKE ? ESCAPE '\'
		AND (c.type = 'channel' OR EXISTS(SELECT 1 FROM chat_members mb WHERE mb.channel_id = c.id AND mb.user_id = ?))
		ORDER BY m.id DESC LIMIT ?`, userID, like, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ChatSearchHit
	for rows.Next() {
		var h model.ChatSearchHit
		if err := rows.Scan(&h.MessageID, &h.ChannelID, &h.ChannelName, &h.AuthorName, &h.Text, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	if out == nil {
		out = []model.ChatSearchHit{}
	}
	return out, rows.Err()
}

// TotalChatUnread sums unread counts across the user's channels (for the rail badge).
func (s *Store) TotalChatUnread(userID int64) int {
	var total int
	s.db.QueryRow(`SELECT COALESCE(SUM(cnt),0) FROM (
		SELECT (SELECT COUNT(*) FROM chat_messages msg
			WHERE msg.channel_id = m.channel_id AND msg.id > m.last_read_id AND msg.deleted = 0
			AND COALESCE(msg.author_id,0) != m.user_id) AS cnt
		FROM chat_members m WHERE m.user_id = ?)`, userID).Scan(&total)
	return total
}
