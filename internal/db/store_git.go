package db

import "kanban/internal/model"

// === Git integration store ===

// ListGitRepos returns the repositories configured for a project.
func (s *Store) ListGitRepos(projectID int64) ([]model.GitRepo, error) {
	rows, err := s.db.Query(`SELECT id, project_id, kind, label, repo, secret, move_column_id, created_at
		FROM git_repos WHERE project_id=? ORDER BY id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.GitRepo{}
	for rows.Next() {
		var r model.GitRepo
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Kind, &r.Label, &r.Repo, &r.Secret, &r.MoveColumnID, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AnyGitRepos reports whether any repositories are configured (for showing the
// task-binding UI only when integration exists).
func (s *Store) AnyGitRepos() bool {
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM git_repos`).Scan(&n)
	return n > 0
}

// GetGitRepo returns one repository by id.
func (s *Store) GetGitRepo(id int64) (*model.GitRepo, error) {
	var r model.GitRepo
	err := s.db.QueryRow(`SELECT id, project_id, kind, label, repo, secret, move_column_id, created_at
		FROM git_repos WHERE id=?`, id).
		Scan(&r.ID, &r.ProjectID, &r.Kind, &r.Label, &r.Repo, &r.Secret, &r.MoveColumnID, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// CreateGitRepo adds a repository.
func (s *Store) CreateGitRepo(projectID int64, kind, label, repo, secret string, moveColumnID *int64) (int64, error) {
	return s.db.ExecInsert(`INSERT INTO git_repos(project_id, kind, label, repo, secret, move_column_id)
		VALUES(?,?,?,?,?,?)`, projectID, kind, label, repo, secret, moveColumnID)
}

// UpdateGitRepo updates a repository's editable fields.
func (s *Store) UpdateGitRepo(id int64, label, repo, secret string, moveColumnID *int64) error {
	_, err := s.db.Exec(`UPDATE git_repos SET label=?, repo=?, secret=?, move_column_id=? WHERE id=?`,
		label, repo, secret, moveColumnID, id)
	return err
}

// DeleteGitRepo removes a repository (and cascades its events).
func (s *Store) DeleteGitRepo(id int64) error {
	_, err := s.db.Exec(`DELETE FROM git_repos WHERE id=?`, id)
	return err
}

// ProjectKey returns a project's short key (for KEY-<n> matching). Empty on error.
func (s *Store) ProjectKey(projectID int64) string {
	var k string
	s.db.QueryRow(`SELECT key FROM projects WHERE id=?`, projectID).Scan(&k)
	return k
}

// GetTaskIDByNumber resolves a per-project task number (KEY-<n>) to a task id.
func (s *Store) GetTaskIDByNumber(projectID int64, number int) (int64, bool) {
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM tasks WHERE project_id=? AND number=?`, projectID, number).Scan(&id); err != nil {
		return 0, false
	}
	return id, true
}

// AddGitEvent records a normalized git event linked to a task.
func (s *Store) AddGitEvent(e model.GitEvent) (int64, error) {
	return s.db.ExecInsert(`INSERT INTO git_events(repo_id, task_id, kind, title, ref, url, status, author)
		VALUES(?,?,?,?,?,?,?,?)`, e.RepoID, e.TaskID, e.Kind, e.Title, e.Ref, e.URL, e.Status, e.Author)
}

// ListGitEventsForTask returns recent git events for a task (newest first).
func (s *Store) ListGitEventsForTask(taskID int64, limit int) ([]model.GitEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.db.Query(`SELECT e.id, e.repo_id, e.task_id, e.kind, e.title, e.ref, e.url, e.status, e.author, e.created_at,
		COALESCE(r.label, r.repo)
		FROM git_events e LEFT JOIN git_repos r ON r.id = e.repo_id
		WHERE e.task_id=? ORDER BY e.id DESC LIMIT ?`, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.GitEvent{}
	for rows.Next() {
		var e model.GitEvent
		if err := rows.Scan(&e.ID, &e.RepoID, &e.TaskID, &e.Kind, &e.Title, &e.Ref, &e.URL, &e.Status, &e.Author, &e.CreatedAt, &e.RepoLabel); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
