package notion

import (
	"net/url"
	"strings"
)

// ParsePageID extracts a plain Notion page ID from a full URL or ID string.
func ParsePageID(input string) (string, error) {
	input = strings.TrimSpace(input)

	if strings.HasPrefix(input, "http") {
		u, err := url.Parse(input)
		if err != nil {
			return "", err
		}
		segments := strings.Split(strings.Trim(u.Path, "/"), "/")
		last := segments[len(segments)-1]
		last = strings.SplitN(last, "?", 2)[0]
		if idx := strings.LastIndex(last, "-"); idx != -1 {
			last = last[idx+1:]
		}
		return normalizeID(last), nil
	}

	return normalizeID(input), nil
}

func normalizeID(id string) string {
	return strings.ReplaceAll(id, "-", "")
}