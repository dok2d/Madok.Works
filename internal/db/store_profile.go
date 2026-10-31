package db

import "strings"

// === Public profile: display name + avatar ===

// UpdateUserDisplayName sets the name shown instead of the login ("" = login).
func (s *Store) UpdateUserDisplayName(id int64, name string) error {
	_, err := s.db.Exec("UPDATE users SET display_name=? WHERE id=?", strings.TrimSpace(name), id)
	return err
}

// FillDisplayNameIfEmpty sets the display name only when the user has none, so
// a name from SSO seeds the profile without overwriting the user's own choice.
func (s *Store) FillDisplayNameIfEmpty(id int64, name string) {
	if name = strings.TrimSpace(name); name != "" {
		s.db.Exec("UPDATE users SET display_name=? WHERE id=? AND display_name=''", name, id)
	}
}

// SetUserAvatar stores a new avatar image and drops the previous one.
func (s *Store) SetUserAvatar(userID int64, data []byte, mime string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var old int64
	tx.QueryRow("SELECT avatar_id FROM users WHERE id=?", userID).Scan(&old)
	id, err := tx.ExecInsert("INSERT INTO images(data,mime) VALUES(?,?)", data, mime)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec("UPDATE users SET avatar_id=? WHERE id=?", id, userID); err != nil {
		return 0, err
	}
	if old != 0 {
		if _, err := tx.Exec("DELETE FROM images WHERE id=?", old); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// ClearUserAvatar removes the user's avatar (the UI falls back to initials).
func (s *Store) ClearUserAvatar(userID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old int64
	tx.QueryRow("SELECT avatar_id FROM users WHERE id=?", userID).Scan(&old)
	if _, err := tx.Exec("UPDATE users SET avatar_id=0 WHERE id=?", userID); err != nil {
		return err
	}
	if old != 0 {
		if _, err := tx.Exec("DELETE FROM images WHERE id=?", old); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// IsAvatar reports whether an image is some user's avatar (avatars are public
// to every signed-in user and to any API token).
func (s *Store) IsAvatar(imageID int64) bool {
	var one int
	return imageID != 0 && s.db.QueryRow("SELECT 1 FROM users WHERE avatar_id=? LIMIT 1", imageID).Scan(&one) == nil
}
