package db

import "kanban/internal/model"

// === Wiki draw.io diagrams ===

// ListWikiDiagrams returns a space's diagrams (without XML/PNG payloads).
func (s *Store) ListWikiDiagrams(spaceID int64) ([]model.WikiDiagram, error) {
	rows, err := s.db.Query(`SELECT id, space_id, title, updated_at FROM wiki_diagrams WHERE space_id=? ORDER BY title COLLATE NOCASE`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.WikiDiagram{}
	for rows.Next() {
		var d model.WikiDiagram
		if err := rows.Scan(&d.ID, &d.SpaceID, &d.Title, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetWikiDiagram returns one diagram including its editable XML (no PNG).
func (s *Store) GetWikiDiagram(id int64) (*model.WikiDiagram, error) {
	var d model.WikiDiagram
	err := s.db.QueryRow(`SELECT id, space_id, title, xml, updated_at FROM wiki_diagrams WHERE id=?`, id).
		Scan(&d.ID, &d.SpaceID, &d.Title, &d.XML, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// WikiDiagramPNG returns the base64-encoded PNG render of a diagram.
func (s *Store) WikiDiagramPNG(id int64) (string, int64, error) {
	var png string
	var spaceID int64
	err := s.db.QueryRow(`SELECT png, space_id FROM wiki_diagrams WHERE id=?`, id).Scan(&png, &spaceID)
	return png, spaceID, err
}

func (s *Store) CreateWikiDiagram(spaceID int64, title string, author int64) (int64, error) {
	return s.db.ExecInsert(`INSERT INTO wiki_diagrams(space_id, title, created_by) VALUES(?,?,?)`, spaceID, title, author)
}

func (s *Store) UpdateWikiDiagram(id int64, title, xml, pngB64 string) error {
	_, err := s.db.Exec(`UPDATE wiki_diagrams SET title=?, xml=?, png=?, updated_at=datetime('now') WHERE id=?`,
		title, xml, pngB64, id)
	return err
}

func (s *Store) DeleteWikiDiagram(id int64) error {
	_, err := s.db.Exec(`DELETE FROM wiki_diagrams WHERE id=?`, id)
	return err
}

// WikiDiagramSpace returns the owning space id of a diagram (for access checks).
func (s *Store) WikiDiagramSpace(id int64) (int64, error) {
	var sid int64
	err := s.db.QueryRow(`SELECT space_id FROM wiki_diagrams WHERE id=?`, id).Scan(&sid)
	return sid, err
}
