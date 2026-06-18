package db

import (
	"database/sql"
	"fmt"
	"html"
	"regexp"
	"strings"

	"kanban/internal/model"
)

// wikiLinkRe matches [[Title]] / [[KEY:Title]] wiki-link tokens. The [[subpages]]
// and [[toc]] markers are excluded by wikiLinkIsMarker below.
var wikiLinkRe = regexp.MustCompile(`\[\[\s*([^\]\n|]+?)\s*\]\]`)

// wikiLinkIsMarker reports whether a [[...]] token is a layout marker (subpages /
// toc) rather than a page link.
func wikiLinkIsMarker(tok string) bool {
	t := strings.ToLower(strings.TrimSpace(tok))
	return t == "subpages" || t == "subpages:tree" || t == "toc"
}

// ResolveWikiLinks parses [[Title]] / [[KEY:Title]] tokens out of markdown and
// resolves each against the wiki: a bare title resolves within currentSpaceID; a
// "KEY:Title" token resolves a page titled Title in the space whose key is KEY.
// Returns a token->pageID map (0 when unresolved) and the distinct resolved ids.
func (s *Store) ResolveWikiLinks(currentSpaceID int64, md string) (map[string]int64, []int64) {
	links := map[string]int64{}
	var ids []int64
	seenID := map[int64]bool{}
	for _, m := range wikiLinkRe.FindAllStringSubmatch(md, -1) {
		tok := strings.TrimSpace(m[1])
		if tok == "" || wikiLinkIsMarker(tok) {
			continue
		}
		if _, done := links[tok]; done {
			continue
		}
		spaceID, title := currentSpaceID, tok
		if i := strings.Index(tok, ":"); i > 0 {
			key := strings.ToUpper(strings.TrimSpace(tok[:i]))
			var sid int64
			if err := s.db.QueryRow(`SELECT id FROM wiki_spaces WHERE key = ?`, key).Scan(&sid); err == nil {
				spaceID, title = sid, strings.TrimSpace(tok[i+1:])
			}
		}
		var pid int64
		if err := s.db.QueryRow(`SELECT id FROM wiki_pages WHERE space_id = ? AND LOWER(title) = LOWER(?) ORDER BY id LIMIT 1`,
			spaceID, title).Scan(&pid); err != nil {
			pid = 0
		}
		links[tok] = pid
		if pid != 0 && !seenID[pid] {
			seenID[pid] = true
			ids = append(ids, pid)
		}
	}
	return links, ids
}

// SetWikiLinks rebuilds the outgoing-link rows for a page (a page never links to
// itself). Called after every save.
func (s *Store) SetWikiLinks(fromPageID int64, toIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM wiki_links WHERE from_page_id = ?`, fromPageID); err != nil {
		return err
	}
	for _, to := range toIDs {
		if to == fromPageID {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO wiki_links(from_page_id, to_page_id) VALUES(?,?) ON CONFLICT DO NOTHING`, fromPageID, to); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// WikiBacklinks returns the pages that link to the given page.
func (s *Store) WikiBacklinks(pageID int64) ([]model.WikiPageRef, error) {
	rows, err := s.db.Query(`SELECT p.id, p.space_id, sp.key, p.title
		FROM wiki_links l
		JOIN wiki_pages p ON p.id = l.from_page_id
		JOIN wiki_spaces sp ON sp.id = p.space_id
		WHERE l.to_page_id = ?
		ORDER BY p.title COLLATE NOCASE`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.WikiPageRef{}
	for rows.Next() {
		var r model.WikiPageRef
		if err := rows.Scan(&r.ID, &r.SpaceID, &r.SpaceKey, &r.Title); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// === Wiki spaces ===

// ListWikiSpaces returns spaces visible to the user with page counts and the
// user's effective role. Admins see everything.
// ListWikiSpaces returns the spaces of the given project plus the user's
// accessible personal spaces (project_id IS NULL). Project-bound spaces are
// gated by project access first (a restricted project hides even its open
// spaces from non-members), then by the space's own wiki role.
func (s *Store) ListWikiSpaces(userID int64, isAdmin bool, projectID int64) ([]model.WikiSpace, error) {
	rows, err := s.db.Query(`SELECT sp.id, sp.key, sp.name, sp.description, sp.visibility, sp.project_id, sp.created_by, sp.created_at,
		(SELECT COUNT(*) FROM wiki_pages p WHERE p.space_id = sp.id),
		COALESCE((SELECT m.role FROM wiki_space_members m WHERE m.space_id = sp.id AND m.user_id = ?), '')
		FROM wiki_spaces sp WHERE sp.project_id = ? OR sp.project_id IS NULL ORDER BY sp.name COLLATE NOCASE`, userID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.WikiSpace
	for rows.Next() {
		var sp model.WikiSpace
		var memberRole string
		if err := rows.Scan(&sp.ID, &sp.Key, &sp.Name, &sp.Description, &sp.Visibility, &sp.ProjectID, &sp.CreatedBy, &sp.CreatedAt, &sp.PageCount, &memberRole); err != nil {
			return nil, err
		}
		// The query already restricts to the current project (+ personal) and the
		// handler gates project access, so the plain wiki role is the effective
		// one here. (A per-row ProjectRole query would deadlock under the single
		// SQLite connection while these rows are open.)
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
	err := s.db.QueryRow(`SELECT sp.id, sp.key, sp.name, sp.description, sp.visibility, sp.project_id, sp.created_by, sp.created_at,
		(SELECT COUNT(*) FROM wiki_pages p WHERE p.space_id = sp.id),
		COALESCE((SELECT m.role FROM wiki_space_members m WHERE m.space_id = sp.id AND m.user_id = ?), '')
		FROM wiki_spaces sp WHERE sp.id = ?`, userID, id).
		Scan(&sp.ID, &sp.Key, &sp.Name, &sp.Description, &sp.Visibility, &sp.ProjectID, &sp.CreatedBy, &sp.CreatedAt, &sp.PageCount, &memberRole)
	if err != nil {
		return nil, err
	}
	sp.MyRole = s.effectiveWikiSpaceRole(&sp, memberRole, userID, isAdmin)
	return &sp, nil
}

// effectiveWikiSpaceRole resolves a space's role, gating project-bound spaces by
// project access (no project access → no space access, regardless of wiki
// visibility). Personal spaces (project_id nil) use the wiki role directly.
func (s *Store) effectiveWikiSpaceRole(sp *model.WikiSpace, memberRole string, userID int64, isAdmin bool) string {
	if sp.ProjectID != nil && !isAdmin {
		prole, err := s.ProjectRole(*sp.ProjectID, userID, isAdmin)
		if err != nil || prole == "" {
			return ""
		}
	}
	return effectiveWikiRole(sp.Visibility, memberRole, userID, sp.CreatedBy, isAdmin)
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

// CreateWikiSpace makes a space in a project (projectID != nil) or a personal
// space (projectID == nil, outside any project).
func (s *Store) CreateWikiSpace(key, name, description, visibility string, projectID *int64, createdBy int64) (int64, error) {
	if visibility != "restricted" {
		visibility = "open"
	}
	id, err := s.db.ExecInsert(`INSERT INTO wiki_spaces(key, name, description, visibility, project_id, created_by) VALUES(?,?,?,?,?,?)`,
		key, name, description, visibility, projectID, createdBy)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, fmt.Errorf("space key %q already exists", key)
		}
		return 0, err
	}
	return id, nil
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
	pageID, err := s.db.ExecInsert(`INSERT INTO wiki_pages(space_id, parent_id, title, content, content_md, position, created_by, updated_by)
		VALUES(?,?,?,?,?,?,?,?)`, spaceID, parentID, title, content, contentMd, pos, author, author)
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
	rows, err := s.db.Query(`SELECT p.id, p.space_id, sp.key, p.title, p.content, sp.visibility, sp.project_id, sp.created_by,
		COALESCE((SELECT m.role FROM wiki_space_members m WHERE m.space_id = sp.id AND m.user_id = ?), '')
		FROM wiki_pages p JOIN wiki_spaces sp ON sp.id = p.space_id
		WHERE p.title LIKE ? ESCAPE '\' OR p.content LIKE ? ESCAPE '\'
		ORDER BY p.updated_at DESC LIMIT ?`, userID, like, like, limit*3)
	if err != nil {
		return nil, err
	}
	// Collect candidates first, then compute roles. effectiveWikiSpaceRole may run
	// a ProjectRole query, which would deadlock under the single SQLite connection
	// if executed while these rows are still open — so we drain the cursor first.
	type cand struct {
		hit        model.WikiSearchHit
		content    string
		sp         model.WikiSpace
		memberRole string
	}
	var cands []cand
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.hit.PageID, &c.hit.SpaceID, &c.hit.SpaceKey, &c.hit.Title, &c.content,
			&c.sp.Visibility, &c.sp.ProjectID, &c.sp.CreatedBy, &c.memberRole); err != nil {
			rows.Close()
			return nil, err
		}
		cands = append(cands, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []model.WikiSearchHit{}
	for _, c := range cands {
		if s.effectiveWikiSpaceRole(&c.sp, c.memberRole, userID, isAdmin) == "" {
			continue
		}
		c.hit.Snippet = wikiSnippet(c.content, query)
		out = append(out, c.hit)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
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

// GetWikiSummary returns the cached LLM summary for a specific page revision.
func (s *Store) GetWikiSummary(pageID int64, revNo int) (string, bool) {
	var summary string
	err := s.db.QueryRow(`SELECT summary FROM wiki_summaries WHERE page_id=? AND rev_no=?`, pageID, revNo).Scan(&summary)
	if err != nil {
		return "", false
	}
	return summary, true
}

// SaveWikiSummary caches an LLM summary for a page revision (idempotent upsert).
func (s *Store) SaveWikiSummary(pageID int64, revNo int, summary, model string) error {
	_, err := s.db.Exec(`INSERT INTO wiki_summaries(page_id, rev_no, summary, model) VALUES(?,?,?,?)
		ON CONFLICT(page_id, rev_no) DO UPDATE SET summary=excluded.summary, model=excluded.model`,
		pageID, revNo, summary, model)
	return err
}

// WikiPagePlainText returns the page's content as plain text (markdown source if
// present, else HTML stripped) — the input for summarization.
func WikiPagePlainText(p *model.WikiPage) string {
	if strings.TrimSpace(p.ContentMd) != "" {
		return p.ContentMd
	}
	return stripTags(p.Content)
}
