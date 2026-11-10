package hosts

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

// Table-driven test: each row is one case; the loop runs them all.
func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    File
		wantErr error
	}{
		{
			name:    "empty file",
			content: "",
			want:    File{},
		},
		{
			name:    "no markers",
			content: "127.0.0.1 localhost\n::1 ip6-localhost\n",
			want:    File{Before: []string{"127.0.0.1 localhost", "::1 ip6-localhost"}},
		},
		{
			name: "block in the middle",
			content: "127.0.0.1 localhost\n" +
				StartMarker + "\n" +
				"0.0.0.0 youtube.com\n" +
				EndMarker + "\n" +
				"# after\n",
			want: File{
				Before:   []string{"127.0.0.1 localhost"},
				Block:    []string{"0.0.0.0 youtube.com"},
				After:    []string{"# after"},
				HasBlock: true,
			},
		},
		{
			name:    "empty block",
			content: StartMarker + "\n" + EndMarker + "\n",
			want:    File{Before: []string{}, Block: []string{}, After: []string{}, HasBlock: true},
		},
		{
			name:    "markers with spaces and CRLF",
			content: "  " + StartMarker + "  \r\nx\n\t" + EndMarker + "\r\n",
			want:    File{Before: []string{}, Block: []string{"x"}, After: []string{}, HasBlock: true},
		},
		{
			name:    "no trailing newline",
			content: "a\n" + StartMarker + "\n" + EndMarker,
			want:    File{Before: []string{"a"}, Block: []string{}, After: []string{}, HasBlock: true},
		},
		{
			name:    "start without end",
			content: StartMarker + "\n0.0.0.0 x.com\n",
			wantErr: ErrMissingEnd,
		},
		{
			name:    "end before start",
			content: EndMarker + "\n" + StartMarker + "\n",
			wantErr: ErrMissingStart,
		},
		{
			name:    "duplicate start",
			content: StartMarker + "\n" + StartMarker + "\n" + EndMarker + "\n",
			wantErr: ErrDuplicateMark,
		},
		{
			name:    "duplicate end",
			content: StartMarker + "\n" + EndMarker + "\n" + EndMarker + "\n",
			wantErr: ErrDuplicateMark,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.content)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got  %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	f, err := Load(filepath.Join("..", "..", "testdata", "hosts"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !f.HasBlock {
		t.Fatal("expected a sadd block in testdata/hosts")
	}
	if len(f.Block) != 4 {
		t.Errorf("block has %d lines, want 4", len(f.Block))
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestPath(t *testing.T) {
	t.Setenv(PathEnv, "") // t.Setenv restores the old value after the test
	if got := Path(); got != DefaultPath {
		t.Errorf("Path() = %q, want %q", got, DefaultPath)
	}
	t.Setenv(PathEnv, "/tmp/fake-hosts")
	if got := Path(); got != "/tmp/fake-hosts" {
		t.Errorf("Path() = %q, want /tmp/fake-hosts", got)
	}
}
