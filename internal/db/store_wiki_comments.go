package db

import (
	"strings"

	"kanban/internal/model"
)

// === Wiki comments ===

// ListWikiComments returns a page's comments (oldest first) with reactions for
// the requesting user. The client renders threading from parent_id.
func (s *Store) ListWikiComments(pageID, viewerID int64) ([]model.WikiComment, error) {
	rows, err := s.db.Query(`SELECT c.id, c.page_id, c.parent_id, c.author_id, COALESCE(u.username,''),
		c.text, c.created_at, c.updated_at
		FROM wiki_comments c LEFT JOIN users u ON u.id = c.author_id
		WHERE c.page_id = ? ORDER BY c.created_at, c.id`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.WikiComment{}
	var ids []int64
	for rows.Next() {
		var c model.WikiComment
		if err := rows.Scan(&c.ID, &c.PageID, &c.ParentID, &c.AuthorID, &c.AuthorName, &c.Text, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
		ids = append(ids, c.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	reacts, err := s.batchWikiReactions("comment", ids, viewerID)
	if err == nil {
		for i := range out {
			out[i].Reactions = reacts[out[i].ID]
		}
	}
	return out, nil
}

// GetWikiComment fetches one comment (for ownership checks on edit/delete).
func (s *Store) GetWikiComment(id int64) (*model.WikiComment, error) {
	var c model.WikiComment
	err := s.db.QueryRow(`SELECT id, page_id, parent_id, author_id, text, created_at, updated_at
		FROM wiki_comments WHERE id = ?`, id).
		Scan(&c.ID, &c.PageID, &c.ParentID, &c.AuthorID, &c.Text, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) AddWikiComment(pageID int64, parentID *int64, text string, authorID int64) (int64, error) {
	return s.db.ExecInsert(`INSERT INTO wiki_comments(page_id, parent_id, author_id, text) VALUES(?,?,?,?)`,
		pageID, parentID, authorID, text)
}

func (s *Store) UpdateWikiComment(id int64, text string) error {
	_, err := s.db.Exec(`UPDATE wiki_comments SET text=?, updated_at=datetime('now') WHERE id=?`, text, id)
	return err
}

func (s *Store) DeleteWikiComment(id int64) error {
	_, err := s.db.Exec(`DELETE FROM wiki_comments WHERE id=?`, id)
	return err
}

// === Wiki reactions (pages and comments) ===

// ToggleWikiReaction adds or removes the viewer's reaction on a page or comment.
func (s *Store) ToggleWikiReaction(targetType string, targetID, userID int64, emoji string) error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM wiki_reactions WHERE target_type=? AND target_id=? AND user_id=? AND emoji=?`,
		targetType, targetID, userID, emoji).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		_, err := s.db.Exec(`DELETE FROM wiki_reactions WHERE target_type=? AND target_id=? AND user_id=? AND emoji=?`,
			targetType, targetID, userID, emoji)
		return err
	}
	_, err := s.db.Exec(`INSERT INTO wiki_reactions(target_type, target_id, user_id, emoji) VALUES(?,?,?,?) ON CONFLICT DO NOTHING`,
		targetType, targetID, userID, emoji)
	return err
}

// WikiReactions returns aggregated reactions for a single target.
func (s *Store) WikiReactions(targetType string, targetID, viewerID int64) ([]model.ChatReaction, error) {
	m, err := s.batchWikiReactions(targetType, []int64{targetID}, viewerID)
	if err != nil {
		return nil, err
	}
	return m[targetID], nil
}

// batchWikiReactions aggregates reactions (emoji -> count + whether viewer
// reacted) for many targets of one type, in a single query.
func (s *Store) batchWikiReactions(targetType string, ids []int64, viewerID int64) (map[int64][]model.ChatReaction, error) {
	res := map[int64][]model.ChatReaction{}
	if len(ids) == 0 {
		return res, nil
	}
	ph := make([]string, len(ids))
	args := make([]any, 0, len(ids)+2)
	args = append(args, viewerID, targetType)
	for i, id := range ids {
		ph[i] = "?"
		args = append(args, id)
	}
	rows, err := s.db.Query(`SELECT target_id, emoji, COUNT(*),
		SUM(CASE WHEN user_id = ? THEN 1 ELSE 0 END)
		FROM wiki_reactions WHERE target_type = ? AND target_id IN (`+strings.Join(ph, ",")+`)
		GROUP BY target_id, emoji ORDER BY MIN(id)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tid int64
		var r model.ChatReaction
		var mine int
		if err := rows.Scan(&tid, &r.Emoji, &r.Count, &mine); err != nil {
			return nil, err
		}
		r.Mine = mine > 0
		res[tid] = append(res[tid], r)
	}
	return res, rows.Err()
}
