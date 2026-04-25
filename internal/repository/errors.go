package repository

import (
	"errors"
	"strings"
)

var ErrNotFound = errors.New("record not found")

// escapeLikeKeyword escapes special characters in LIKE queries
func escapeLikeKeyword(keyword string) string {
	keyword = strings.ReplaceAll(keyword, "\\", "\\\\")
	keyword = strings.ReplaceAll(keyword, "%", "\\%")
	keyword = strings.ReplaceAll(keyword, "_", "\\_")
	return keyword
}