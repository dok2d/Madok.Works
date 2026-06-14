package db

import (
	"database/sql"
	"fmt"
	"html"
	"strings"

	"kanban/internal/model"
)

// === Wiki spaces ===

// ListWikiSpaces returns spaces visible to the user with page counts and the
// user's effective role. Admins see everything.
func (s *Store) ListWikiSpaces(userID int64, isAdmin bool) ([]model.WikiSpace, error) {
	rows, err := s.db.Query(`SELECT sp.id, sp.key, sp.name, sp.description, sp.visibility, sp.created_by, sp.created_at,
		(SELECT COUNT(*) FROM wiki_pages p WHERE p.space_id = sp.id),
		COALESCE((SELECT m.role FROM wiki_space_members m WHERE m.space_id = sp.id AND m.user_id = ?), '')
		FROM wiki_spaces sp ORDER BY sp.name COLLATE NOCASE`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.WikiSpace
	for rows.Next() {
		var sp model.WikiSpace
		var memberRole string
		if err := rows.Scan(&sp.ID, &sp.Key, &sp.Name, &sp.Description, &sp.Visibility, &sp.CreatedBy, &sp.CreatedAt, &sp.PageCount, &memberRole); err != nil {
			return nil, err
		}
		sp.MyRole = effectiveWikiRole(sp.Visibility, memberRole, userID, sp.CreatedBy, isAdmin)
		if sp.MyRole == "" {
			continue // restricted space, not a member
		}
		out = append(out, sp)
	}
	if out == nil {
		out = []model.WikiSpace{}
	}
	return out, rows.Err()
}

// GetWikiSpace returns one space with the user's effective role ("" = no access).
func (s *Store) GetWikiSpace(id, userID int64, isAdmin bool) (*model.WikiSpace, error) {
	var sp model.WikiSpace
	var memberRole string
	err := s.db.QueryRow(`SELECT sp.id, sp.key, sp.name, sp.description, sp.visibility, sp.created_by, sp.created_at,
		(SELECT COUNT(*) FROM wiki_pages p WHERE p.space_id = sp.id),
		COALESCE((SELECT m.role FROM wiki_space_members m WHERE m.space_id = sp.id AND m.user_id = ?), '')
		FROM wiki_spaces sp WHERE sp.id = ?`, userID, id).
		Scan(&sp.ID, &sp.Key, &sp.Name, &sp.Description, &sp.Visibility, &sp.CreatedBy, &sp.CreatedAt, &sp.PageCount, &memberRole)
	if err != nil {
		return nil, err
	}
	sp.MyRole = effectiveWikiRole(sp.Visibility, memberRole, userID, sp.CreatedBy, isAdmin)
	return &sp, nil
}

// effectiveWikiRole resolves the user's role in a space.
// Global admins and space creators are space admins. In open spaces every
// user is at least an editor (the handler downgrades global readonly users
// to viewer). In restricted spaces only listed members have access.
func effectiveWikiRole(visibility, memberRole string, userID int64, createdBy *int64, isAdmin bool) string {
	if isAdmin {
		return "admin"
	}
	if createdBy != nil && *createdBy == userID {
		return "admin"
	}
	if memberRole != "" {
		return memberRole
	}
	if visibility == "open" {
		return "editor"
	}
	return ""
}

func (s *Store) CreateWikiSpace(key, name, description, visibility string, createdBy int64) (int64, error) {
	if visibility != "restricted" {
		visibility = "open"
	}
	res, err := s.db.Exec(`INSERT INTO wiki_spaces(key, name, description, visibility, created_by) VALUES(?,?,?,?,?)`,
		key, name, description, visibility, createdBy)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, fmt.Errorf("space key %q already exists", key)
		}
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateWikiSpace(id int64, name, description, visibility string) error {
	if visibility != "restricted" {
		visibility = "open"
	}
	_, err := s.db.Exec(`UPDATE wiki_spaces SET name=?, description=?, visibility=? WHERE id=?`,
		name, description, visibility, id)
	return err
}

func (s *Store) DeleteWikiSpace(id int64) error {
	_, err := s.db.Exec(`DELETE FROM wiki_spaces WHERE id=?`, id)
	return err
}

// === Space members ===

func (s *Store) ListWikiSpaceMembers(spaceID int64) ([]model.WikiSpaceMember, error) {
	rows, err := s.db.Query(`SELECT m.space_id, m.user_id, m.role, u.username
		FROM wiki_space_members m JOIN users u ON u.id = m.user_id
		WHERE m.space_id = ? ORDER BY u.username COLLATE NOCASE`, spaceID)
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

func (s *Store) SetWikiSpaceMember(spaceID, userID int64, role string) error {
	switch role {
	case "admin", "editor", "viewer":
	default:
		return fmt.Errorf("invalid role %q", role)
	}
	_, err := s.db.Exec(`INSERT INTO wiki_space_members(space_id, user_id, role) VALUES(?,?,?)
		ON CONFLICT(space_id, user_id) DO UPDATE SET role=excluded.role`, spaceID, userID, role)
	return err
}

func (s *Store) RemoveWikiSpaceMember(spaceID, userID int64) error {
	_, err := s.db.Exec(`DELETE FROM wiki_space_members WHERE space_id=? AND user_id=?`, spaceID, userID)
	return err
}

// === Pages ===

// ListWikiPageTree returns all pages of a space (without content) ordered for
// tree building: by parent, then position, then title.
func (s *Store) ListWikiPageTree(spaceID int64) ([]model.WikiPage, error) {
	rows, err := s.db.Query(`SELECT id, space_id, parent_id, title, position, rev_no, updated_at
		FROM wiki_pages WHERE space_id = ?
		ORDER BY COALESCE(parent_id, 0), position, title COLLATE NOCASE`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.WikiPage
	for rows.Next() {
		var p model.WikiPage
		if err := rows.Scan(&p.ID, &p.SpaceID, &p.ParentID, &p.Title, &p.Position, &p.RevNo, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []model.WikiPage{}
	}
	return out, rows.Err()
}

func (s *Store) GetWikiPage(id int64) (*model.WikiPage, error) {
	var p model.WikiPage
	var createdName, updatedName sql.NullString
	err := s.db.QueryRow(`SELECT p.id, p.space_id, p.parent_id, p.title, p.content, p.content_md, p.position, p.rev_no,
		p.created_by, p.created_at, p.updated_by, p.updated_at, cu.username, uu.username
		FROM wiki_pages p
		LEFT JOIN users cu ON cu.id = p.created_by
		LEFT JOIN users uu ON uu.id = p.updated_by
		WHERE p.id = ?`, id).
		Scan(&p.ID, &p.SpaceID, &p.ParentID, &p.Title, &p.Content, &p.ContentMd, &p.Position, &p.RevNo,
			&p.CreatedBy, &p.CreatedAt, &p.UpdatedBy, &p.UpdatedAt, &createdName, &updatedName)
	if err != nil {
		return nil, err
	}
	p.CreatedByName = createdName.String
	p.UpdatedByName = updatedName.String
	return &p, nil
}

func (s *Store) CreateWikiPage(spaceID int64, parentID *int64, title, content, contentMd string, author int64) (int64, error) {
	var pos int
	s.db.QueryRow(`SELECT COALESCE(MAX(position),-1)+1 FROM wiki_pages WHERE space_id=? AND COALESCE(parent_id,0)=COALESCE(?,0)`,
		spaceID, parentID).Scan(&pos)
	res, err := s.db.Exec(`INSERT INTO wiki_pages(space_id, parent_id, title, content, content_md, position, created_by, updated_by)
		VALUES(?,?,?,?,?,?,?,?)`, spaceID, parentID, title, content, contentMd, pos, author, author)
	if err != nil {
		return 0, err
	}
	pageID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	// first revision
	_, err = s.db.Exec(`INSERT INTO wiki_revisions(page_id, rev_no, title, content, content_md, author_id, comment)
		VALUES(?,?,?,?,?,?,?)`, pageID, 1, title, content, contentMd, author, "")
	return pageID, err
}

// UpdateWikiPage saves a new revision. baseRev guards against concurrent
// edits: if it does not match the stored rev_no the update is rejected.
func (s *Store) UpdateWikiPage(id int64, title, content, contentMd, comment string, author int64, baseRev int) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var cur int
	if err := tx.QueryRow(`SELECT rev_no FROM wiki_pages WHERE id=?`, id).Scan(&cur); err != nil {
		return 0, err
	}
	if baseRev != 0 && baseRev != cur {
		return cur, fmt.Errorf("conflict: page was modified (rev %d, your base %d)", cur, baseRev)
	}
	next := cur + 1
	if _, err := tx.Exec(`UPDATE wiki_pages SET title=?, content=?, content_md=?, rev_no=?, updated_by=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		title, content, contentMd, next, author, id); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`INSERT INTO wiki_revisions(page_id, rev_no, title, content, content_md, author_id, comment)
		VALUES(?,?,?,?,?,?,?)`, id, next, title, content, contentMd, author, comment); err != nil {
		return 0, err
	}
	return next, tx.Commit()
}

// DeleteWikiPage removes a page; its children are re-parented to the deleted
// page's parent so no subtree is lost accidentally.
func (s *Store) DeleteWikiPage(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var parentID *int64
	if err := tx.QueryRow(`SELECT parent_id FROM wiki_pages WHERE id=?`, id).Scan(&parentID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE wiki_pages SET parent_id=? WHERE parent_id=?`, parentID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM wiki_pages WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// MoveWikiPage changes a page's parent and/or position within siblings.
// It refuses moves that would create a cycle (page under its own descendant).
func (s *Store) MoveWikiPage(id int64, newParent *int64, position int) error {
	if newParent != nil {
		// walk up from newParent; if we meet id, it's a cycle
		cur := *newParent
		for cur != 0 {
			if cur == id {
				return fmt.Errorf("cannot move a page under its own descendant")
			}
			var next sql.NullInt64
			if err := s.db.QueryRow(`SELECT parent_id FROM wiki_pages WHERE id=?`, cur).Scan(&next); err != nil {
				return err
			}
			if !next.Valid {
				break
			}
			cur = next.Int64
		}
		// parent must exist in the same space
		var sameSpace int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM wiki_pages a, wiki_pages b
			WHERE a.id=? AND b.id=? AND a.space_id=b.space_id`, id, *newParent).Scan(&sameSpace); err != nil {
			return err
		}
		if sameSpace == 0 {
			return fmt.Errorf("target parent not found in the same space")
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var spaceID int64
	if err := tx.QueryRow(`SELECT space_id FROM wiki_pages WHERE id=?`, id).Scan(&spaceID); err != nil {
		return err
	}
	// shift siblings at and after the target position, then place the page
	if _, err := tx.Exec(`UPDATE wiki_pages SET position = position + 1
		WHERE space_id=? AND COALESCE(parent_id,0)=COALESCE(?,0) AND position >= ? AND id != ?`,
		spaceID, newParent, position, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE wiki_pages SET parent_id=?, position=? WHERE id=?`, newParent, position, id); err != nil {
		return err
	}
	return tx.Commit()
}

// === Revisions ===

func (s *Store) ListWikiRevisions(pageID int64) ([]model.WikiRevision, error) {
	rows, err := s.db.Query(`SELECT r.id, r.page_id, r.rev_no, r.title, r.author_id, COALESCE(u.username,''), r.comment, r.created_at
		FROM wiki_revisions r LEFT JOIN users u ON u.id = r.author_id
		WHERE r.page_id = ? ORDER BY r.rev_no DESC`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.WikiRevision
	for rows.Next() {
		var r model.WikiRevision
		if err := rows.Scan(&r.ID, &r.PageID, &r.RevNo, &r.Title, &r.AuthorID, &r.AuthorName, &r.Comment, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if out == nil {
		out = []model.WikiRevision{}
	}
	return out, rows.Err()
}

func (s *Store) GetWikiRevision(id int64) (*model.WikiRevision, error) {
	var r model.WikiRevision
	err := s.db.QueryRow(`SELECT r.id, r.page_id, r.rev_no, r.title, r.content, r.content_md, r.author_id, COALESCE(u.username,''), r.comment, r.created_at
		FROM wiki_revisions r LEFT JOIN users u ON u.id = r.author_id WHERE r.id = ?`, id).
		Scan(&r.ID, &r.PageID, &r.RevNo, &r.Title, &r.Content, &r.ContentMd, &r.AuthorID, &r.AuthorName, &r.Comment, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// === Search ===

// SearchWikiPages does a LIKE search over title and content of pages in
// spaces visible to the user. Snippets are plain text around the first match.
func (s *Store) SearchWikiPages(userID int64, isAdmin bool, query string, limit int) ([]model.WikiSearchHit, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	like := "%" + escapeLike(query) + "%"
	rows, err := s.db.Query(`SELECT p.id, p.space_id, sp.key, p.title, p.content, sp.visibility, sp.created_by,
		COALESCE((SELECT m.role FROM wiki_space_members m WHERE m.space_id = sp.id AND m.user_id = ?), '')
		FROM wiki_pages p JOIN wiki_spaces sp ON sp.id = p.space_id
		WHERE p.title LIKE ? ESCAPE '\' OR p.content LIKE ? ESCAPE '\'
		ORDER BY p.updated_at DESC LIMIT ?`, userID, like, like, limit*3)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.WikiSearchHit
	for rows.Next() {
		var h model.WikiSearchHit
		var content, visibility, memberRole string
		var createdBy *int64
		if err := rows.Scan(&h.PageID, &h.SpaceID, &h.SpaceKey, &h.Title, &content, &visibility, &createdBy, &memberRole); err != nil {
			return nil, err
		}
		if effectiveWikiRole(visibility, memberRole, userID, createdBy, isAdmin) == "" {
			continue
		}
		h.Snippet = wikiSnippet(content, query)
		out = append(out, h)
		if len(out) >= limit {
			break
		}
	}
	if out == nil {
		out = []model.WikiSearchHit{}
	}
	return out, rows.Err()
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// wikiSnippet strips tags and returns ~160 chars of text around the first
// case-insensitive match of query.
func wikiSnippet(rawHTML, query string) string {
	text := html.UnescapeString(stripTags(rawHTML))
	lower := strings.ToLower(text)
	idx := strings.Index(lower, strings.ToLower(query))
	if idx < 0 {
		idx = 0
	}
	start := idx - 60
	if start < 0 {
		start = 0
	}
	end := start + 160
	if end > len(text) {
		end = len(text)
	}
	// avoid splitting multi-byte runes
	for start > 0 && start < len(text) && text[start]&0xC0 == 0x80 {
		start--
	}
	for end < len(text) && text[end]&0xC0 == 0x80 {
		end++
	}
	snippet := strings.TrimSpace(text[start:end])
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(text) {
		snippet += "…"
	}
	return snippet
}

// stripTags removes HTML tags and collapses whitespace (for snippets only).
func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	lastSpace := true
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		case r == '>':
			inTag = false
		case !inTag:
			if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
				if !lastSpace {
					b.WriteByte(' ')
					lastSpace = true
				}
			} else {
				b.WriteRune(r)
				lastSpace = false
			}
		}
	}
	return strings.TrimSpace(b.String())
}
