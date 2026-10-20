//go:build windows

package main

import (
	"sort"
	"strings"
	"time"

	"arsivacici/arc"
)

// node is one folder or file in the browsable tree built from the flat
// entry list. Folders that only exist implicitly (a/b/c.txt without an
// "a/b/" entry) are created here.
type node struct {
	name     string
	path     string // full path inside the archive; folders end with '/'
	dir      bool
	entry    *arc.Entry
	parent   *node
	children map[string]*node
	size     int64
	packed   int64
	mod      time.Time
	files    int
}

func buildTree(entries []*arc.Entry) *node {
	root := &node{dir: true, children: map[string]*node{}}
	for _, e := range entries {
		parts := strings.Split(strings.TrimSuffix(e.Path, "/"), "/")
		cur := root
		for i, p := range parts {
			if p == "" || p == "." {
				continue
			}
			last := i == len(parts)-1
			isDir := !last || e.IsDir
			key := p
			if !isDir {
				key = "\x00" + p // files and folders may share a name
			}
			child := cur.children[key]
			if child == nil {
				child = &node{name: p, dir: isDir, parent: cur}
				if isDir {
					child.children = map[string]*node{}
					child.path = cur.path + p + "/"
				} else {
					child.path = cur.path + p
				}
				cur.children[key] = child
			}
			if last {
				if !isDir {
					child.entry = e
					child.size = e.Size
					child.packed = e.Packed
					child.mod = e.Modified
				} else if child.mod.IsZero() {
					child.mod = e.Modified
				}
			}
			cur = child
		}
	}
	root.sum()
	return root
}

// sum fills in folder totals.
func (n *node) sum() {
	if !n.dir {
		n.files = 1
		return
	}
	n.size, n.files = 0, 0
	var packed int64
	packedKnown := true
	for _, c := range n.children {
		c.sum()
		n.size += c.size
		n.files += c.files
		if c.packed < 0 {
			packedKnown = false
		}
		packed += c.packed
		if c.mod.After(n.mod) && n.entry == nil {
			n.mod = c.mod
		}
	}
	if packedKnown {
		n.packed = packed
	} else {
		n.packed = -1
	}
}

func (n *node) sortedChildren() []*node {
	out := make([]*node, 0, len(n.children))
	for _, c := range n.children {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].dir != out[j].dir {
			return out[i].dir
		}
		return strings.ToLower(out[i].name) < strings.ToLower(out[j].name)
	})
	return out
}

// find returns the folder node at path ("" = root), or nil.
func (n *node) find(path string) *node {
	cur := n
	for _, p := range strings.Split(strings.TrimSuffix(path, "/"), "/") {
		if p == "" {
			continue
		}
		next := cur.children[p]
		if next == nil || !next.dir {
			return nil
		}
		cur = next
	}
	return cur
}

// countDirs counts all folders below n.
func (n *node) countDirs() int {
	c := 0
	for _, ch := range n.children {
		if ch.dir {
			c += 1 + ch.countDirs()
		}
	}
	return c
}
