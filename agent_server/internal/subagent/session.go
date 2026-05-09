package subagent

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

const sessionIDPrefix = "subsess"

var sessionCounter uint64

func NewSessionID(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "subagent"
	}
	return fmt.Sprintf("%s_%s_%d_%d", sessionIDPrefix, safeSessionPrefix(prefix), time.Now().UTC().UnixNano(), atomic.AddUint64(&sessionCounter, 1))
}

func safeSessionPrefix(prefix string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(prefix) {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "subagent"
	}
	return out
}
