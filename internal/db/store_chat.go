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
		COALESCE((SELECT MAX(created_at) FROM chat_messages msg WHERE msg.channel_id = c.id AND msg.deleted = 0), ''),
		COALESCE((SELECT msg.text FROM chat_messages msg WHERE msg.channel_id = c.id AND msg.deleted = 0 ORDER BY msg.id DESC LIMIT 1), ''),
		COALESCE((SELECT COALESCE(u2.username,'') FROM chat_messages msg LEFT JOIN users u2 ON u2.id = msg.author_id WHERE msg.channel_id = c.id AND msg.deleted = 0 ORDER BY msg.id DESC LIMIT 1), '')
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
			&c.MemberCount, &lastRead, &maxID, &c.LastMsgAt, &c.LastMsgText, &c.LastMsgFrom); err != nil {
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
			var peerID, peerLastRead int64
			var peerName string
			err := s.db.QueryRow(`SELECT u.id, u.username, COALESCE(m.last_read_id,0) FROM chat_members m JOIN users u ON u.id = m.user_id
				WHERE m.channel_id = ? AND m.user_id != ? LIMIT 1`, out[i].ID, userID).Scan(&peerID, &peerName, &peerLastRead)
			if err == nil {
				out[i].PeerID = &peerID
				out[i].PeerName = peerName
				out[i].PeerLastRead = peerLastRead
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

// GetChatChannelFor returns a channel with viewer-relative DM fields populated
// (peer id/name and the peer's last_read_id, for read ticks in the open DM).
func (s *Store) GetChatChannelFor(id, viewerID int64) (*model.ChatChannel, error) {
	c, err := s.GetChatChannel(id)
	if err != nil {
		return nil, err
	}
	if c.Type == "dm" {
		var peerID, peerLastRead int64
		var peerName string
		if e := s.db.QueryRow(`SELECT u.id, u.username, COALESCE(m.last_read_id,0) FROM chat_members m JOIN users u ON u.id = m.user_id
			WHERE m.channel_id = ? AND m.user_id != ? LIMIT 1`, id, viewerID).Scan(&peerID, &peerName, &peerLastRead); e == nil {
			c.PeerID = &peerID
			c.PeerName = peerName
			c.PeerLastRead = peerLastRead
		}
	}
	return c, nil
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
// chatMsgCols is the shared projection for message rows (incl. reply/pin state).
const chatMsgCols = `m.id, m.channel_id, m.author_id, COALESCE(u.username,''), m.text, m.created_at,
	COALESCE(m.edited_at,''), m.deleted, COALESCE(m.reply_to,0), m.pinned, COALESCE(m.pinned_at,'')`

// scanChatMsg scans a row produced by chatMsgCols.
func scanChatMsg(sc interface{ Scan(...any) error }) (model.ChatMessage, error) {
	var m model.ChatMessage
	var deleted, pinned int
	var replyTo int64
	if err := sc.Scan(&m.ID, &m.ChannelID, &m.AuthorID, &m.AuthorName, &m.Text, &m.CreatedAt,
		&m.EditedAt, &deleted, &replyTo, &pinned, &m.PinnedAt); err != nil {
		return m, err
	}
	m.Deleted = deleted == 1
	m.Pinned = pinned == 1
	if replyTo > 0 {
		m.ReplyTo = &replyTo
	}
	if m.Deleted {
		m.Text = ""
	}
	return m, nil
}

func (s *Store) ListChatMessages(channelID int64, beforeID, afterID int64, limit int, viewerID int64) ([]model.ChatMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	switch {
	case afterID > 0:
		rows, err = s.db.Query(`SELECT `+chatMsgCols+`
			FROM chat_messages m LEFT JOIN users u ON u.id = m.author_id
			WHERE m.channel_id = ? AND m.id > ? ORDER BY m.id LIMIT ?`, channelID, afterID, limit)
	case beforeID > 0:
		rows, err = s.db.Query(`SELECT * FROM (
			SELECT `+chatMsgCols+`
			FROM chat_messages m LEFT JOIN users u ON u.id = m.author_id
			WHERE m.channel_id = ? AND m.id < ? ORDER BY m.id DESC LIMIT ?
		) ORDER BY id`, channelID, beforeID, limit)
	default:
		rows, err = s.db.Query(`SELECT * FROM (
			SELECT `+chatMsgCols+`
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
		m, err := scanChatMsg(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Drain-then-query (single conn): batch attachments, reply previews, reactions.
	ids := make([]int64, 0, len(out))
	for i := range out {
		if !out[i].Deleted {
			ids = append(ids, out[i].ID)
		}
	}
	atts, err := s.batchChatAttachments(ids)
	if err != nil {
		return nil, err
	}
	replyIDs := make([]int64, 0)
	for i := range out {
		if out[i].ReplyTo != nil {
			replyIDs = append(replyIDs, *out[i].ReplyTo)
		}
	}
	replies, err := s.batchChatReplyPreviews(replyIDs)
	if err != nil {
		return nil, err
	}
	reacts, err := s.batchChatReactions(ids, viewerID)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Attachments = atts[out[i].ID]
		out[i].Reactions = reacts[out[i].ID]
		if out[i].ReplyTo != nil {
			if p, ok := replies[*out[i].ReplyTo]; ok {
				out[i].ReplyToAuthor = p.author
				out[i].ReplyToText = p.text
			}
		}
	}
	if out == nil {
		out = []model.ChatMessage{}
	}
	return out, nil
}

type chatReplyPreview struct {
	author string
	text   string
}

// batchChatReplyPreviews loads author+snippet of the parent messages being replied to.
func (s *Store) batchChatReplyPreviews(ids []int64) (map[int64]chatReplyPreview, error) {
	res := map[int64]chatReplyPreview{}
	if len(ids) == 0 {
		return res, nil
	}
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT m.id, COALESCE(u.username,''), m.text, m.deleted
		FROM chat_messages m LEFT JOIN users u ON u.id = m.author_id
		WHERE m.id IN (`+strings.Join(ph, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var author, text string
		var deleted int
		if err := rows.Scan(&id, &author, &text, &deleted); err != nil {
			return nil, err
		}
		if deleted == 1 {
			text = ""
		}
		if len(text) > 140 {
			text = text[:140]
		}
		res[id] = chatReplyPreview{author: author, text: text}
	}
	return res, rows.Err()
}

// batchChatReactions aggregates reactions for many messages (emoji -> count, and
// whether the viewer reacted), in one query.
func (s *Store) batchChatReactions(ids []int64, viewerID int64) (map[int64][]model.ChatReaction, error) {
	res := map[int64][]model.ChatReaction{}
	if len(ids) == 0 {
		return res, nil
	}
	ph := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, viewerID)
	for i, id := range ids {
		ph[i] = "?"
		args = append(args, id)
	}
	rows, err := s.db.Query(`SELECT message_id, emoji, COUNT(*),
		SUM(CASE WHEN user_id = ? THEN 1 ELSE 0 END)
		FROM chat_reactions WHERE message_id IN (`+strings.Join(ph, ",")+`)
		GROUP BY message_id, emoji ORDER BY MIN(id)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mid int64
		var r model.ChatReaction
		var mine int
		if err := rows.Scan(&mid, &r.Emoji, &r.Count, &mine); err != nil {
			return nil, err
		}
		r.Mine = mine > 0
		res[mid] = append(res[mid], r)
	}
	return res, rows.Err()
}

// chatAttachments returns attachments for one message.
func (s *Store) chatAttachments(messageID int64) ([]model.ChatAttachment, error) {
	rows, err := s.db.Query(`SELECT id, file_id, filename FROM chat_attachments WHERE message_id = ?`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ChatAttachment
	for rows.Next() {
		var a model.ChatAttachment
		if err := rows.Scan(&a.ID, &a.FileID, &a.Filename); err != nil {
			return nil, err
		}
		a.URL = fmt.Sprintf("/api/files/%d", a.FileID)
		out = append(out, a)
	}
	return out, rows.Err()
}

// batchChatAttachments loads attachments for many messages in one query.
func (s *Store) batchChatAttachments(messageIDs []int64) (map[int64][]model.ChatAttachment, error) {
	res := map[int64][]model.ChatAttachment{}
	if len(messageIDs) == 0 {
		return res, nil
	}
	ph := make([]string, len(messageIDs))
	args := make([]any, len(messageIDs))
	for i, id := range messageIDs {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT message_id, id, file_id, filename FROM chat_attachments
		WHERE message_id IN (`+strings.Join(ph, ",")+`) ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mid int64
		var a model.ChatAttachment
		if err := rows.Scan(&mid, &a.ID, &a.FileID, &a.Filename); err != nil {
			return nil, err
		}
		a.URL = fmt.Sprintf("/api/files/%d", a.FileID)
		res[mid] = append(res[mid], a)
	}
	return res, rows.Err()
}

func (s *Store) GetChatMessage(id int64) (*model.ChatMessage, error) { return s.GetChatMessageFor(id, 0) }

// GetChatMessageFor loads one message with reply preview, pin state and reactions
// resolved for viewerID (viewer 0 leaves reaction "mine" false).
func (s *Store) GetChatMessageFor(id, viewerID int64) (*model.ChatMessage, error) {
	row := s.db.QueryRow(`SELECT `+chatMsgCols+`
		FROM chat_messages m LEFT JOIN users u ON u.id = m.author_id WHERE m.id = ?`, id)
	m, err := scanChatMsg(row)
	if err != nil {
		return nil, err
	}
	// Drain-then-query: the QueryRow above is fully consumed by scanChatMsg.
	if !m.Deleted {
		m.Attachments, _ = s.chatAttachments(m.ID)
	}
	if m.ReplyTo != nil {
		if rp, err := s.batchChatReplyPreviews([]int64{*m.ReplyTo}); err == nil {
			if p, ok := rp[*m.ReplyTo]; ok {
				m.ReplyToAuthor, m.ReplyToText = p.author, p.text
			}
		}
	}
	if rs, err := s.batchChatReactions([]int64{m.ID}, viewerID); err == nil {
		m.Reactions = rs[m.ID]
	}
	return &m, nil
}

func (s *Store) CreateChatMessage(channelID, authorID int64, text string, atts []model.ChatAttachment, replyTo int64) (*model.ChatMessage, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var replyArg any
	if replyTo > 0 {
		replyArg = replyTo
	}
	res, err := tx.Exec(`INSERT INTO chat_messages(channel_id, author_id, text, reply_to) VALUES(?,?,?,?)`,
		channelID, authorID, text, replyArg)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	for _, a := range atts {
		if a.FileID <= 0 {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO chat_attachments(message_id, file_id, filename) VALUES(?,?,?)`, id, a.FileID, a.Filename); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	// author has obviously read their own message
	s.MarkChatRead(channelID, authorID, id)
	return s.GetChatMessageFor(id, authorID)
}

// SetChatPinned pins or unpins a message.
func (s *Store) SetChatPinned(messageID, userID int64, pinned bool) error {
	if pinned {
		_, err := s.db.Exec(`UPDATE chat_messages SET pinned=1, pinned_at=CURRENT_TIMESTAMP, pinned_by=? WHERE id=? AND deleted=0`, userID, messageID)
		return err
	}
	_, err := s.db.Exec(`UPDATE chat_messages SET pinned=0, pinned_at=NULL, pinned_by=NULL WHERE id=?`, messageID)
	return err
}

// ListPinnedMessages returns a channel's pinned messages, newest pin first.
func (s *Store) ListPinnedMessages(channelID, viewerID int64) ([]model.ChatMessage, error) {
	rows, err := s.db.Query(`SELECT `+chatMsgCols+`
		FROM chat_messages m LEFT JOIN users u ON u.id = m.author_id
		WHERE m.channel_id = ? AND m.pinned = 1 AND m.deleted = 0
		ORDER BY m.pinned_at DESC, m.id DESC`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ChatMessage
	for rows.Next() {
		m, err := scanChatMsg(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []model.ChatMessage{}
	}
	return out, nil
}

// ToggleChatReaction adds the emoji reaction for userID, or removes it if present.
func (s *Store) ToggleChatReaction(messageID, userID int64, emoji string) error {
	var cnt int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM chat_reactions WHERE message_id=? AND user_id=? AND emoji=?`,
		messageID, userID, emoji).Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		_, err := s.db.Exec(`DELETE FROM chat_reactions WHERE message_id=? AND user_id=? AND emoji=?`, messageID, userID, emoji)
		return err
	}
	_, err := s.db.Exec(`INSERT INTO chat_reactions(message_id, user_id, emoji) VALUES(?,?,?)`, messageID, userID, emoji)
	return err
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
