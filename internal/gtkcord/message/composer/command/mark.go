package command

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

// immutableMark is a mark that cannot be edited by the user. When the user
// attempts to delete it, the mark will be invalidated, and a callback will be
// called.
type immutableMark struct {
	marks   [2]*gtk.TextMark
	content string
	deleted func(start, end *gtk.TextIter)
}

func newImmutableMark(iter *gtk.TextIter, name string, deletedFunc func()) *immutableMark {
	buffer := iter.Buffer()

	m := &immutableMark{}
	m.marks[0] = buffer.CreateMark("__immutablemark_"+name, iter, true)
	m.marks[1] = buffer.CreateMark("__immutablemark_"+name, iter, false)

	m.deleted = func(start, end *gtk.TextIter) {
		if deletedFunc != nil {
			deletedFunc()
		}

		buffer := start.Buffer()
		buffer.Delete(start, end)
	}

	return m
}

// Iters returns the start and end iters of the immutableMark.
func (m *immutableMark) Iters() (start, end *gtk.TextIter) {
	buffer := m.marks[0].Buffer()
	return buffer.IterAtMark(m.marks[0]), buffer.IterAtMark(m.marks[1])
}

// Validate validates that the immutableMark is still valid.
func (m *immutableMark) Validate() {
	if m.marks[0].Deleted() || m.marks[1].Deleted() {
		m.deleted(m.Iters())
		return
	}

	buffer := m.marks[0].Buffer()
	start, end := m.Iters()

	content := buffer.Slice(start, end, true)
	if content != m.content {
		m.deleted(start, end)
		return
	}
}

// SetContent sets the content of the immutableMark.
func (m *immutableMark) SetContent(content string) {
	m.Validate()

	if m.content == content {
		return
	}

	m.content = content

	start, end := m.Iters()
	buffer := m.marks[0].Buffer()
	buffer.Delete(start, end)
	buffer.Insert(start, content)
}
