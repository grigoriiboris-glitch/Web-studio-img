package assistant

import (
	"strings"
	"testing"
)

func TestFactCheckSupportedContract(t *testing.T) {
	result, explanation, confidence, uncertainty, err := factCheck(map[string]any{
		"claim": "A claim with supporting evidence",
		"evidence": []any{
			map[string]any{
				"text": "Primary source supports the claim.",
				"source": "https://example.test/primary",
				"assessment": "supports",
				"confidence": 0.9,
			},
			map[string]any{
				"text": "Second source supports the claim.",
				"source": "https://example.test/secondary",
				"assessment": "supports",
				"confidence": 0.8,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != factStatusSupported {
		t.Fatalf("unexpected status: %#v", result["status"])
	}
	if confidence != 0.85 {
		t.Fatalf("unexpected confidence: %v", confidence)
	}
	if !strings.Contains(explanation, "does not invent evidence") {
		t.Fatalf("expected non-fabrication explanation, got %q", explanation)
	}
	if !strings.Contains(uncertainty, "not independently verified") {
		t.Fatalf("expected verification uncertainty, got %q", uncertainty)
	}
	if result["method"] != "evidence-adjudicator-v1" {
		t.Fatalf("unexpected method: %#v", result["method"])
	}
}

func TestFactCheckMixedAndUnknownStatuses(t *testing.T) {
	mixed, _, _, _, err := factCheck(map[string]any{
		"claim": "Disputed claim",
		"evidence": []any{
			map[string]any{"text": "Support", "source": "a", "assessment": "supports", "confidence": 0.8},
			map[string]any{"text": "Contradiction", "source": "b", "assessment": "contradicts", "confidence": 0.7},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mixed["status"] != factStatusPartiallySupported {
		t.Fatalf("unexpected mixed status: %#v", mixed["status"])
	}

	unknown, _, _, _, err := factCheck(map[string]any{
		"claim": "Unknown claim",
		"evidence": []any{
			map[string]any{"text": "No determination", "source": "a", "assessment": "unknown", "confidence": 0.2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if unknown["status"] != factStatusUnknown {
		t.Fatalf("unexpected unknown status: %#v", unknown["status"])
	}
}

func TestFactCheckRejectsMissingOrInvalidEvidence(t *testing.T) {
	cases := []map[string]any{
		{"claim": "Claim"},
		{"claim": "Claim", "evidence": []any{
			map[string]any{"text": "e", "source": "s", "assessment": "supports", "confidence": 1.1},
		}},
		{"claim": "Claim", "evidence": []any{
			map[string]any{"text": "e", "source": "s", "assessment": "invented", "confidence": 0.5},
		}},
	}
	for index, input := range cases {
		if _, _, _, _, err := factCheck(input); err == nil {
			t.Fatalf("case %d: expected validation error", index)
		}
	}
}
