package gateway

import (
	"errors"
	"fmt"
	"strings"
)

const (
	topicSep    = "/"
	singleLevel = "+"
	multiLevel  = "#"
)

const topicSpecialChars = topicSep + singleLevel + multiLevel

var (
	errEmptyTopicLevelName   = errors.New("invalid topic level name: name is empty")
	errInvalidTopicLevelName = errors.New("invalid topic level name: name contains invalid characters")
)

// checkTopicLevelName checks if topic level name consists of valid characters.
func checkTopicLevelName(name string) error {
	switch {
	case name == "":
		return errEmptyTopicLevelName
	case strings.ContainsAny(name, topicSpecialChars):
		return errInvalidTopicLevelName
	default:
		return nil
	}
}

// root / class / node / property / [command]

const (
	classCS    = "cs"
	classLoco  = "loco"
	classError = "error"
)

const (
	partRoot = iota
	partClass
	partNode
	partProperty
	partCommand
)

const (
	minNumPart = 1
	maxNumPart = partCommand + 1
)

func joinTopic(topics ...string) string { return strings.Join(topics, topicSep) }

func splitTopic(topic string) []string { return strings.Split(topic, topicSep) }

type topic []string

func (t topic) String() string { return joinTopic(t...) }

// noRoot returns topic without root part.
func (t topic) noRoot() string {
	return joinTopic(t[1:]...)
}

// noCommand returns topic without command part.
func (t topic) noCommand() string {
	l := len(t)
	if l == maxNumPart {
		return joinTopic(t[:l-1]...)
	}
	return joinTopic(t...)
}

func parseTopic(s string) (topic, error) {
	parts := splitTopic(s)
	l := len(parts)
	if l < minNumPart || l > maxNumPart {
		return nil, fmt.Errorf("invalid number of topic parts %d - expected %d - %d", l, minNumPart, maxNumPart)
	}
	return topic(parts), nil
}
