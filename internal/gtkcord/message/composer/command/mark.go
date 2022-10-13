package command

import (
	"log"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// immutableMark is a mark that cannot be edited by the user. When the user
// attempts to delete it, the mark will be invalidated, and a callback will be
// called.
type immutableMark struct {
	marks       [2]*gtk.TextMark
	content     string
	deletedFunc func()
	deleted     bool
}

func newImmutableMark(iter *gtk.TextIter, deletedFunc func()) *immutableMark {
	buffer := iter.Buffer()

	return &immutableMark{
		marks: [2]*gtk.TextMark{
			buffer.CreateMark("", iter, false),
			buffer.CreateMark("", iter, true),
		},
		deletedFunc: deletedFunc,
	}
}

// Iters returns the start and end iters of the immutableMark.
func (m *immutableMark) Iters() (start, end *gtk.TextIter) {
	buffer := m.marks[0].Buffer()
	return buffer.IterAtMark(m.marks[0]), buffer.IterAtMark(m.marks[1])
}

// IsDeleted returns whether the immutableMark is deleted.
func (m *immutableMark) IsDeleted() bool {
	return m.deleted || m.marks[0].Deleted() || m.marks[1].Deleted()
}

// Delete deletes the mark. The immutableMark is invalid after calling this
// function.
func (m *immutableMark) Delete() {
	if m.deleted {
		return
	}

	m.deleted = true

	buffer := m.marks[0].Buffer()

	start, end := m.Iters()
	buffer.DeleteMark(m.marks[0])
	buffer.DeleteMark(m.marks[1])
	buffer.Delete(start, end)

	if m.deletedFunc != nil {
		m.deletedFunc()
	}
}

// Validate validates that the immutableMark is still valid.
func (m *immutableMark) Validate() bool {
	if m.IsDeleted() {
		log.Println("immutableMark is deleted")
		return false
	}

	buffer := m.marks[0].Buffer()
	start, end := m.Iters()

	content := buffer.Slice(start, end, true)
	if content != m.content {
		log.Printf("immutableMark content mismatch: %q != %q", content, m.content)
		m.Delete()
		return false
	}

	return true
}

// SetContent sets the content of the immutableMark. The content will be
// suffixed with a space.
func (m *immutableMark) SetContent(content string) {
	if !m.Validate() {
		return
	}

	if m.content == content {
		return
	}

	m.content = content

	start, end := m.Iters()
	buffer := m.marks[0].Buffer()
	buffer.Delete(start, end)
	buffer.Insert(end, content+" ") // this invalidates the start iter

	// Undo the space.
	end.BackwardChar()
	// Revalidate the start iter.
	start = buffer.IterAtOffset(end.Offset() - len(content))

	log.Printf("set content to %q", buffer.Slice(start, end, true))
	buffer.MoveMark(m.marks[0], start)
	buffer.MoveMark(m.marks[1], end)
}
