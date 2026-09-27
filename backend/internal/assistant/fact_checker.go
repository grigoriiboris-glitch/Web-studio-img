package assistant

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	factAssessmentSupports    = "supports"
	factAssessmentContradicts = "contradicts"
	factAssessmentUnknown     = "unknown"

	factStatusSupported          = "SUPPORTED"
	factStatusPartiallySupported = "PARTIALLY_SUPPORTED"
	factStatusUnsupported        = "UNSUPPORTED"
	factStatusUnknown            = "UNKNOWN"
)

type factCheckEvidence struct {
	Text       string  `json:"text"`
	Source     string  `json:"source"`
	Assessment string  `json:"assessment"`
	Confidence float64 `json:"confidence"`
}

func factCheck(input map[string]any) (map[string]any, string, float64, string, error) {
	claim, ok := input["claim"].(string)
	claim = strings.TrimSpace(claim)
	if !ok || claim == "" {
		return nil, "", 0, "", errors.New("claim is required")
	}
	if len(claim) > 20000 {
		return nil, "", 0, "", errors.New("claim exceeds 20000 characters")
	}

	rawEvidence, ok := input["evidence"].([]any)
	if !ok || len(rawEvidence) == 0 {
		return nil, "", 0, "", errors.New("evidence must contain at least one item")
	}
	if len(rawEvidence) > 50 {
		return nil, "", 0, "", errors.New("evidence must contain at most 50 items")
	}

	evidence := make([]factCheckEvidence, 0, len(rawEvidence))
	sources := make(map[string]struct{})
	supporting, contradicting, unknown := 0, 0, 0
	confidenceSum := 0.0

	for index, raw := range rawEvidence {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, "", 0, "", fmt.Errorf("evidence[%d] must be an object", index)
		}

		text, ok := item["text"].(string)
		text = strings.TrimSpace(text)
		if !ok || text == "" {
			return nil, "", 0, "", fmt.Errorf("evidence[%d].text is required", index)
		}
		if len(text) > 20000 {
			return nil, "", 0, "", fmt.Errorf("evidence[%d].text exceeds 20000 characters", index)
		}

		source, ok := item["source"].(string)
		source = strings.TrimSpace(source)
		if !ok || source == "" {
			return nil, "", 0, "", fmt.Errorf("evidence[%d].source is required", index)
		}
		if len(source) > 2000 {
			return nil, "", 0, "", fmt.Errorf("evidence[%d].source exceeds 2000 characters", index)
		}

		assessment, ok := item["assessment"].(string)
		if !ok {
			return nil, "", 0, "", fmt.Errorf("evidence[%d].assessment is required", index)
		}
		assessment = strings.ToLower(strings.TrimSpace(assessment))
		switch assessment {
		case factAssessmentSupports:
			supporting++
		case factAssessmentContradicts:
			contradicting++
		case factAssessmentUnknown:
			unknown++
		default:
			return nil, "", 0, "", fmt.Errorf("evidence[%d].assessment must be supports, contradicts, or unknown", index)
		}

		confidence, ok := item["confidence"].(float64)
		if !ok || confidence < 0 || confidence > 1 {
			return nil, "", 0, "", fmt.Errorf("evidence[%d].confidence must be between 0 and 1", index)
		}

		evidence = append(evidence, factCheckEvidence{
			Text:       text,
			Source:     source,
			Assessment: assessment,
			Confidence: confidence,
		})
		sources[source] = struct{}{}
		confidenceSum += confidence
	}

	status := classifyFactStatus(supporting, contradicting, unknown)
	overallConfidence := confidenceSum / float64(len(evidence))
	sourceList := make([]string, 0, len(sources))
	for source := range sources {
		sourceList = append(sourceList, source)
	}
	sort.Strings(sourceList)

	result := map[string]any{
		"claim": claim,
		"evidence": evidence,
		"sources": sourceList,
		"supporting_count": supporting,
		"contradicting_count": contradicting,
		"unknown_count": unknown,
		"status": status,
		"confidence": overallConfidence,
		"checked_at": time.Now().UTC(),
		"method": "evidence-adjudicator-v1",
		"source_verification": "not_performed",
	}
	return result,
		"Fact check classifies only user-supplied evidence assessments; it does not invent evidence or fetch external sources.",
		overallConfidence,
		"Confidence is the average of the supplied evidence confidence values. Source contents and provenance are not independently verified by this MVP.",
		nil
}

func classifyFactStatus(supporting, contradicting, unknown int) string {
	switch {
	case supporting == 0 && contradicting == 0:
		return factStatusUnknown
	case supporting > 0 && contradicting == 0 && unknown == 0:
		return factStatusSupported
	case supporting > 0 && contradicting > 0:
		return factStatusPartiallySupported
	case supporting > 0 && unknown > 0:
		return factStatusPartiallySupported
	default:
		return factStatusUnsupported
	}
}
