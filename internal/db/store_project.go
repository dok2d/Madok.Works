package db

import (
	"fmt"
	"strconv"
	"strings"

	"kanban/internal/model"
)

// Projects are the top-level container (own kanban/wiki/calendar). This mirrors
// the wiki-space container+membership+role model (see store_wiki.go).

// effectiveProjectRole resolves the user's role in a project. Global admins and
// the creator are project admins. In "open" projects every user is at least an
// editor (the handler downgrades global readonly users to viewer). In
// "restricted" projects only listed members have access.
func effectiveProjectRole(visibility, memberRole string, userID int64, createdBy *int64, isAdmin bool) string {
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

// ListProjects returns the projects visible to the user with their effective role.
func (s *Store) ListProjects(userID int64, isAdmin bool) ([]model.Project, error) {
	rows, err := s.db.Query(`SELECT p.id, p.name, p.key, p.description, p.visibility, p.created_by, p.created_at,
		COALESCE((SELECT m.role FROM project_members m WHERE m.project_id = p.id AND m.user_id = ?), '')
		FROM projects p ORDER BY (p.id=1) DESC, p.name COLLATE NOCASE`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Project
	for rows.Next() {
		var p model.Project
		var memberRole string
		if err := rows.Scan(&p.ID, &p.Name, &p.Key, &p.Description, &p.Visibility, &p.CreatedBy, &p.CreatedAt, &memberRole); err != nil {
			return nil, err
		}
		p.MyRole = effectiveProjectRole(p.Visibility, memberRole, userID, p.CreatedBy, isAdmin)
		if p.MyRole == "" {
			continue // restricted project, not a member
		}
		out = append(out, p)
	}
	if out == nil {
		out = []model.Project{}
	}
	return out, rows.Err()
}

// GetProject returns one project with the user's effective role ("" = no access).
func (s *Store) GetProject(id, userID int64, isAdmin bool) (*model.Project, error) {
	var p model.Project
	var memberRole string
	err := s.db.QueryRow(`SELECT p.id, p.name, p.key, p.description, p.visibility, p.created_by, p.created_at,
		COALESCE((SELECT m.role FROM project_members m WHERE m.project_id = p.id AND m.user_id = ?), '')
		FROM projects p WHERE p.id = ?`, userID, id).
		Scan(&p.ID, &p.Name, &p.Key, &p.Description, &p.Visibility, &p.CreatedBy, &p.CreatedAt, &memberRole)
	if err != nil {
		return nil, err
	}
	p.MyRole = effectiveProjectRole(p.Visibility, memberRole, userID, p.CreatedBy, isAdmin)
	return &p, nil
}

// ProjectRole returns the user's effective role in a project ("" = no access).
func (s *Store) ProjectRole(projectID, userID int64, isAdmin bool) (string, error) {
	p, err := s.GetProject(projectID, userID, isAdmin)
	if err != nil {
		return "", err
	}
	return p.MyRole, nil
}

func (s *Store) CreateProject(name, description, visibility string, createdBy int64) (int64, error) {
	if visibility != "open" {
		visibility = "restricted"
	}
	key := s.uniqueProjectKey(name)
	id, err := s.db.ExecInsert(`INSERT INTO projects(name, key, description, visibility, created_by) VALUES(?,?,?,?,?)`,
		name, key, description, visibility, createdBy)
	if err != nil {
		return 0, err
	}
	// Seed the starter board for the new project.
	if err := s.seedDefaultColumns(id); err != nil {
		return 0, err
	}
	return id, nil
}

// cyrillic→latin transliteration for deriving project keys from a name.
var keyTranslit = map[rune]string{
	'а': "A", 'б': "B", 'в': "V", 'г': "G", 'д': "D", 'е': "E", 'ж': "ZH", 'з': "Z",
	'и': "I", 'й': "Y", 'к': "K", 'л': "L", 'м': "M", 'н': "N", 'о': "O", 'п': "P",
	'р': "R", 'с': "S", 'т': "T", 'у': "U", 'ф': "F", 'х': "H", 'ц': "C", 'ч': "CH",
	'ш': "SH", 'щ': "SCH", 'ъ': "", 'ы': "Y", 'ь': "", 'э': "E", 'ю': "YU", 'я': "YA",
}

// deriveProjectKey builds a short uppercase key from a name (e.g. "Команда A" → "KOMANDAA").
func deriveProjectKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r - 32) // uppercase ASCII
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		default:
			if t, ok := keyTranslit[r]; ok {
				b.WriteString(t)
			}
		}
		if b.Len() >= 6 {
			break
		}
	}
	key := b.String()
	if key == "" {
		key = "PRJ"
	}
	return key
}

// uniqueProjectKey returns a key not yet used by another project.
func (s *Store) uniqueProjectKey(name string) string {
	base := deriveProjectKey(name)
	key := base
	for n := 2; ; n++ {
		var cnt int
		s.db.QueryRow("SELECT COUNT(*) FROM projects WHERE key=?", key).Scan(&cnt)
		if cnt == 0 {
			return key
		}
		key = base + strconv.Itoa(n)
	}
}

func (s *Store) UpdateProject(id int64, name, description, visibility string) error {
	if visibility != "open" {
		visibility = "restricted"
	}
	_, err := s.db.Exec(`UPDATE projects SET name=?, description=?, visibility=? WHERE id=?`,
		name, description, visibility, id)
	return err
}

// DeleteProject removes a project. Foreign keys handle the dependent data:
// members, kanban (columns/epics/sprints/tags/tasks) and calendars cascade-delete
// (ON DELETE CASCADE), while wiki spaces are reparented to personal
// (project_id → NULL via ON DELETE SET NULL) so docs are never lost. The default
// project "Общее" (id 1) cannot be deleted.
func (s *Store) DeleteProject(id int64) error {
	if id == 1 {
		return fmt.Errorf("cannot delete the default project")
	}
	_, err := s.db.Exec(`DELETE FROM projects WHERE id=?`, id)
	return err
}

// === Project members ===

func (s *Store) ListProjectMembers(projectID int64) ([]model.ProjectMember, error) {
	rows, err := s.db.Query(`SELECT m.project_id, m.user_id, m.role, u.username
		FROM project_members m JOIN users u ON u.id = m.user_id
		WHERE m.project_id = ? ORDER BY u.username COLLATE NOCASE`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ProjectMember
	for rows.Next() {
		var m model.ProjectMember
		if err := rows.Scan(&m.ProjectID, &m.UserID, &m.Role, &m.Username); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if out == nil {
		out = []model.ProjectMember{}
	}
	return out, rows.Err()
}

func (s *Store) SetProjectMember(projectID, userID int64, role string) error {
	switch role {
	case "admin", "editor", "viewer":
	default:
		return fmt.Errorf("invalid role %q", role)
	}
	_, err := s.db.Exec(`INSERT INTO project_members(project_id, user_id, role) VALUES(?,?,?)
		ON CONFLICT(project_id, user_id) DO UPDATE SET role=excluded.role`, projectID, userID, role)
	return err
}

func (s *Store) RemoveProjectMember(projectID, userID int64) error {
	_, err := s.db.Exec(`DELETE FROM project_members WHERE project_id=? AND user_id=?`, projectID, userID)
	return err
}
