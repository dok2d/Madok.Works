package db

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"kanban/internal/auth"
	"kanban/internal/model"
)

// === Personal API tokens ===
//
// A token is "mw_" + 64 hex chars. Only its SHA-256 is stored (the token is
// already 256 bits of entropy, so a slow hash buys nothing), plus a short
// display prefix so users can tell their tokens apart.

// APITokenPrefix marks a Madok.Works API token (handy for secret scanners).
const APITokenPrefix = "mw_"

// apiTokenTouchEvery throttles last_used_at writes to one per token per minute.
const apiTokenTouchEvery = 60

var ErrAPITokenInvalid = errors.New("api token: invalid or expired")

func hashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func splitServices(s string) []string {
	out := []string{}
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// CreateAPIToken mints a token for userID and returns the secret (shown to the
// user once) with its stored metadata. expiresAt is unix seconds, 0 = never.
func (s *Store) CreateAPIToken(userID int64, name, role string, services []string, expiresAt int64) (string, *model.APIToken, error) {
	raw, err := auth.GenerateToken()
	if err != nil {
		return "", nil, err
	}
	token := APITokenPrefix + raw
	prefix := token[:len(APITokenPrefix)+8]
	id, err := s.db.ExecInsert(`INSERT INTO api_tokens(user_id, name, token_hash, prefix, role, services, expires_at)
		VALUES(?,?,?,?,?,?,?)`, userID, name, hashAPIToken(token), prefix, role, strings.Join(services, ","), expiresAt)
	if err != nil {
		return "", nil, err
	}
	t, err := s.GetAPIToken(id)
	if err != nil {
		return "", nil, err
	}
	return token, t, nil
}

const apiTokenCols = `t.id, t.user_id, u.username, t.name, t.prefix, t.role, t.services, t.expires_at, t.last_used_at, t.created_at`

func scanAPIToken(sc interface{ Scan(...any) error }) (*model.APIToken, error) {
	var t model.APIToken
	var services string
	if err := sc.Scan(&t.ID, &t.UserID, &t.Username, &t.Name, &t.Prefix, &t.Role, &services,
		&t.ExpiresAt, &t.LastUsedAt, &t.CreatedAt); err != nil {
		return nil, err
	}
	t.Services = splitServices(services)
	return &t, nil
}

func (s *Store) GetAPIToken(id int64) (*model.APIToken, error) {
	return scanAPIToken(s.db.QueryRow(`SELECT `+apiTokenCols+`
		FROM api_tokens t JOIN users u ON u.id = t.user_id WHERE t.id = ?`, id))
}

// ListAPITokens returns a user's tokens, or every user's when userID is 0.
func (s *Store) ListAPITokens(userID int64) ([]model.APIToken, error) {
	q := `SELECT ` + apiTokenCols + ` FROM api_tokens t JOIN users u ON u.id = t.user_id`
	var args []any
	if userID != 0 {
		q += ` WHERE t.user_id = ?`
		args = append(args, userID)
	}
	rows, err := s.db.Query(q+` ORDER BY t.id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.APIToken{}
	for rows.Next() {
		t, err := scanAPIToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// DeleteAPIToken revokes a token; with userID != 0 only that user's own.
// Reports whether a token was removed.
func (s *Store) DeleteAPIToken(id, userID int64) (bool, error) {
	q, args := `DELETE FROM api_tokens WHERE id = ?`, []any{id}
	if userID != 0 {
		q += ` AND user_id = ?`
		args = append(args, userID)
	}
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ValidateAPIToken resolves a bearer token to its owner and the token itself.
// The owner carries their current role; narrowing it to the token's role is
// the caller's job.
func (s *Store) ValidateAPIToken(token string) (*model.User, *model.APIToken, error) {
	if !strings.HasPrefix(token, APITokenPrefix) {
		return nil, nil, ErrAPITokenInvalid
	}
	var t model.APIToken
	var u model.User
	var services string
	var admin int
	err := s.db.QueryRow(`SELECT t.id, t.name, t.prefix, t.role, t.services, t.expires_at, t.last_used_at, t.created_at,
			u.id, u.username, u.is_admin, u.role, u.created_at
		FROM api_tokens t JOIN users u ON u.id = t.user_id WHERE t.token_hash = ?`, hashAPIToken(token)).
		Scan(&t.ID, &t.Name, &t.Prefix, &t.Role, &services, &t.ExpiresAt, &t.LastUsedAt, &t.CreatedAt,
			&u.ID, &u.Username, &admin, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, nil, ErrAPITokenInvalid
	}
	now := time.Now().Unix()
	if t.ExpiresAt != 0 && t.ExpiresAt <= now {
		return nil, nil, ErrAPITokenInvalid
	}
	t.UserID, t.Username, t.Services = u.ID, u.Username, splitServices(services)
	u.IsAdmin = admin == 1
	if now-t.LastUsedAt >= apiTokenTouchEvery {
		s.db.Exec(`UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, now, t.ID)
		t.LastUsedAt = now
	}
	return &u, &t, nil
}

// blobTextRefs lists, per service, the text columns that embed /api/images/N
// or /api/files/N links (the same sources CleanupOrphanFiles scans).
var blobTextRefs = map[string][]string{
	"kanban": {"SELECT text FROM comments WHERE text LIKE ?", "SELECT description FROM tasks WHERE description LIKE ?"},
	"wiki":   {"SELECT content FROM wiki_pages WHERE content LIKE ?", "SELECT content FROM wiki_revisions WHERE content LIKE ?"},
	"chat":   {"SELECT text FROM chat_messages WHERE deleted = 0 AND text LIKE ?"},
	"mail":   {"SELECT body_html FROM mail_messages WHERE body_html LIKE ?", "SELECT body_text FROM mail_messages WHERE body_text LIKE ?"},
}

// blobDirectRefs lists, per service, the tables that point at files(id) directly.
var blobDirectRefs = map[string]string{
	"chat":  "SELECT 1 FROM chat_attachments WHERE file_id = ?",
	"mail":  "SELECT 1 FROM mail_attachments WHERE file_id = ?",
	"files": "SELECT 1 FROM storage_items WHERE file_id = ?",
}

// BlobUsedByServices reports whether an uploaded blob (kind "images" or
// "files") is used by any of the given services, so a service-scoped API token
// can't read another service's attachments by guessing ids.
func (s *Store) BlobUsedByServices(kind string, id int64, services []string) bool {
	link := "/api/" + kind + "/" + strconv.FormatInt(id, 10)
	for _, svc := range services {
		if q, ok := blobDirectRefs[svc]; ok && kind == "files" {
			var one int
			if s.db.QueryRow(q+" LIMIT 1", id).Scan(&one) == nil {
				return true
			}
		}
		for _, q := range blobTextRefs[svc] {
			// LIKE narrows the rows; the regexp rejects longer ids sharing the
			// prefix (/api/files/12 inside /api/files/123).
			rows, err := s.db.Query(q+" LIMIT 50", "%"+link+"%")
			if err != nil {
				continue
			}
			found := false
			for rows.Next() && !found {
				var text string
				if rows.Scan(&text) != nil {
					continue
				}
				for _, m := range fileRefRe.FindAllStringSubmatch(text, -1) {
					if m[1] == kind && m[2] == strconv.FormatInt(id, 10) {
						found = true
						break
					}
				}
			}
			rows.Close()
			if found {
				return true
			}
		}
	}
	return false
}
