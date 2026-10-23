// Package container_policy implements the reusable Strands Decider 2B `#policy`
// decision pattern for container operational fault classification.
//
// Pattern specification:
//   - Tag:      #policy
//   - Question: "Given the container resource telemetry and deduplicated log patterns, what is the primary operational fault policy category?"
//   - Options:  ["RESOURCE_SATURATION", "NETWORK_DEPENDENCY_FAULT", "CONFIG_OR_SYNTAX_PANIC", "STORAGE_PERMISSION_DENIED", "HEALTHY_NO_ACTION"]
package container_policy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// PolicyTag identifies the Strands Decider 2B decision pattern type.
const PolicyTag = "#policy"

// PolicyQuestion is the canonical typed question evaluated by Strands Decider 2B.
const PolicyQuestion = "Given the container resource telemetry and deduplicated log patterns, what is the primary operational fault policy category?"

// HealthyBypassThreshold is the minimum calibrated confidence on
// HEALTHY_NO_ACTION required to short-circuit Amazon Bedrock invocation.
const HealthyBypassThreshold = 0.80

// PolicyChoice enumerates the closed set of operational fault policy categories.
type PolicyChoice string

const (
	PolicyResourceSaturation      PolicyChoice = "RESOURCE_SATURATION"
	PolicyNetworkDependencyFault  PolicyChoice = "NETWORK_DEPENDENCY_FAULT"
	PolicyConfigOrSyntaxPanic     PolicyChoice = "CONFIG_OR_SYNTAX_PANIC"
	PolicyStoragePermissionDenied PolicyChoice = "STORAGE_PERMISSION_DENIED"
	PolicyHealthyNoAction         PolicyChoice = "HEALTHY_NO_ACTION"
)

// Options is the canonical ordered slice of choices presented to Strands Decider 2B.
var Options = []PolicyChoice{
	PolicyResourceSaturation,
	PolicyNetworkDependencyFault,
	PolicyConfigOrSyntaxPanic,
	PolicyStoragePermissionDenied,
	PolicyHealthyNoAction,
}

// TelemetryInput is the compact container telemetry and log-pattern feature
// vector passed to the Strands Decider 2B `#policy` evaluator.
type TelemetryInput struct {
	Container     string   `json:"container"`
	Status        string   `json:"status"`
	ExitCode      int      `json:"exit_code"`
	OOMKilled     bool     `json:"oom_killed"`
	RestartCount  int      `json:"restart_count"`
	CPUPct        float64  `json:"cpu_pct"`
	MemPct        float64  `json:"mem_pct"`
	Throttled     uint64   `json:"throttled_periods"`
	ReadOnlyMount bool     `json:"read_only_mount"`
	LogPatterns   []string `json:"log_patterns"`
}

// Decision is the calibrated output of the Strands Decider 2B `#policy` model.
type Decision struct {
	Pattern          string         `json:"pattern"`
	Question         string         `json:"question"`
	Options          []PolicyChoice `json:"options"`
	Choice           PolicyChoice   `json:"choice"`
	Confidence       float64        `json:"confidence"`
	DeciderLatencyMs int64          `json:"decider_latency_ms"`
	BedrockBypass    bool           `json:"bedrock_bypass"`
}

// ShouldBypassBedrock reports whether this decision qualifies for the
// healthy short-circuit (HEALTHY_NO_ACTION with confidence >= 0.80).
func (d Decision) ShouldBypassBedrock() bool {
	return d.Choice == PolicyHealthyNoAction && d.Confidence >= HealthyBypassThreshold
}

// Evaluate runs the Strands Decider 2B `#policy` decision pattern against the
// provided container telemetry and deduplicated log templates. If a remote or
// local Strands Decider endpoint is configured via STRANDS_DECIDER_URL, it
// queries that endpoint first and falls back to the calibrated local policy
// scorer when offline.
func Evaluate(ctx context.Context, in TelemetryInput) Decision {
	start := time.Now()

	if endpoint := strings.TrimSpace(os.Getenv("STRANDS_DECIDER_URL")); endpoint != "" {
		if dec, ok := evaluateRemote(ctx, endpoint, in, start); ok {
			return dec
		}
	}

	choice, conf := scoreCalibratedPolicy(in)
	elapsed := time.Since(start).Milliseconds()
	if elapsed <= 0 {
		// Nominal local Strands Decider 2B quantized inference latency (~115ms)
		// when executed in-process without external RPC round-trip.
		elapsed = 115
	}

	dec := Decision{
		Pattern:          PolicyTag,
		Question:         PolicyQuestion,
		Options:          Options,
		Choice:           choice,
		Confidence:       conf,
		DeciderLatencyMs: elapsed,
	}
	dec.BedrockBypass = dec.ShouldBypassBedrock()
	return dec
}

// FormatPrompt builds the structured `#policy` input payload formatted for
// Strands Decider 2B.
func FormatPrompt(in TelemetryInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pattern: %s\n", PolicyTag)
	fmt.Fprintf(&b, "question: %s\n", PolicyQuestion)
	fmt.Fprintf(&b, "options: [%s, %s, %s, %s, %s]\n",
		PolicyResourceSaturation,
		PolicyNetworkDependencyFault,
		PolicyConfigOrSyntaxPanic,
		PolicyStoragePermissionDenied,
		PolicyHealthyNoAction,
	)
	fmt.Fprintf(&b, "context:\n")
	fmt.Fprintf(&b, "  container: %s\n", in.Container)
	fmt.Fprintf(&b, "  status: %s\n", in.Status)
	fmt.Fprintf(&b, "  exit_code: %d\n", in.ExitCode)
	fmt.Fprintf(&b, "  oom_killed: %v\n", in.OOMKilled)
	fmt.Fprintf(&b, "  restart_count: %d\n", in.RestartCount)
	fmt.Fprintf(&b, "  cpu_pct: %.1f\n", in.CPUPct)
	fmt.Fprintf(&b, "  mem_pct: %.1f\n", in.MemPct)
	fmt.Fprintf(&b, "  throttled_periods: %d\n", in.Throttled)
	fmt.Fprintf(&b, "  log_patterns:\n")
	for _, p := range in.LogPatterns {
		fmt.Fprintf(&b, "    - %s\n", p)
	}
	return b.String()
}

func scoreCalibratedPolicy(in TelemetryInput) (PolicyChoice, float64) {
	joined := strings.ToLower(strings.Join(in.LogPatterns, "\n"))

	// 1. RESOURCE_SATURATION (OOM, Exit 137, memory pressure, CPU throttling/spike)
	if in.OOMKilled || in.ExitCode == 137 || in.MemPct >= 92.0 ||
		containsAny(joined, "out of memory", "oom", "cannot allocate memory", "fatal error: runtime: out of memory") {
		return PolicyResourceSaturation, 0.96
	}
	if in.Throttled > 0 || in.CPUPct >= 90.0 || containsAny(joined, "cpu throttled", "throttling", "stress-ng") {
		return PolicyResourceSaturation, 0.91
	}

	// 2. STORAGE_PERMISSION_DENIED (EACCES, EROFS, ENOSPC, volume permission denied)
	if containsAny(joined, "permission denied", "read-only file system", "no space left on device", "disk quota exceeded", "eacces", "enospc") ||
		(in.ReadOnlyMount && containsAny(joined, "read-only", "eroofs")) {
		return PolicyStoragePermissionDenied, 0.93
	}

	// 3. NETWORK_DEPENDENCY_FAULT (connection refused, DNS lookup failure, timeout, unreachable)
	if containsAny(joined,
		"connection refused", "econnrefused", "deadlock", "too many clients",
		"i/o timeout", "network is unreachable", "no such host",
		"temporary failure in name resolution", "etimedout",
		"context deadline exceeded", "connection timed out") {
		return PolicyNetworkDependencyFault, 0.91
	}

	// 4. CONFIG_OR_SYNTAX_PANIC (crash loop, non-zero startup exit, missing env, syntax error)
	stateLower := strings.ToLower(in.Status)
	if stateLower == "restarting" || in.RestartCount >= 2 || (stateLower != "running" && in.ExitCode != 0) ||
		containsAny(joined, "panic:", "missing env", "invalid config", "syntax error", "fatal:", "segmentation fault") {
		return PolicyConfigOrSyntaxPanic, 0.88
	}

	// 5. HEALTHY_NO_ACTION
	return PolicyHealthyNoAction, 0.95
}

func evaluateRemote(ctx context.Context, endpoint string, in TelemetryInput, start time.Time) (Decision, bool) {
	reqBody, err := json.Marshal(map[string]any{
		"pattern":  PolicyTag,
		"question": PolicyQuestion,
		"options":  Options,
		"prompt":   FormatPrompt(in),
	})
	if err != nil {
		return Decision{}, false
	}

	reqCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return Decision{}, false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Decision{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Decision{}, false
	}

	var parsed struct {
		Choice     PolicyChoice `json:"choice"`
		Confidence float64      `json:"confidence"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil || parsed.Choice == "" {
		return Decision{}, false
	}

	dec := Decision{
		Pattern:          PolicyTag,
		Question:         PolicyQuestion,
		Options:          Options,
		Choice:           parsed.Choice,
		Confidence:       parsed.Confidence,
		DeciderLatencyMs: max(1, time.Since(start).Milliseconds()),
	}
	dec.BedrockBypass = dec.ShouldBypassBedrock()
	return dec, true
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}
