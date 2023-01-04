package gateway

import (
	"errors"
	"strings"
)

const (
	sep         = "/"
	singleLevel = "+"
	multiLevel  = "#"
)

const specialChars = sep + singleLevel + multiLevel

var (
	errEmptyTopicLevelName   = errors.New("invalid topic level name: name is empty")
	errInvalidTopicLevelName = errors.New("invalid topic level name: name contains invalid characters")
)

// CheckLevelName checks if topic level name consists of valid characters.
func CheckLevelName(name string) error {
	switch {
	case name == "":
		return errEmptyTopicLevelName
	case strings.ContainsAny(name, specialChars):
		return errInvalidTopicLevelName
	default:
		return nil
	}
}

func topicJoin(topicParts []string) string    { return strings.Join(topicParts, sep) }
func topicJoinStr(topicStrs ...string) string { return strings.Join(topicStrs, sep) }
func topicSplit(topicStr string) []string     { return strings.Split(topicStr, sep) }
