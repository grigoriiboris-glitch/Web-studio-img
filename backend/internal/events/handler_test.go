package events

import "testing"

func TestParseLastEventID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
		ok    bool
	}{
		{name: "empty", input: "", want: 0, ok: true},
		{name: "valid", input: "42", want: 42, ok: true},
		{name: "zero", input: "0", want: 0, ok: true},
		{name: "negative", input: "-1", want: 0, ok: false},
		{name: "invalid", input: "abc", want: 0, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLastEventID(tt.input)
			if tt.ok && err != nil { t.Fatalf("unexpected error: %v", err) }
			if !tt.ok && err == nil { t.Fatal("expected error") }
			if got != tt.want { t.Fatalf("got %d, want %d", got, tt.want) }
		})
	}
}
