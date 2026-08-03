package db

import (
	"errors"
	"fmt"
	"strings"

	"kanban/internal/model"
)

// === Shared file storage ("Files" service) ===
//
// Folder tree in storage_folders, files in storage_items; the blobs themselves
// live in the shared files table (SaveFile/GetFile/DeleteFile) so upload policy,
// compression, export and GC apply unchanged. folder_id NULL means the root.

// Sentinel errors the handler maps to 409.
var (
	ErrStorageNameTaken = errors.New("storage: name already taken")
	ErrStorageCycle     = errors.New("storage: folder move would create a cycle")
)

// folderCond builds "folder_id = ?" / "folder_id IS NULL" (NULL never matches =).
func folderCond(col string, id *int64) (string, []any) {
	if id == nil {
		return col + " IS NULL", nil
	}
	return col + " = ?", []any{*id}
}

// storageNameTaken reports whether name (case-insensitive) is already used by a
// folder or an item inside the given parent folder, ignoring the given ids.
func (s *Store) storageNameTaken(parent *int64, name string, skipFolder, skipItem int64) (bool, error) {
	lower := strings.ToLower(name)
	cond, args := folderCond("parent_id", parent)
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM storage_folders WHERE "+cond+" AND LOWER(name) = ? AND id != ?",
		append(args, lower, skipFolder)...).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	cond, args = folderCond("folder_id", parent)
	if err := s.db.QueryRow("SELECT COUNT(*) FROM storage_items WHERE "+cond+" AND LOWER(name) = ? AND id != ?",
		append(args, lower, skipItem)...).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// ListStorageFolders returns the whole folder tree as a flat list (name order);
// the client builds breadcrumbs and the move picker from it.
func (s *Store) ListStorageFolders() ([]model.StorageFolder, error) {
	rows, err := s.db.Query(`SELECT id, parent_id, name, created_by, created_at
		FROM storage_folders ORDER BY LOWER(name), id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.StorageFolder{}
	for rows.Next() {
		var f model.StorageFolder
		if err := rows.Scan(&f.ID, &f.ParentID, &f.Name, &f.CreatedBy, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) GetStorageFolder(id int64) (*model.StorageFolder, error) {
	var f model.StorageFolder
	err := s.db.QueryRow(`SELECT id, parent_id, name, created_by, created_at
		FROM storage_folders WHERE id = ?`, id).
		Scan(&f.ID, &f.ParentID, &f.Name, &f.CreatedBy, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *Store) CreateStorageFolder(name string, parentID *int64, createdBy int64) (int64, error) {
	if taken, err := s.storageNameTaken(parentID, name, 0, 0); err != nil {
		return 0, err
	} else if taken {
		return 0, ErrStorageNameTaken
	}
	return s.db.ExecInsert(`INSERT INTO storage_folders(parent_id, name, created_by) VALUES(?,?,?)`,
		parentID, name, createdBy)
}

func (s *Store) RenameStorageFolder(id int64, name string) error {
	f, err := s.GetStorageFolder(id)
	if err != nil {
		return err
	}
	if taken, err := s.storageNameTaken(f.ParentID, name, id, 0); err != nil {
		return err
	} else if taken {
		return ErrStorageNameTaken
	}
	_, err = s.db.Exec(`UPDATE storage_folders SET name=? WHERE id=?`, name, id)
	return err
}

// MoveStorageFolder re-parents a folder after checking the new parent is not
// the folder itself or one of its descendants.
func (s *Store) MoveStorageFolder(id int64, newParent *int64) error {
	f, err := s.GetStorageFolder(id)
	if err != nil {
		return err
	}
	if newParent != nil {
		if *newParent == id {
			return ErrStorageCycle
		}
		parents, err := s.storageParents()
		if err != nil {
			return err
		}
		for p := newParent; p != nil; {
			if *p == id {
				return ErrStorageCycle
			}
			next, ok := parents[*p]
			if !ok {
				break
			}
			p = next
		}
	}
	if taken, err := s.storageNameTaken(newParent, f.Name, id, 0); err != nil {
		return err
	} else if taken {
		return ErrStorageNameTaken
	}
	_, err = s.db.Exec(`UPDATE storage_folders SET parent_id=? WHERE id=?`, newParent, id)
	return err
}

// storageParents reads the whole id->parent map in one query (single SQLite
// connection: read everything before any follow-up statement).
func (s *Store) storageParents() (map[int64]*int64, error) {
	rows, err := s.db.Query(`SELECT id, parent_id FROM storage_folders`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	parents := map[int64]*int64{}
	for rows.Next() {
		var id int64
		var parent *int64
		if err := rows.Scan(&id, &parent); err != nil {
			return nil, err
		}
		parents[id] = parent
	}
	return parents, rows.Err()
}

// DeleteStorageFolder removes a folder subtree. Child rows go via ON DELETE
// CASCADE; the orphaned blobs are deleted explicitly right away (not left for
// GC). Returns the number of files removed.
func (s *Store) DeleteStorageFolder(id int64) (int, error) {
	parents, err := s.storageParents()
	if err != nil {
		return 0, err
	}
	if _, ok := parents[id]; !ok {
		return 0, fmt.Errorf("storage folder %d not found", id)
	}
	// Collect the subtree by walking each folder's parent chain up to id.
	subtree := []any{id}
	for fid := range parents {
		for p := parents[fid]; p != nil; {
			if *p == id {
				subtree = append(subtree, fid)
				break
			}
			p = parents[*p]
		}
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(subtree)), ",")
	rows, err := s.db.Query(`SELECT file_id FROM storage_items WHERE folder_id IN (`+ph+`)`, subtree...)
	if err != nil {
		return 0, err
	}
	var fileIDs []int64
	for rows.Next() {
		var fid int64
		rows.Scan(&fid)
		fileIDs = append(fileIDs, fid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if _, err := s.db.Exec(`DELETE FROM storage_folders WHERE id=?`, id); err != nil {
		return 0, err
	}
	for _, fid := range fileIDs {
		s.DeleteFile(fid)
	}
	return len(fileIDs), nil
}

const storageItemCols = `i.id, i.folder_id, i.file_id, i.name, i.public_token, i.uploaded_by,
	COALESCE(u.username,''), i.created_at, f.mime, f.size`

func scanStorageItem(rows interface{ Scan(...any) error }) (*model.StorageItem, error) {
	var it model.StorageItem
	if err := rows.Scan(&it.ID, &it.FolderID, &it.FileID, &it.Name, &it.PublicToken,
		&it.UploadedBy, &it.UploaderName, &it.CreatedAt, &it.Mime, &it.Size); err != nil {
		return nil, err
	}
	return &it, nil
}

// ListStorageItems returns the files of one folder (nil = root), name order.
func (s *Store) ListStorageItems(folderID *int64) ([]model.StorageItem, error) {
	cond, args := folderCond("i.folder_id", folderID)
	rows, err := s.db.Query(`SELECT `+storageItemCols+`
		FROM storage_items i
		JOIN files f ON f.id = i.file_id
		LEFT JOIN users u ON u.id = i.uploaded_by
		WHERE `+cond+` ORDER BY LOWER(i.name), i.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.StorageItem{}
	for rows.Next() {
		it, err := scanStorageItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *it)
	}
	return out, rows.Err()
}

func (s *Store) GetStorageItem(id int64) (*model.StorageItem, error) {
	return s.getStorageItemBy("i.id = ?", id)
}

// GetStorageItemByToken resolves a public share link; empty tokens never match.
func (s *Store) GetStorageItemByToken(token string) (*model.StorageItem, error) {
	if token == "" {
		return nil, errors.New("empty token")
	}
	return s.getStorageItemBy("i.public_token = ?", token)
}

func (s *Store) getStorageItemBy(cond string, arg any) (*model.StorageItem, error) {
	row := s.db.QueryRow(`SELECT `+storageItemCols+`
		FROM storage_items i
		JOIN files f ON f.id = i.file_id
		LEFT JOIN users u ON u.id = i.uploaded_by
		WHERE `+cond, arg)
	return scanStorageItem(row)
}

// CreateStorageItem registers an uploaded blob in a folder. A clashing name
// gets an automatic " (2)"-style suffix (before the extension) instead of an
// error, mirroring what desktop file managers do on upload.
func (s *Store) CreateStorageItem(folderID *int64, fileID int64, name string, uploadedBy int64) (int64, string, error) {
	final := name
	for n := 2; ; n++ {
		taken, err := s.storageNameTaken(folderID, final, 0, 0)
		if err != nil {
			return 0, "", err
		}
		if !taken {
			break
		}
		if n > 200 {
			return 0, "", ErrStorageNameTaken
		}
		final = suffixName(name, n)
	}
	id, err := s.db.ExecInsert(`INSERT INTO storage_items(folder_id, file_id, name, uploaded_by) VALUES(?,?,?,?)`,
		folderID, fileID, final, uploadedBy)
	return id, final, err
}

// suffixName turns "report.pdf" into "report (2).pdf" (extension preserved).
func suffixName(name string, n int) string {
	base, ext := name, ""
	if dot := strings.LastIndex(name, "."); dot > 0 {
		base, ext = name[:dot], name[dot:]
	}
	return fmt.Sprintf("%s (%d)%s", base, n, ext)
}

func (s *Store) RenameStorageItem(id int64, name string) error {
	it, err := s.GetStorageItem(id)
	if err != nil {
		return err
	}
	if taken, err := s.storageNameTaken(it.FolderID, name, 0, id); err != nil {
		return err
	} else if taken {
		return ErrStorageNameTaken
	}
	_, err = s.db.Exec(`UPDATE storage_items SET name=? WHERE id=?`, name, id)
	return err
}

func (s *Store) MoveStorageItem(id int64, folderID *int64) error {
	it, err := s.GetStorageItem(id)
	if err != nil {
		return err
	}
	if taken, err := s.storageNameTaken(folderID, it.Name, 0, id); err != nil {
		return err
	} else if taken {
		return ErrStorageNameTaken
	}
	_, err = s.db.Exec(`UPDATE storage_items SET folder_id=? WHERE id=?`, folderID, id)
	return err
}

// DeleteStorageItem removes the item and its blob immediately.
func (s *Store) DeleteStorageItem(id int64) error {
	it, err := s.GetStorageItem(id)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM storage_items WHERE id=?`, id); err != nil {
		return err
	}
	return s.DeleteFile(it.FileID)
}

// SetStorageItemToken sets ('' revokes) an item's public share token.
func (s *Store) SetStorageItemToken(id int64, token string) error {
	_, err := s.db.Exec(`UPDATE storage_items SET public_token=? WHERE id=?`, token, id)
	return err
}

// StorageUsage returns the number of stored files and their total (uncompressed) size.
func (s *Store) StorageUsage() (count int64, total int64) {
	s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(f.size),0)
		FROM storage_items i JOIN files f ON f.id = i.file_id`).Scan(&count, &total)
	return
}

// SearchStorage finds folders and files by name (all terms must match).
func (s *Store) SearchStorage(query string, limit int) ([]model.StorageFolder, []model.StorageItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	where, wargs := termAndOrWhere([]string{"LOWER(name)"}, query)
	rows, err := s.db.Query(`SELECT id, parent_id, name, created_by, created_at
		FROM storage_folders WHERE `+where+` ORDER BY LOWER(name) LIMIT ?`,
		append(wargs, limit)...)
	if err != nil {
		return nil, nil, err
	}
	folders := []model.StorageFolder{}
	for rows.Next() {
		var f model.StorageFolder
		if err := rows.Scan(&f.ID, &f.ParentID, &f.Name, &f.CreatedBy, &f.CreatedAt); err != nil {
			rows.Close()
			return nil, nil, err
		}
		folders = append(folders, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	where, wargs = termAndOrWhere([]string{"LOWER(i.name)"}, query)
	rows, err = s.db.Query(`SELECT `+storageItemCols+`
		FROM storage_items i
		JOIN files f ON f.id = i.file_id
		LEFT JOIN users u ON u.id = i.uploaded_by
		WHERE `+where+` ORDER BY LOWER(i.name) LIMIT ?`,
		append(wargs, limit)...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := []model.StorageItem{}
	for rows.Next() {
		it, err := scanStorageItem(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, *it)
	}
	return folders, items, rows.Err()
}
