package types

import "testing"

func TestUniqueFileName(t *testing.T) {
	files := map[string]File{}
	for _, tc := range []struct{ path, want string }{
		{"/Show/Season 1/01.mkv", "01.mkv"},
		{"/Show/Season 2/01.mkv", "Season 2 - 01.mkv"},
		{"/Other/Season 2/01.mkv", "Other - Season 2 - 01.mkv"},
		{"/Other/Season 2/01.mkv", "Other - Season 2 - 01 (2).mkv"},
		{"movie.mkv", "movie.mkv"},
	} {
		got := UniqueFileName(files, tc.path)
		if got != tc.want {
			t.Fatalf("UniqueFileName(%q) = %q, want %q", tc.path, got, tc.want)
		}
		files[got] = File{}
	}
}
