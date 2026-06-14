package db

import (
	"fmt"
	"strings"

	"kanban/internal/model"
)

func mailSnippet(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 140 {
		return text[:140] + "…"
	}
	return text
}

// mailSystemFolders are the built-in folders that can't be created/renamed/deleted.
var mailSystemFolders = map[string]bool{"inbox": true, "sent": true, "drafts": true, "trash": true, "junk": true}

// === Custom folders ===

func (s *Store) ListMailFolders(userID int64) ([]model.MailFolder, error) {
	rows, err := s.db.Query(`SELECT id, name FROM mail_folders WHERE user_id = ? ORDER BY name COLLATE NOCASE`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.MailFolder
	for rows.Next() {
		var f model.MailFolder
		if err := rows.Scan(&f.ID, &f.Name); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if out == nil {
		out = []model.MailFolder{}
	}
	return out, rows.Err()
}

func (s *Store) CreateMailFolder(userID int64, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 60 {
		return 0, fmt.Errorf("invalid folder name")
	}
	if mailSystemFolders[strings.ToLower(name)] {
		return 0, fmt.Errorf("reserved folder name")
	}
	res, err := s.db.Exec(`INSERT INTO mail_folders(user_id, name) VALUES(?,?)`, userID, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) RenameMailFolder(id, userID int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 60 || mailSystemFolders[strings.ToLower(name)] {
		return fmt.Errorf("invalid folder name")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old string
	if err := tx.QueryRow(`SELECT name FROM mail_folders WHERE id=? AND user_id=?`, id, userID).Scan(&old); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE mail_folders SET name=? WHERE id=? AND user_id=?`, name, id, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE mail_box SET folder=? WHERE user_id=? AND folder=?`, name, userID, old); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteMailFolder removes a custom folder and moves its messages back to inbox.
func (s *Store) DeleteMailFolder(id, userID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var name string
	if err := tx.QueryRow(`SELECT name FROM mail_folders WHERE id=? AND user_id=?`, id, userID).Scan(&name); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE mail_box SET folder='inbox' WHERE user_id=? AND folder=?`, userID, name); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM mail_folders WHERE id=? AND user_id=?`, id, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// validMailFolder reports whether folder is a system folder or one the user owns.
func (s *Store) validMailFolder(userID int64, folder string) bool {
	if mailSystemFolders[strings.ToLower(folder)] {
		return true
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM mail_folders WHERE user_id=? AND name=?`, userID, folder).Scan(&n)
	return n > 0
}

// SendInternalMail stores a message and delivers it: a "sent" copy for the
// sender and an "inbox" copy for each recipient. threadID 0 starts a new thread.
func (s *Store) SendInternalMail(fromUserID int64, fromName string, toUserIDs []int64, toAddrs, ccAddrs, subject, bodyText, bodyHTML, inReplyTo string, threadID int64, atts []model.MailAttachment) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO mail_messages(thread_id, from_user_id, from_name, to_addrs, cc_addrs, subject, body_text, body_html, in_reply_to)
		VALUES(?,?,?,?,?,?,?,?,?)`, threadID, fromUserID, fromName, toAddrs, ccAddrs, subject, bodyText, bodyHTML, inReplyTo)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if threadID == 0 {
		if _, err := tx.Exec(`UPDATE mail_messages SET thread_id = ? WHERE id = ?`, id, id); err != nil {
			return 0, err
		}
	}
	for _, a := range atts {
		if _, err := tx.Exec(`INSERT INTO mail_attachments(message_id, file_id, filename) VALUES(?,?,?)`, id, a.FileID, a.Filename); err != nil {
			return 0, err
		}
	}
	// sender's Sent copy (already read)
	if _, err := tx.Exec(`INSERT INTO mail_box(user_id, message_id, folder, seen) VALUES(?,?, 'sent', 1)`, fromUserID, id); err != nil {
		return 0, err
	}
	seen := map[int64]bool{fromUserID: true}
	for _, uid := range toUserIDs {
		if seen[uid] {
			continue
		}
		seen[uid] = true
		if _, err := tx.Exec(`INSERT INTO mail_box(user_id, message_id, folder, seen) VALUES(?,?, 'inbox', 0)`, uid, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// DeliverInbound stores an externally-received message and files an Inbox copy
// for each local recipient (no Sent copy; sender is external).
func (s *Store) DeliverInbound(toUserIDs []int64, fromName, fromAddr, toAddrs, subject, bodyText, bodyHTML string, atts []model.MailAttachment) (int64, error) {
	if len(toUserIDs) == 0 {
		return 0, fmt.Errorf("no local recipients")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO mail_messages(thread_id, from_user_id, from_name, from_addr, to_addrs, subject, body_text, body_html)
		VALUES(0, NULL, ?, ?, ?, ?, ?, ?)`, fromName, fromAddr, toAddrs, subject, bodyText, bodyHTML)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if _, err := tx.Exec(`UPDATE mail_messages SET thread_id = ? WHERE id = ?`, id, id); err != nil {
		return 0, err
	}
	for _, a := range atts {
		if _, err := tx.Exec(`INSERT INTO mail_attachments(message_id, file_id, filename) VALUES(?,?,?)`, id, a.FileID, a.Filename); err != nil {
			return 0, err
		}
	}
	seen := map[int64]bool{}
	for _, uid := range toUserIDs {
		if seen[uid] {
			continue
		}
		seen[uid] = true
		if _, err := tx.Exec(`INSERT INTO mail_box(user_id, message_id, folder, seen) VALUES(?,?, 'inbox', 0)`, uid, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

const mailListCols = `m.id, m.thread_id, m.from_user_id, m.from_name, m.from_addr, m.subject, m.created_at,
	b.id, b.folder, b.seen, b.flagged, m.body_text`

func scanMailListRow(sc interface{ Scan(...any) error }) (*model.MailMessage, error) {
	var m model.MailMessage
	var seen, flagged int
	var body string
	if err := sc.Scan(&m.ID, &m.ThreadID, &m.FromUserID, &m.FromName, &m.FromAddr, &m.Subject, &m.CreatedAt,
		&m.BoxID, &m.Folder, &seen, &flagged, &body); err != nil {
		return nil, err
	}
	m.Seen, m.Flagged = seen == 1, flagged == 1
	m.Snippet = mailSnippet(body)
	return &m, nil
}

// ListMailbox returns the messages in a user's folder, newest first.
func (s *Store) ListMailbox(userID int64, folder string, limit, offset int) ([]model.MailMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT `+mailListCols+`
		FROM mail_box b JOIN mail_messages m ON m.id = b.message_id
		WHERE b.user_id = ? AND b.folder = ?
		ORDER BY b.id DESC LIMIT ? OFFSET ?`, userID, folder, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.MailMessage
	for rows.Next() {
		m, err := scanMailListRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	if out == nil {
		out = []model.MailMessage{}
	}
	return out, rows.Err()
}

// GetMailBox loads one mailbox entry (full body + attachments) for the user.
func (s *Store) GetMailBox(userID, boxID int64) (*model.MailMessage, error) {
	var m model.MailMessage
	var seen, flagged int
	err := s.db.QueryRow(`SELECT m.id, m.thread_id, m.from_user_id, m.from_name, m.from_addr, m.to_addrs, m.cc_addrs,
		m.subject, m.body_text, m.body_html, m.in_reply_to, m.created_at, b.id, b.folder, b.seen, b.flagged
		FROM mail_box b JOIN mail_messages m ON m.id = b.message_id
		WHERE b.id = ? AND b.user_id = ?`, boxID, userID).
		Scan(&m.ID, &m.ThreadID, &m.FromUserID, &m.FromName, &m.FromAddr, &m.ToAddrs, &m.CcAddrs,
			&m.Subject, &m.BodyText, &m.BodyHTML, &m.InReplyTo, &m.CreatedAt, &m.BoxID, &m.Folder, &seen, &flagged)
	if err != nil {
		return nil, err
	}
	m.Seen, m.Flagged = seen == 1, flagged == 1
	m.Attachments, _ = s.mailAttachments(m.ID)
	return &m, nil
}

func (s *Store) mailAttachments(messageID int64) ([]model.MailAttachment, error) {
	rows, err := s.db.Query(`SELECT id, file_id, filename FROM mail_attachments WHERE message_id = ?`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.MailAttachment
	for rows.Next() {
		var a model.MailAttachment
		if err := rows.Scan(&a.ID, &a.FileID, &a.Filename); err != nil {
			return nil, err
		}
		a.URL = fmt.Sprintf("/api/files/%d", a.FileID)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ThreadForUser returns all messages in a thread the user can see (has a box for),
// oldest first, with full bodies and attachments.
func (s *Store) ThreadForUser(userID, threadID int64) ([]model.MailMessage, error) {
	rows, err := s.db.Query(`SELECT DISTINCT m.id, m.thread_id, m.from_user_id, m.from_name, m.from_addr, m.to_addrs, m.cc_addrs,
		m.subject, m.body_text, m.body_html, m.created_at, b.id, b.folder, b.seen, b.flagged
		FROM mail_messages m JOIN mail_box b ON b.message_id = m.id AND b.user_id = ?
		WHERE m.thread_id = ? ORDER BY m.id`, userID, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []model.MailMessage
	for rows.Next() {
		var m model.MailMessage
		var seen, flagged int
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.FromUserID, &m.FromName, &m.FromAddr, &m.ToAddrs, &m.CcAddrs,
			&m.Subject, &m.BodyText, &m.BodyHTML, &m.CreatedAt, &m.BoxID, &m.Folder, &seen, &flagged); err != nil {
			return nil, err
		}
		m.Seen, m.Flagged = seen == 1, flagged == 1
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range msgs {
		msgs[i].Attachments, _ = s.mailAttachments(msgs[i].ID)
	}
	if msgs == nil {
		msgs = []model.MailMessage{}
	}
	return msgs, nil
}

func (s *Store) SetMailSeen(boxID, userID int64, seen bool) error {
	v := 0
	if seen {
		v = 1
	}
	_, err := s.db.Exec(`UPDATE mail_box SET seen = ? WHERE id = ? AND user_id = ?`, v, boxID, userID)
	return err
}

func (s *Store) SetMailFlagged(boxID, userID int64, flagged bool) error {
	v := 0
	if flagged {
		v = 1
	}
	_, err := s.db.Exec(`UPDATE mail_box SET flagged = ? WHERE id = ? AND user_id = ?`, v, boxID, userID)
	return err
}

// MoveMail changes the folder of a mailbox entry (e.g. to "trash"). The target
// must be a system folder or one the user owns.
func (s *Store) MoveMail(boxID, userID int64, folder string) error {
	folder = strings.TrimSpace(folder)
	if !s.validMailFolder(userID, folder) {
		return fmt.Errorf("unknown folder")
	}
	_, err := s.db.Exec(`UPDATE mail_box SET folder = ? WHERE id = ? AND user_id = ?`, folder, boxID, userID)
	return err
}

// DeleteMailBox permanently removes a mailbox entry for the user.
func (s *Store) DeleteMailBox(boxID, userID int64) error {
	_, err := s.db.Exec(`DELETE FROM mail_box WHERE id = ? AND user_id = ?`, boxID, userID)
	return err
}

func (s *Store) MailFolderCounts(userID int64) ([]model.MailFolderCount, error) {
	rows, err := s.db.Query(`SELECT folder, SUM(CASE WHEN seen = 0 THEN 1 ELSE 0 END), COUNT(*)
		FROM mail_box WHERE user_id = ? GROUP BY folder`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.MailFolderCount
	for rows.Next() {
		var fc model.MailFolderCount
		if err := rows.Scan(&fc.Folder, &fc.Unread, &fc.Total); err != nil {
			return nil, err
		}
		out = append(out, fc)
	}
	if out == nil {
		out = []model.MailFolderCount{}
	}
	return out, rows.Err()
}

// UnreadMailCount counts unseen messages in the user's inbox (for the rail badge).
func (s *Store) UnreadMailCount(userID int64) int {
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM mail_box WHERE user_id = ? AND folder = 'inbox' AND seen = 0`, userID).Scan(&n)
	return n
}

// SearchMail does a LIKE search over the user's messages (subject + bodies).
func (s *Store) SearchMail(userID int64, query string, limit int) ([]model.MailMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	like := "%" + escapeLike(query) + "%"
	rows, err := s.db.Query(`SELECT `+mailListCols+`
		FROM mail_box b JOIN mail_messages m ON m.id = b.message_id
		WHERE b.user_id = ? AND (m.subject LIKE ? ESCAPE '\' OR m.body_text LIKE ? ESCAPE '\' OR m.from_name LIKE ? ESCAPE '\')
		ORDER BY b.id DESC LIMIT ?`, userID, like, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.MailMessage
	for rows.Next() {
		m, err := scanMailListRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	if out == nil {
		out = []model.MailMessage{}
	}
	return out, rows.Err()
}
