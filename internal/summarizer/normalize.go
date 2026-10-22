// Package summarizer implements deterministic, non-AI log normalization,
// Drain-style template clustering and statistical container anomaly correlation
// for the /whatsup (/pulse) command.
package summarizer

import (
	"sort"
	"strings"
	"sync"
)

// Severity levels ordered by priority.
const (
	SevError = "ERROR"
	SevWarn  = "WARN"
	SevInfo  = "INFO"
)

// LogCluster represents a group of log lines sharing the same normalized template.
type LogCluster struct {
	Severity string
	Template string
	Count    int
	Sample   string
}

var builderPool = sync.Pool{
	New: func() any {
		var b strings.Builder
		b.Grow(256)
		return &b
	},
}

// NormalizeLine converts a raw log line into its canonical template and
// extracts its severity (ERROR, WARN, INFO) without regular expressions on the
// hot path.
func NormalizeLine(line string) (severity, template string) {
	severity = detectSeverity(line)
	body := stripLeadingTimestamp(strings.TrimSpace(line))

	b := builderPool.Get().(*strings.Builder)
	b.Reset()
	defer builderPool.Put(b)

	first := true
	for len(body) > 0 {
		// Skip whitespace
		i := 0
		for i < len(body) && (body[i] == ' ' || body[i] == '\t') {
			i++
		}
		body = body[i:]
		if len(body) == 0 {
			break
		}
		// Find end of token
		j := 0
		for j < len(body) && body[j] != ' ' && body[j] != '\t' {
			j++
		}
		tok := body[:j]
		body = body[j:]

		if isSeverityToken(tok) {
			continue
		}
		if !first {
			b.WriteByte(' ')
		}
		first = false
		b.WriteString(normalizeToken(tok))
	}
	return severity, b.String()
}

// ClusterLogs groups raw log lines by normalized template and sorts the
// resulting clusters by severity (ERROR > WARN > INFO) and descending count.
func ClusterLogs(lines []string) []LogCluster {
	if len(lines) == 0 {
		return nil
	}
	type key struct {
		sev  string
		tmpl string
	}
	idx := make(map[key]int, min(len(lines), 64))
	clusters := make([]LogCluster, 0, min(len(lines), 32))

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		sev, tmpl := NormalizeLine(line)
		if tmpl == "" {
			continue
		}
		k := key{sev: sev, tmpl: tmpl}
		if pos, ok := idx[k]; ok {
			clusters[pos].Count++
		} else {
			idx[k] = len(clusters)
			clusters = append(clusters, LogCluster{
				Severity: sev,
				Template: tmpl,
				Count:    1,
				Sample:   line,
			})
		}
	}

	sort.Slice(clusters, func(a, b int) bool {
		ra, rb := sevRank(clusters[a].Severity), sevRank(clusters[b].Severity)
		if ra != rb {
			return ra < rb
		}
		if clusters[a].Count != clusters[b].Count {
			return clusters[a].Count > clusters[b].Count
		}
		return clusters[a].Template < clusters[b].Template
	})
	return clusters
}

func sevRank(s string) int {
	switch s {
	case SevError:
		return 0
	case SevWarn:
		return 1
	default:
		return 2
	}
}

func detectSeverity(line string) string {
	upper := strings.ToUpper(line)
	for _, kw := range []string{"ERROR", "FATAL", "PANIC", "CRIT", "ERR]", "OOM", "KILLED", "REFUSED", "DENIED", "TIMEOUT", "EXCEPTION"} {
		if strings.Contains(upper, kw) {
			return SevError
		}
	}
	for _, kw := range []string{"WARN", "THROTTL", "RETRY", "DEPRECATED", "SLOW"} {
		if strings.Contains(upper, kw) {
			return SevWarn
		}
	}
	return SevInfo
}

func isSeverityToken(tok string) bool {
	clean := strings.Trim(tok, "[]():")
	switch strings.ToUpper(clean) {
	case "INFO", "WARN", "WARNING", "ERROR", "ERR", "FATAL", "DEBUG", "TRACE":
		return true
	}
	return false
}

// stripLeadingTimestamp removes common ISO-8601 / RFC3339 date-time prefixes
// like "2026-10-22T09:15:02Z " or "2026-10-22 09:15:02.123 ".
func stripLeadingTimestamp(s string) string {
	if len(s) >= 19 && isDigit(s[0]) && isDigit(s[1]) && isDigit(s[2]) && isDigit(s[3]) && s[4] == '-' {
		// Advance past first token; if the second token starts with digits:digits, skip it too.
		sp := strings.IndexByte(s, ' ')
		if sp > 0 {
			rest := strings.TrimLeft(s[sp+1:], " ")
			if len(rest) >= 8 && isDigit(rest[0]) && isDigit(rest[1]) && rest[2] == ':' {
				if sp2 := strings.IndexByte(rest, ' '); sp2 > 0 {
					return strings.TrimLeft(rest[sp2+1:], " ")
				}
			}
			return rest
		}
	}
	return s
}

func normalizeToken(tok string) string {
	// Preserve trailing punctuation such as ',', ';', ')', ']'
	prefix, core, suffix := splitPunct(tok)
	if core == "" {
		return tok
	}
	// Handle key=value pairs recursively (e.g. status=503, ip=10.0.0.1:8080)
	if eq := strings.IndexByte(core, '='); eq > 0 && eq < len(core)-1 {
		k := core[:eq+1]
		v := core[eq+1:]
		return prefix + k + classifyAtom(v) + suffix
	}
	return prefix + classifyAtom(core) + suffix
}

func classifyAtom(s string) string {
	// IPv4:Port or IPv4
	if ip, port, ok := strings.Cut(s, ":"); ok && isIPv4(ip) && isAllDigits(port) {
		return "<IP>:<PORT>"
	}
	if isIPv4(s) {
		return "<IP>"
	}
	// UUID (8-4-4-4-12 hex)
	if isUUID(s) {
		return "<UUID>"
	}
	// Hex literal (0x... or >=8 hex chars with digits)
	if isHexToken(s) {
		return "<HEX>"
	}
	// File path containing at least two slashes
	if strings.HasPrefix(s, "/") && strings.Count(s, "/") >= 2 {
		return "<PATH>"
	}
	// Pure number or number with unit (e.g. 42, 123ms, 98.5%)
	if isNumberOrUnit(s) {
		return "<NUM>"
	}
	return s
}

func splitPunct(s string) (prefix, core, suffix string) {
	start := 0
	for start < len(s) && (s[start] == '[' || s[start] == '(' || s[start] == '"' || s[start] == '\'') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ']' || s[end-1] == ')' || s[end-1] == ',' || s[end-1] == ';' || s[end-1] == ':' || s[end-1] == '"' || s[end-1] == '\'') {
		end--
	}
	return s[:start], s[start:end], s[end:]
}

func isIPv4(s string) bool {
	if strings.Count(s, ".") != 3 {
		return false
	}
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 || len(p) > 3 || !isAllDigits(p) {
			return false
		}
	}
	return true
}

func isUUID(s string) bool {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !isHexChar(s[i]) {
			return false
		}
	}
	return true
}

func isHexToken(s string) bool {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		if len(s) <= 2 {
			return false
		}
		for i := 2; i < len(s); i++ {
			if !isHexChar(s[i]) {
				return false
			}
		}
		return true
	}
	if len(s) < 8 {
		return false
	}
	hasDigit := false
	hasAlpha := false
	for i := 0; i < len(s); i++ {
		if !isHexChar(s[i]) {
			return false
		}
		if isDigit(s[i]) {
			hasDigit = true
		} else {
			hasAlpha = true
		}
	}
	return hasDigit && hasAlpha
}

func isNumberOrUnit(s string) bool {
	if len(s) == 0 || (!isDigit(s[0]) && (s[0] != '-' || len(s) == 1 || !isDigit(s[1]))) {
		return false
	}
	i := 0
	if s[0] == '-' {
		i++
	}
	for i < len(s) && (isDigit(s[i]) || s[i] == '.') {
		i++
	}
	rest := s[i:]
	switch rest {
	case "", "ms", "s", "us", "ns", "m", "h", "%", "B", "KB", "MB", "GB":
		return true
	}
	return false
}

func isAllDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isHexChar(b byte) bool {
	return isDigit(b) || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
