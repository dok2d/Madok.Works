package mindmap

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Op is one edit of the map, as accepted by the ops API and used internally by
// the node endpoints. Field use by op:
//
//	add          parent_id (required), index (optional, append by default),
//	             node (subtree to insert; text alone is fine)
//	update       id, set (partial node fields as a JSON object; null clears)
//	delete       id (root cannot be deleted)
//	move         id, parent_id, index (optional), side (optional)
//	link_add     link {from, to, label}
//	link_update  id, link {label}
//	link_delete  id
//	settings     layout / line / palette (any subset)
type Op struct {
	Op       string          `json:"op"`
	ID       string          `json:"id,omitempty"`
	ParentID string          `json:"parent_id,omitempty"`
	Index    *int            `json:"index,omitempty"`
	Side     string          `json:"side,omitempty"`
	Node     *Node           `json:"node,omitempty"`
	Set      json.RawMessage `json:"set,omitempty"`
	Link     *Link           `json:"link,omitempty"`
	Layout   string          `json:"layout,omitempty"`
	Line     string          `json:"line,omitempty"`
	Palette  string          `json:"palette,omitempty"`
}

// OpResult reports what an op produced (the id of an added node or link).
type OpResult struct {
	Op string `json:"op"`
	ID string `json:"id,omitempty"`
}

// Apply runs the ops in order against the document. The document is modified
// in place only if every op succeeds (the caller keeps the original otherwise).
func (d *Doc) Apply(ops []Op) ([]OpResult, error) {
	if len(ops) == 0 {
		return nil, errors.New("no operations")
	}
	if len(ops) > 1000 {
		return nil, errors.New("too many operations (max 1000)")
	}
	work := d.Clone()
	if work == nil {
		return nil, errors.New("bad document")
	}
	results := make([]OpResult, 0, len(ops))
	for i, op := range ops {
		res, err := work.apply(op)
		if err != nil {
			return nil, fmt.Errorf("op %d (%s): %w", i, op.Op, err)
		}
		results = append(results, res)
	}
	if err := work.Normalize(); err != nil {
		return nil, err
	}
	*d = *work
	return results, nil
}

func (d *Doc) apply(op Op) (OpResult, error) {
	switch strings.ToLower(strings.TrimSpace(op.Op)) {
	case "add":
		return d.opAdd(op)
	case "update":
		return d.opUpdate(op)
	case "delete":
		return d.opDelete(op)
	case "move":
		return d.opMove(op)
	case "link_add":
		if op.Link == nil {
			return OpResult{}, errors.New("link required")
		}
		from, _ := d.Find(op.Link.From)
		to, _ := d.Find(op.Link.To)
		if from == nil || to == nil || from == to {
			return OpResult{}, errors.New("link endpoints not found")
		}
		if len(d.Links) >= MaxLinks {
			return OpResult{}, errors.New("too many links")
		}
		l := Link{ID: NewID(), From: op.Link.From, To: op.Link.To, Label: op.Link.Label}
		d.Links = append(d.Links, l)
		return OpResult{Op: "link_add", ID: l.ID}, nil
	case "link_update":
		for i := range d.Links {
			if d.Links[i].ID == op.ID {
				if op.Link != nil {
					d.Links[i].Label = op.Link.Label
				}
				return OpResult{Op: "link_update", ID: op.ID}, nil
			}
		}
		return OpResult{}, errors.New("link not found")
	case "link_delete":
		for i := range d.Links {
			if d.Links[i].ID == op.ID {
				d.Links = append(d.Links[:i], d.Links[i+1:]...)
				return OpResult{Op: "link_delete", ID: op.ID}, nil
			}
		}
		return OpResult{}, errors.New("link not found")
	case "settings":
		if op.Layout != "" {
			d.Layout = op.Layout
		}
		if op.Line != "" {
			d.Line = op.Line
		}
		if op.Palette != "" {
			d.Palette = op.Palette
		}
		return OpResult{Op: "settings"}, nil
	}
	return OpResult{}, errors.New("unknown op")
}

func (d *Doc) opAdd(op Op) (OpResult, error) {
	if op.ParentID == "" {
		op.ParentID = d.Root.ID
	}
	parent, _ := d.Find(op.ParentID)
	if parent == nil {
		return OpResult{}, errors.New("parent not found")
	}
	n := op.Node
	if n == nil {
		n = &Node{}
	}
	// Ids inside the inserted subtree are regenerated so a pasted copy never
	// collides with the existing map.
	n = CloneNode(n)
	if strings.TrimSpace(n.Text) == "" {
		n.Text = ""
	}
	if d.Count()+countNodes(n) > MaxNodes {
		return OpResult{}, fmt.Errorf("too many nodes (max %d)", MaxNodes)
	}
	if op.Side != "" {
		n.Side = op.Side
	}
	insertChild(parent, n, op.Index)
	return OpResult{Op: "add", ID: n.ID}, nil
}

func (d *Doc) opUpdate(op Op) (OpResult, error) {
	n, parent := d.Find(op.ID)
	if n == nil {
		return OpResult{}, errors.New("node not found")
	}
	if len(op.Set) == 0 {
		return OpResult{}, errors.New("set required")
	}
	// Merge: existing node (minus children) → map → overlay the patch → node.
	base := *n
	base.Children = nil
	raw, _ := json.Marshal(base)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return OpResult{}, err
	}
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(op.Set, &patch); err != nil {
		return OpResult{}, errors.New("set must be an object")
	}
	for k, v := range patch {
		switch k {
		case "id", "children":
			continue // immutable here
		}
		if string(v) == "null" {
			delete(m, k)
		} else {
			m[k] = v
		}
	}
	merged, _ := json.Marshal(m)
	var upd Node
	if err := json.Unmarshal(merged, &upd); err != nil {
		return OpResult{}, fmt.Errorf("bad field: %w", err)
	}
	upd.ID = n.ID
	upd.Children = n.Children
	if parent == nil {
		upd.Side = ""
		upd.Collapsed = false
	}
	*n = upd
	return OpResult{Op: "update", ID: n.ID}, nil
}

func (d *Doc) opDelete(op Op) (OpResult, error) {
	n, parent := d.Find(op.ID)
	if n == nil {
		return OpResult{}, errors.New("node not found")
	}
	if parent == nil {
		return OpResult{}, errors.New("root cannot be deleted")
	}
	removeChild(parent, n)
	// Drop links into the removed subtree.
	gone := map[string]bool{}
	var mark func(x *Node)
	mark = func(x *Node) {
		gone[x.ID] = true
		for _, c := range x.Children {
			mark(c)
		}
	}
	mark(n)
	kept := d.Links[:0]
	for _, l := range d.Links {
		if !gone[l.From] && !gone[l.To] {
			kept = append(kept, l)
		}
	}
	d.Links = kept
	return OpResult{Op: "delete", ID: op.ID}, nil
}

func (d *Doc) opMove(op Op) (OpResult, error) {
	n, parent := d.Find(op.ID)
	if n == nil {
		return OpResult{}, errors.New("node not found")
	}
	if parent == nil {
		return OpResult{}, errors.New("root cannot be moved")
	}
	targetID := op.ParentID
	if targetID == "" {
		targetID = parent.ID
	}
	if d.IsDescendant(targetID, n.ID) {
		return OpResult{}, errors.New("cannot move a node under its own descendant")
	}
	target, _ := d.Find(targetID)
	if target == nil {
		return OpResult{}, errors.New("target parent not found")
	}
	// When re-inserting among the same siblings, compute the index on the list
	// without the moving node so "index" means the final position.
	removeChild(parent, n)
	if op.Side != "" {
		n.Side = op.Side
	}
	if target != d.Root {
		n.Side = ""
	}
	insertChild(target, n, op.Index)
	return OpResult{Op: "move", ID: n.ID}, nil
}

func insertChild(parent, n *Node, index *int) {
	if index == nil || *index < 0 || *index >= len(parent.Children) {
		parent.Children = append(parent.Children, n)
		return
	}
	i := *index
	parent.Children = append(parent.Children, nil)
	copy(parent.Children[i+1:], parent.Children[i:])
	parent.Children[i] = n
}

func removeChild(parent, n *Node) {
	for i, c := range parent.Children {
		if c == n {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return
		}
	}
}

func countNodes(n *Node) int {
	c := 1
	for _, k := range n.Children {
		c += countNodes(k)
	}
	return c
}
