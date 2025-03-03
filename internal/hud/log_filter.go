package hud

import (
	"strings"

	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

func LogFilterFromString(v string) LogFilter {
	return LogFilter{
		spanPrefix: strings.TrimPrefix(v, "!"),
		not:        strings.HasPrefix(v, "!"),
	}
}

func LogFiltersFromStrings(v []string) LogFilters {
	r := LogFilters{}
	for _, s := range v {
		f := LogFilterFromString(s)
		if f.not {
			r.deny = append(r.deny, f)
		} else {
			r.allow = append(r.allow, f)
		}
	}

	return r
}

type LogFilter struct {
	spanPrefix string
	not        bool
}

func (f LogFilter) Matches(l logstore.LogLine) bool {
	return strings.HasPrefix(string(l.SpanID), f.spanPrefix) != f.not
}

type LogFilters struct {
	allow []LogFilter
	deny  []LogFilter
}

func (s LogFilters) applyDeny(line logstore.LogLine) bool {
	shouldInclude := true
	for _, f := range s.deny {
		shouldInclude = shouldInclude && f.Matches(line)
	}

	return shouldInclude
}

func (s LogFilters) applyAllow(line logstore.LogLine) bool {
	if len(s.allow) == 0 {
		return true
	}

	shouldInclude := false
	for _, f := range s.allow {
		shouldInclude = shouldInclude || f.Matches(line)
	}

	return shouldInclude
}

func (s LogFilters) Apply(lines []logstore.LogLine) []logstore.LogLine {
	hasDeny := len(s.deny) > 0
	hasAllow := len(s.allow) > 0
	if !hasDeny && !hasAllow {
		return lines
	}

	filtered := []logstore.LogLine{}
	for _, line := range lines {
		if (hasDeny && s.applyDeny(line)) || (hasAllow && s.applyAllow(line)) {
			filtered = append(filtered, line)
		}
	}

	return filtered
}
