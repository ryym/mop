package daemon

import "testing"

func TestDocumentDisplayPath(t *testing.T) {
	t.Setenv("HOME", "/home/user")

	tests := []struct {
		path string
		want string
	}{
		{"/home/user/notes/todo.md", "~/notes/todo.md"},
		{"/home/user", "~"},
		{"/home/user2/notes/todo.md", "/home/user2/notes/todo.md"},
		{"/srv/notes/todo.md", "/srv/notes/todo.md"},
	}
	for _, tt := range tests {
		if got := newDocument(tt.path).displayPath(); got != tt.want {
			t.Errorf("displayPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestDocumentTitle(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/notes/todo.md", "notes/todo.md"},
		{"/todo.md", "/todo.md"},
	}
	for _, tt := range tests {
		if got := newDocument(tt.path).title(); got != tt.want {
			t.Errorf("title(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
