package handlers

import "strings"

func sanitizeUsers(users []string) []string {
	clean := make([]string, 0, len(users))
	for _, u := range users {
		trimmed := strings.TrimSpace(u)
		if trimmed == "" {
			continue
		}
		clean = append(clean, trimmed)
	}
	return clean
}
