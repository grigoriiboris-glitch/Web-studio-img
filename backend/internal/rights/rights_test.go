package rights

import "testing"

func TestRightsValidation(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
		fn   func(string) bool
	}{
		{"asset target", true, validTargetType},
		{"reference target", true, validTargetType},
		{"invalid target", false, validTargetType},
		{"user owned", true, validOwnership},
		{"unknown ownership", true, validOwnership},
		{"invalid ownership", false, validOwnership},
		{"verified", true, validVerificationState},
		{"restricted", true, validVerificationState},
		{"invalid verification", false, validVerificationState},
		{"artist constraint", true, validConstraintKind},
		{"style constraint", true, validConstraintKind},
		{"invalid constraint", false, validConstraintKind},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := "invalid"
			switch tt.name {
			case "asset target":
				value = "asset"
			case "reference target":
				value = "reference"
			case "user owned":
				value = "user_owned"
			case "unknown ownership":
				value = "unknown"
			case "verified":
				value = "verified"
			case "restricted":
				value = "restricted"
			case "artist constraint":
				value = "artist"
			case "style constraint":
				value = "style"
			}
			if got := tt.fn(value); got != tt.ok {
				t.Fatalf("got %v, want %v", got, tt.ok)
			}
		})
	}
}
