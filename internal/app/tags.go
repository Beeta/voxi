package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Beeta/voxi/internal/store"
)

// Tag is a label the user puts on recordings, any number to each, to
// filter the sidebar by. Recordings hold its ID, so renaming a tag is one
// write to the settings rather than a rewrite of every recording carrying
// it.
type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// tagByID returns the tag with id among tags.
func tagByID(tags []Tag, id string) (Tag, bool) {
	for _, t := range tags {
		if t.ID == id {
			return t, true
		}
	}
	return Tag{}, false
}

// activeFilter is the tag the sidebar filters by, or "" for all. A filter
// naming a tag that no longer exists shows all, so a settings file edited
// by hand, or one an older build wrote back, cannot hide every recording.
func activeFilter(s Settings) string {
	if _, ok := tagByID(s.Tags, s.TagFilter); ok {
		return s.TagFilter
	}
	return ""
}

// filterRecs is the recordings carrying tag, or all of them when tag is "".
func filterRecs(recs []store.Recording, tag string) []store.Recording {
	if tag == "" {
		return recs
	}
	var out []store.Recording
	for _, r := range recs {
		if slices.Contains(r.Tags, tag) {
			out = append(out, r)
		}
	}
	return out
}

// tagCounts counts the recordings carrying each tag.
func tagCounts(recs []store.Recording) map[string]int {
	n := map[string]int{}
	for _, r := range recs {
		for _, t := range r.Tags {
			n[t]++
		}
	}
	return n
}

// tagName is the name a tag is saved under: trimmed, and empty when there
// is nothing but space, which the editor refuses.
func tagName(s string) string { return strings.TrimSpace(s) }

// tagTaken reports whether another tag than id is named name already: two
// tags of one name would be told apart only by which recordings they hold.
func tagTaken(tags []Tag, id, name string) bool {
	for _, t := range tags {
		if t.ID != id && t.Name == name {
			return true
		}
	}
	return false
}

// newTagID is an ID for a tag made now.
func newTagID() string { return fmt.Sprintf("tag-%d", time.Now().UnixNano()) }

// setFilter shows the recordings of tag, or all for "", and remembers it.
func (a *app) setFilter(tag string) {
	a.settings.update(func(s *Settings) { s.TagFilter = tag })
}

// showAll clears the filter, as when a new recording or an import arrives
// without the tag shown: it would otherwise be selected but out of sight.
func (a *app) showAll() {
	if activeFilter(a.settings.snapshot()) != "" {
		a.setFilter("")
	}
}

// openTagEditor opens the tag dialog on tag id, or on a new tag when id is
// "", which goes on recording forRec when that is set.
func (a *app) openTagEditor(id, forRec string) {
	a.tagEditID, a.tagEditFor, a.tagEditName = id, forRec, ""
	if t, ok := tagByID(a.settings.snapshot().Tags, id); ok {
		a.tagEditName = t.Name
	}
	a.tagMenuOpen = false
	a.tagEditOpen = true
}

// saveTag writes the editor's draft: renaming the tag it was opened on, or
// making a new one, put on the recording the editor was opened for. It
// reports false, leaving everything as it was, for an empty name or one
// another tag has.
func (a *app) saveTag() bool {
	name := tagName(a.tagEditName)
	tags := a.settings.snapshot().Tags
	if name == "" || tagTaken(tags, a.tagEditID, name) {
		return false
	}
	if a.tagEditID != "" {
		id := a.tagEditID
		a.settings.update(func(s *Settings) {
			if i := slices.IndexFunc(s.Tags, func(t Tag) bool { return t.ID == id }); i >= 0 {
				s.Tags = slices.Clone(s.Tags)
				s.Tags[i].Name = name
			}
		})
		return true
	}
	id := newTagID()
	a.settings.update(func(s *Settings) { s.Tags = append(slices.Clip(s.Tags), Tag{ID: id, Name: name}) })
	if a.tagEditFor != "" {
		a.lib.SetTagged(a.tagEditFor, id, true)
	}
	return true
}

// deleteTag deletes tag id: it comes off every recording carrying it, the
// recordings themselves stay, and a filter on it goes back to all.
func (a *app) deleteTag(id string) {
	a.lib.RemoveTag(id)
	a.settings.update(func(s *Settings) {
		s.Tags = slices.DeleteFunc(slices.Clone(s.Tags), func(t Tag) bool { return t.ID == id })
		if s.TagFilter == id {
			s.TagFilter = ""
		}
	})
}
