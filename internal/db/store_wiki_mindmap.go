package db

import (
	"fmt"
	"strings"
	"time"

	"kanban/internal/model"
)

// wikiSquashWindow is how long consecutive autosaves by the same author keep
// replacing the previous revision instead of piling up a new one each time.
const wikiSquashWindow = 10 * time.Minute

// CreateWikiPageFull creates a page of any kind. For mind maps data is the
// canonical JSON document and content/contentMd the derived outline.
func (s *Store) CreateWikiPageFull(spaceID int64, parentID *int64, title, content, contentMd, kind, data string, author int64) (int64, error) {
	if kind == "" {
		kind = model.WikiKindPage
	}
	var pos int
	s.db.QueryRow(`SELECT COALESCE(MAX(position),-1)+1 FROM wiki_pages WHERE space_id=? AND COALESCE(parent_id,0)=COALESCE(?,0)`,
		spaceID, parentID).Scan(&pos)
	pageID, err := s.db.ExecInsert(`INSERT INTO wiki_pages(space_id, parent_id, title, content, content_md, position, created_by, updated_by, kind, data)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, spaceID, parentID, title, content, contentMd, pos, author, author, kind, data)
	if err != nil {
		return 0, err
	}
	// first revision
	_, err = s.db.Exec(`INSERT INTO wiki_revisions(page_id, rev_no, title, content, content_md, author_id, comment, kind, data)
		VALUES(?,?,?,?,?,?,?,?,?)`, pageID, 1, title, content, contentMd, author, "", kind, data)
	return pageID, err
}

// UpdateWikiPageFull saves a new revision of a page of any kind. baseRev guards
// against concurrent edits (0 = skip the check). With squash set — used by the
// mind-map editor's autosave — the previous revision is dropped when it was an
// uncommented save by the same author within wikiSquashWindow, so a burst of
// autosaves leaves one revision in the history. rev_no still increments on
// every save, so other clients' base_rev checks keep detecting the change.
func (s *Store) UpdateWikiPageFull(id int64, title, content, contentMd, data, comment string, author int64, baseRev int, squash bool) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var cur int
	var kind string
	if err := tx.QueryRow(`SELECT rev_no, kind FROM wiki_pages WHERE id=?`, id).Scan(&cur, &kind); err != nil {
		return 0, err
	}
	if baseRev != 0 && baseRev != cur {
		return cur, fmt.Errorf("conflict: page was modified (rev %d, your base %d)", cur, baseRev)
	}
	if kind == "" {
		kind = model.WikiKindPage
	}
	next := cur + 1
	if _, err := tx.Exec(`UPDATE wiki_pages SET title=?, content=?, content_md=?, data=?, rev_no=?, updated_by=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		title, content, contentMd, data, next, author, id); err != nil {
		return 0, err
	}
	if squash && strings.TrimSpace(comment) == "" && cur > 1 {
		cutoff := time.Now().UTC().Add(-wikiSquashWindow).Format("2006-01-02 15:04:05")
		if _, err := tx.Exec(`DELETE FROM wiki_revisions WHERE page_id=? AND rev_no=? AND author_id=? AND comment='' AND created_at > ?`,
			id, cur, author, cutoff); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(`INSERT INTO wiki_revisions(page_id, rev_no, title, content, content_md, author_id, comment, kind, data)
		VALUES(?,?,?,?,?,?,?,?,?)`, id, next, title, content, contentMd, author, comment, kind, data); err != nil {
		return 0, err
	}
	return next, tx.Commit()
}

// WikiPageTitle returns a page's title ("" when it does not exist). Used to
// render node → page links as [[Title]] in a mind map's derived outline.
func (s *Store) WikiPageTitle(id int64) string {
	var t string
	s.db.QueryRow(`SELECT title FROM wiki_pages WHERE id=?`, id).Scan(&t)
	return t
}

// WikiPageIDByTitle resolves a page title inside a space (case-insensitive;
// 0 when unknown). "KEY:Title" looks in the space with that key.
func (s *Store) WikiPageIDByTitle(spaceID int64, title string) int64 {
	title = strings.TrimSpace(title)
	if i := strings.Index(title, ":"); i > 0 {
		var sid int64
		if err := s.db.QueryRow(`SELECT id FROM wiki_spaces WHERE key = ?`, strings.ToUpper(strings.TrimSpace(title[:i]))).Scan(&sid); err == nil {
			spaceID, title = sid, strings.TrimSpace(title[i+1:])
		}
	}
	var id int64
	s.db.QueryRow(`SELECT id FROM wiki_pages WHERE space_id = ? AND LOWER(title) = LOWER(?) ORDER BY id LIMIT 1`, spaceID, title).Scan(&id)
	return id
}

// WikiPageKind returns a page's kind and space (for access checks on
// kind-specific routes).
func (s *Store) WikiPageKind(id int64) (kind string, spaceID int64, err error) {
	err = s.db.QueryRow(`SELECT kind, space_id FROM wiki_pages WHERE id=?`, id).Scan(&kind, &spaceID)
	return
}
