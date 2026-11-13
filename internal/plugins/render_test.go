package plugins

import (
	"strings"
	"testing"
)

func sampleCatalog() Catalog {
	return Catalog{Entries: []Entry{
		{Repo: "speardragon/herdr-plugin-manager", Reason: "plugin management TUI"},
		{Repo: "ramarivera/herdr-pretty-which", Reason: "keybinding guide beta", Optional: true},
	}}
}

func TestRenderWritesTheSection(t *testing.T) {
	section := Render(sampleCatalog())

	for _, want := range []string{
		StartMarker,
		EndMarker,
		"| Plugin | Why | Kind |",
		"[`speardragon/herdr-plugin-manager`](https://github.com/speardragon/herdr-plugin-manager)",
		"plugin management TUI",
		"| core |",
		"| optional |",
	} {
		if !strings.Contains(section, want) {
			t.Fatalf("Render section does not contain %q:\n%s", want, section)
		}
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	first := Render(sampleCatalog())
	second := Render(sampleCatalog())
	if first != second {
		t.Fatalf("Render is not deterministic:\n%s\n---\n%s", first, second)
	}
}

func TestReplaceSectionRewritesBetweenMarkers(t *testing.T) {
	document := "# README\n\nbefore\n" + StartMarker + "\nstale\n" + EndMarker + "\nafter\n"

	updated, err := ReplaceSection(document, Render(sampleCatalog()))
	if err != nil {
		t.Fatalf("ReplaceSection error = %v, want nil", err)
	}
	if strings.Contains(updated, "stale") {
		t.Fatalf("ReplaceSection kept the stale section:\n%s", updated)
	}
	if !strings.HasPrefix(updated, "# README\n\nbefore\n"+StartMarker) {
		t.Fatalf("ReplaceSection changed the prefix:\n%s", updated)
	}
	if !strings.HasSuffix(updated, EndMarker+"\nafter\n") {
		t.Fatalf("ReplaceSection changed the suffix:\n%s", updated)
	}
}

func TestReplaceSectionIsIdempotent(t *testing.T) {
	document := "# README\n\n" + StartMarker + "\nold\n" + EndMarker + "\n"

	first, err := ReplaceSection(document, Render(sampleCatalog()))
	if err != nil {
		t.Fatalf("first ReplaceSection error = %v, want nil", err)
	}
	second, err := ReplaceSection(first, Render(sampleCatalog()))
	if err != nil {
		t.Fatalf("second ReplaceSection error = %v, want nil", err)
	}
	if first != second {
		t.Fatalf("a second render changed the document:\n%s\n---\n%s", first, second)
	}
}

func TestReplaceSectionRefusesBrokenMarkers(t *testing.T) {
	tests := []struct {
		name     string
		document string
		want     string
	}{
		{
			name:     "no markers",
			document: "# README\n",
			want:     "no " + StartMarker,
		},
		{
			name:     "start without end",
			document: StartMarker + "\n",
			want:     "no " + EndMarker,
		},
		{
			name:     "end before start",
			document: EndMarker + "\n" + StartMarker + "\n",
			want:     "comes before",
		},
		{
			name:     "duplicate start",
			document: StartMarker + "\n" + StartMarker + "\n" + EndMarker + "\n",
			want:     "declared twice",
		},
		{
			name:     "duplicate end",
			document: StartMarker + "\n" + EndMarker + "\n" + EndMarker + "\n",
			want:     "declared twice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReplaceSection(tt.document, Render(sampleCatalog()))
			if err == nil {
				t.Fatal("ReplaceSection error = nil, want a refusal")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ReplaceSection error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}
