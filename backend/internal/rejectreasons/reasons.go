package rejectreasons

import (
	"errors"
	"sort"
	"strings"
)

var MVP = []string{
	"composition", "subject", "pose", "lighting", "color", "material",
	"background", "object", "style", "prompt", "quality", "other",
}

var allowed = func() map[string]struct{} {
	result := make(map[string]struct{}, len(MVP))
	for _, reason := range MVP {
		result[reason] = struct{}{}
	}
	return result
}()

func IsAllowed(reason string) bool {
	_, ok := allowed[strings.ToLower(strings.TrimSpace(reason))]
	return ok
}

func Normalize(input []string) ([]string, error) {
	if len(input) > len(MVP) {
		return nil, errors.New("at most 12 reject reasons are allowed")
	}
	seen := make(map[string]struct{}, len(input))
	result := make([]string, 0, len(input))
	for _, reason := range input {
		value := strings.ToLower(strings.TrimSpace(reason))
		if !IsAllowed(value) {
			return nil, errors.New("unknown reject reason")
		}
		if _, exists := seen[value]; exists {
			return nil, errors.New("duplicated reject reason")
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}
