//go:build windows

package main

import (
	"sort"
	"strings"

	"github.com/lxn/walk"
)

const (
	colName = iota
	colSize
	colPacked
	colType
	colDate
)

type row struct {
	n  *node
	up bool // the ".." row
}

type listModel struct {
	walk.TableModelBase
	walk.SorterBase
	rows      []*row
	sortCol   int
	sortOrder walk.SortOrder
}

func (m *listModel) RowCount() int { return len(m.rows) }

func (m *listModel) Value(i, col int) interface{} {
	r := m.rows[i]
	if r.up {
		if col == colName {
			return ".."
		}
		if col == colType {
			return "Üst klasör"
		}
		return ""
	}
	n := r.n
	switch col {
	case colName:
		return n.name
	case colSize:
		return fmtSize(n.size)
	case colPacked:
		if n.packed < 0 {
			return ""
		}
		return fmtSize(n.packed)
	case colType:
		t := typeName(n.name, n.dir)
		if n.entry != nil && n.entry.Encrypted {
			t += " (şifreli)"
		}
		return t
	case colDate:
		return fmtDate(n.mod)
	}
	return ""
}

func (m *listModel) Image(i int) interface{} {
	r := m.rows[i]
	var ic *walk.Icon
	if r.up {
		ic = shellIcon("", true)
	} else {
		ic = shellIcon(r.n.name, r.n.dir)
	}
	if ic == nil {
		return nil // never a typed nil inside the interface
	}
	return ic
}

func (m *listModel) Sort(col int, order walk.SortOrder) error {
	m.sortCol, m.sortOrder = col, order
	m.sortRows()
	m.PublishRowsReset()
	return m.SorterBase.Sort(col, order)
}

func (m *listModel) sortRows() {
	col, asc := m.sortCol, m.sortOrder == walk.SortAscending
	sort.SliceStable(m.rows, func(i, j int) bool {
		a, b := m.rows[i], m.rows[j]
		if a.up != b.up {
			return a.up
		}
		if a.up {
			return false
		}
		if a.n.dir != b.n.dir {
			return a.n.dir
		}
		var less, equal bool
		switch col {
		case colSize:
			less, equal = a.n.size < b.n.size, a.n.size == b.n.size
		case colPacked:
			less, equal = a.n.packed < b.n.packed, a.n.packed == b.n.packed
		case colType:
			ta, tb := typeName(a.n.name, a.n.dir), typeName(b.n.name, b.n.dir)
			less, equal = ta < tb, ta == tb
		case colDate:
			less, equal = a.n.mod.Before(b.n.mod), a.n.mod.Equal(b.n.mod)
		}
		if col == colName || equal {
			na, nb := strings.ToLower(a.n.name), strings.ToLower(b.n.name)
			if col == colName {
				if asc {
					return na < nb
				}
				return na > nb
			}
			return na < nb
		}
		if asc {
			return less
		}
		return !less
	})
}

func (m *listModel) setFolder(n *node) {
	m.rows = m.rows[:0]
	if n.parent != nil {
		m.rows = append(m.rows, &row{up: true})
	}
	for _, c := range n.children {
		m.rows = append(m.rows, &row{n: c})
	}
	m.sortRows()
	m.PublishRowsReset()
}

func (m *listModel) clear() {
	m.rows = nil
	m.PublishRowsReset()
}
