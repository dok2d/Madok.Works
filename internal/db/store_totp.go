package db

import "kanban/internal/auth"

// === Two-factor authentication (TOTP) ===

// GetTOTP returns a user's TOTP secret and whether 2FA is enabled.
func (s *Store) GetTOTP(userID int64) (secret string, enabled bool, err error) {
	var en int
	err = s.db.QueryRow(`SELECT COALESCE(totp_secret,''), COALESCE(totp_enabled,0) FROM users WHERE id=?`, userID).Scan(&secret, &en)
	return secret, en == 1, err
}

// SetTOTPSecret stores a (pending) secret and enabled flag for a user.
func (s *Store) SetTOTPSecret(userID int64, secret string, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	_, err := s.db.Exec(`UPDATE users SET totp_secret=?, totp_enabled=? WHERE id=?`, secret, en, userID)
	return err
}

// DisableTOTP clears a user's 2FA secret and recovery codes.
func (s *Store) DisableTOTP(userID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE users SET totp_secret='', totp_enabled=0 WHERE id=?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM totp_recovery WHERE user_id=?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceRecoveryCodes hashes and stores a fresh set of single-use recovery codes
// (clearing any previous ones).
func (s *Store) ReplaceRecoveryCodes(userID int64, codes []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM totp_recovery WHERE user_id=?`, userID); err != nil {
		return err
	}
	for _, c := range codes {
		h, err := auth.HashPassword(c)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO totp_recovery(user_id, code_hash, used) VALUES(?,?,0)`, userID, h); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UseRecoveryCode consumes a matching unused recovery code, returning true on
// success (the code is then marked used).
func (s *Store) UseRecoveryCode(userID int64, code string) bool {
	rows, err := s.db.Query(`SELECT id, code_hash FROM totp_recovery WHERE user_id=? AND used=0`, userID)
	if err != nil {
		return false
	}
	type rc struct {
		id   int64
		hash string
	}
	var list []rc
	for rows.Next() {
		var r rc
		if rows.Scan(&r.id, &r.hash) == nil {
			list = append(list, r)
		}
	}
	rows.Close()
	for _, r := range list {
		if auth.CheckPassword(code, r.hash) {
			s.db.Exec(`UPDATE totp_recovery SET used=1 WHERE id=?`, r.id)
			return true
		}
	}
	return false
}
