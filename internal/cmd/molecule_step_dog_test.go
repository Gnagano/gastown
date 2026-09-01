package cmd

import "testing"

func TestDogNameFromActor(t *testing.T) {
	tests := []struct {
		name  string
		actor string
		want  string
		ok    bool
	}{
		{name: "generic dog", actor: "dog", ok: true},
		{name: "named dog", actor: "deacon/dogs/alpha", want: "alpha", ok: true},
		{name: "polecat unchanged", actor: "gastown/polecats/alpha"},
		{name: "deacon unchanged", actor: "deacon"},
		{name: "malformed dog", actor: "deacon/dogs/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := dogNameFromActor(tt.actor)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("dogNameFromActor(%q) = (%q, %v), want (%q, %v)", tt.actor, got, ok, tt.want, tt.ok)
			}
		})
	}
}
