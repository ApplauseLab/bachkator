package quality

import (
	"encoding/json"
	"strconv"
	"strings"
)

func splitLocation(location string) (string, int) {
	parts := strings.Split(location, ":")
	if len(parts) < 2 {
		return location, 0
	}
	line, _ := strconv.Atoi(parts[len(parts)-2])
	return strings.Join(parts[:len(parts)-2], ":"), line
}

func parseFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func numberFromJSON(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
