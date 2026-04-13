package core

import "testing"

// TestNumberedScheme_Format verifies the traditional NN-slug.html format
// that slyds has used since day one. Regression guard — any change here
// breaks all existing decks.
func TestNumberedScheme_Format(t *testing.T) {
	s := NumberedScheme{}
	cases := []struct {
		pos  int
		slug string
		want string
	}{
		{1, "title", "01-title.html"},
		{3, "intro", "03-intro.html"},
		{12, "metrics", "12-metrics.html"},
		{99, "closing", "99-closing.html"},
	}
	for _, c := range cases {
		got := s.Format(c.pos, c.slug)
		if got != c.want {
			t.Errorf("NumberedScheme.Format(%d, %q) = %q, want %q", c.pos, c.slug, got, c.want)
		}
	}
}

// TestNumberedScheme_ExtractSlug verifies slug extraction from numbered
// filenames — strips the NN- prefix and .html suffix.
func TestNumberedScheme_ExtractSlug(t *testing.T) {
	s := NumberedScheme{}
	cases := []struct {
		file string
		want string
	}{
		{"03-intro.html", "intro"},
		{"01-title.html", "title"},
		{"99-closing.html", "closing"},
		{"intro.html", "intro"}, // no prefix → still works (falls through ExtractNamePart)
	}
	for _, c := range cases {
		got := s.ExtractSlug(c.file)
		if got != c.want {
			t.Errorf("NumberedScheme.ExtractSlug(%q) = %q, want %q", c.file, got, c.want)
		}
	}
}

// TestSlugOnlyScheme_Format verifies that slug-only filenames have no
// numeric prefix — just slug.html. The position argument is ignored.
func TestSlugOnlyScheme_Format(t *testing.T) {
	s := SlugOnlyScheme{}
	cases := []struct {
		pos  int
		slug string
		want string
	}{
		{1, "title", "title.html"},
		{3, "intro", "intro.html"},
		{99, "closing", "closing.html"},
	}
	for _, c := range cases {
		got := s.Format(c.pos, c.slug)
		if got != c.want {
			t.Errorf("SlugOnlyScheme.Format(%d, %q) = %q, want %q", c.pos, c.slug, got, c.want)
		}
	}
}

// TestSlugOnlyScheme_ExtractSlug verifies slug extraction from slug-only
// filenames — just strips the .html suffix.
func TestSlugOnlyScheme_ExtractSlug(t *testing.T) {
	s := SlugOnlyScheme{}
	cases := []struct {
		file string
		want string
	}{
		{"intro.html", "intro"},
		{"closing.html", "closing"},
		{"my-long-slug.html", "my-long-slug"},
	}
	for _, c := range cases {
		got := s.ExtractSlug(c.file)
		if got != c.want {
			t.Errorf("SlugOnlyScheme.ExtractSlug(%q) = %q, want %q", c.file, got, c.want)
		}
	}
}

// TestSchemeForStyle verifies the factory function returns the correct
// scheme for each manifest filename_style value.
func TestSchemeForStyle(t *testing.T) {
	if SchemeForStyle("").Name() != "numbered" {
		t.Error("empty style should default to numbered")
	}
	if SchemeForStyle("numbered").Name() != "numbered" {
		t.Error("explicit numbered should return numbered")
	}
	if SchemeForStyle("slug-only").Name() != "slug-only" {
		t.Error("slug-only should return slug-only")
	}
	if SchemeForStyle("unknown").Name() != "numbered" {
		t.Error("unknown style should default to numbered")
	}
}

// TestShouldRenumber verifies the key behavioral difference between
// the two schemes: numbered requires renames, slug-only does not.
func TestShouldRenumber(t *testing.T) {
	n := NumberedScheme{}
	if !n.ShouldRenumber() {
		t.Error("NumberedScheme.ShouldRenumber() should be true")
	}
	s := SlugOnlyScheme{}
	if s.ShouldRenumber() {
		t.Error("SlugOnlyScheme.ShouldRenumber() should be false")
	}
}
