package proxy

import "strings"

const (
	itemOpen     = "<item>"
	itemClose    = "</item>"
	channelClose = "</channel>"
)

func countItems(xml string) int {
	if strings.TrimSpace(xml) == "" {
		return 0
	}
	return strings.Count(xml, itemOpen)
}

func mergeXML(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if !strings.Contains(a, itemOpen) {
		return b
	}
	if !strings.Contains(b, itemOpen) {
		return a
	}
	cutA := strings.Index(a, channelClose)
	startB := strings.Index(b, itemOpen)
	if cutA < 0 || startB < 0 {
		return a
	}
	return a[:cutA] + b[startB:]
}

func trimXMLItems(xml string, limit int) string {
	if limit <= 0 || countItems(xml) <= limit {
		return xml
	}
	idx := -1
	pos := 0
	for i := 0; i < limit+1; i++ {
		n := strings.Index(xml[pos:], itemOpen)
		if n < 0 {
			return xml
		}
		idx = pos + n
		pos = idx + len(itemOpen)
	}
	lastClose := strings.LastIndex(xml, itemClose)
	if idx < 0 || lastClose < 0 {
		return xml
	}
	return xml[:idx] + xml[lastClose+len(itemClose):]
}

func hasChannel(xml string) bool { return strings.Contains(xml, "<channel>") }
