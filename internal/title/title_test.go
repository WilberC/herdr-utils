package title

import (
	"strings"
	"testing"
)

func TestEligible(t *testing.T) {
	tests := []struct {
		name    string
		request string
		want    bool
	}{
		{name: "empty is not eligible", request: "", want: false},
		{name: "eleven runes is not eligible", request: "hola mundo!", want: false},
		{name: "twelve runes is eligible", request: "hola mundo!!", want: true},
		{name: "a real request is eligible", request: "fix the login redirect loop", want: true},
		{name: "surrounding space does not count", request: "   hola mundo!!   ", want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Eligible(test.request); got != test.want {
				t.Fatalf("Eligible(%q) = %v, want %v", test.request, got, test.want)
			}
		})
	}
}

func TestFrom(t *testing.T) {
	tests := []struct {
		name    string
		request string
		want    string
	}{
		{
			name:    "a short two-word request passes through",
			request: "Fix login redirect",
			want:    "Fix login redirect",
		},
		{
			name:    "a long request keeps the first words inside the cap",
			request: "Fix the login redirect loop in the auth service when the session cookie expires",
			want:    "Fix the login redirect loop",
		},
		{
			name:    "whitespace collapses and keeps five words",
			request: "  Fix   the   login   redirect   loop   now  ",
			want:    "Fix the login redirect loop",
		},
		{
			name:    "a non-English request keeps its own words",
			request: "Arreglar el bucle de redirección del login en el servicio de autenticación",
			want:    "Arreglar el bucle de redirección",
		},
		{
			name:    "one long word truncates to the cap",
			request: strings.Repeat("x", 50),
			want:    strings.Repeat("x", 34),
		},
		{
			name:    "two words beat the one-word cap squeeze",
			request: "Reimplementar supercalifragilisticoespialidosamente",
			want:    "Reimplementar",
		},
		{name: "empty request gives an empty title", request: "", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := From(test.request)
			if got != test.want {
				t.Fatalf("From(%q) = %q, want %q", test.request, got, test.want)
			}
			if got != "" && len([]rune(got)) > MaxLen {
				t.Fatalf("From(%q) = %q exceeds %d runes", test.request, got, MaxLen)
			}
		})
	}
}

func TestSkip(t *testing.T) {
	tests := []struct {
		name         string
		existing     string
		label        string
		defaultLabel string
		want         bool
	}{
		{
			name:         "an existing title wins: once per workspace",
			existing:     "Fix login redirect",
			label:        "herdr-sidecar",
			defaultLabel: "herdr-sidecar",
			want:         true,
		},
		{
			name:         "a manual rename wins",
			existing:     "",
			label:        "my workspace",
			defaultLabel: "herdr-sidecar",
			want:         true,
		},
		{
			name:         "the default label is workable",
			existing:     "",
			label:        "herdr-sidecar",
			defaultLabel: "herdr-sidecar",
			want:         false,
		},
		{
			name:         "no agent cwd means the rename check cannot fire",
			existing:     "",
			label:        "herdr-sidecar",
			defaultLabel: "",
			want:         false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Skip(test.existing, test.label, test.defaultLabel); got != test.want {
				t.Fatalf("Skip(%q, %q, %q) = %v, want %v", test.existing, test.label, test.defaultLabel, got, test.want)
			}
		})
	}
}
