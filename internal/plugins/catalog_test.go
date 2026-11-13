package plugins

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseReadsEveryCatalogEntry(t *testing.T) {
	data := []byte(`# The recommended herdr plugins.
[[plugin]]
repo = "speardragon/herdr-plugin-manager"
reason = "plugin management TUI"

[[plugin]]
repo = "ramarivera/herdr-pretty-which"
reason = "Rust keybinding guide beta"
optional = true
ref = "v0.3.0"
`)

	catalog, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse error = %v, want nil", err)
	}

	want := Catalog{Entries: []Entry{
		{Repo: "speardragon/herdr-plugin-manager", Reason: "plugin management TUI"},
		{Repo: "ramarivera/herdr-pretty-which", Reason: "Rust keybinding guide beta", Optional: true, Ref: "v0.3.0"},
	}}
	if !reflect.DeepEqual(catalog, want) {
		t.Fatalf("Parse = %#v, want %#v", catalog, want)
	}
}

func TestParseDefaultsOptionalToFalse(t *testing.T) {
	catalog, err := Parse([]byte("[[plugin]]\nrepo = \"owner/repo\"\nreason = \"why\"\n"))
	if err != nil {
		t.Fatalf("Parse error = %v, want nil", err)
	}
	if len(catalog.Entries) != 1 || catalog.Entries[0].Optional {
		t.Fatalf("Parse = %#v, want one non-optional entry", catalog.Entries)
	}
}

func TestParseRefusesIncompleteEntries(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "missing repo",
			data: "[[plugin]]\nreason = \"why\"\n",
			want: "entry 1 declares no repo",
		},
		{
			name: "missing reason",
			data: "[[plugin]]\nrepo = \"owner/repo\"\n",
			want: "entry 1 (owner/repo) declares no reason",
		},
		{
			name: "repo is not owner/repo",
			data: "[[plugin]]\nrepo = \"ownerrepo\"\nreason = \"why\"\n",
			want: `"ownerrepo" is not an owner/repo pair`,
		},
		{
			name: "optional is not a boolean",
			data: "[[plugin]]\nrepo = \"owner/repo\"\nreason = \"why\"\noptional = \"yes\"\n",
			want: "optional must be true or false",
		},
		{
			name: "unknown key",
			data: "[[plugin]]\nrepo = \"owner/repo\"\nreason = \"why\"\noptinal = true\n",
			want: `unknown key "optinal"`,
		},
		{
			name: "duplicate repo",
			data: "[[plugin]]\nrepo = \"owner/repo\"\nreason = \"why\"\n[[plugin]]\nrepo = \"owner/repo\"\nreason = \"again\"\n",
			want: "owner/repo is declared twice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.data))
			if err == nil {
				t.Fatal("Parse error = nil, want a refusal")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Parse error = %q, want it to contain %q", err, tt.want)
			}
			if !strings.Contains(err.Error(), FileName) {
				t.Fatalf("Parse error = %q, want it to name %s", err, FileName)
			}
		})
	}
}
