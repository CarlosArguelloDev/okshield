package diagnose

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/CarlosArguelloDev/okshield/decider_patterns/container_policy"
	"github.com/CarlosArguelloDev/okshield/internal/summarizer"
)

// DefaultBedrockModel is the default Amazon Bedrock Converse model identifier.
const DefaultBedrockModel = "us.amazon.nova-lite-v1:0"

// BuildBedrockPrompt constructs the Amazon Bedrock prompt injected with the
// Strands Decider 2B `#policy` classification choice and calibrated confidence.
// For RESOURCE_SATURATION, it explicitly instructs the model to audit and
// contain runaway/abusive processes (`docker top`, capping `--cpus`) instead of
// rewarding stress workloads with more CPU.
func BuildBedrockPrompt(dec container_policy.Decision, in summarizer.ContainerInput, logPatterns []string) string {
	d := in.Detail
	st := in.Stats
	name := d.Name
	if name == "" {
		name = d.ShortID()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "You are an SRE incident responder integrated with Strands Decider 2B (%s).\n", container_policy.PolicyTag)
	fmt.Fprintf(&b, "Strands Decider 2B evaluated:\n  Question: %q\n  Policy Choice: %s\n  Calibrated Confidence: %.2f\n  Latency: %dms\n\n",
		dec.Question, dec.Choice, dec.Confidence, dec.DeciderLatencyMs)

	fmt.Fprintf(&b, "Container Telemetry:\n")
	fmt.Fprintf(&b, "  name: %s (image: %s, state: %s, exit_code: %d, oom_killed: %v, restarts: %d)\n",
		name, d.Image, d.State, d.ExitCode, d.OOMKilled, d.RestartCount)
	fmt.Fprintf(&b, "  cpu_pct: %.1f%% (throttled_periods: %d) | mem_pct: %.1f%%\n",
		st.CPUPercent, st.Throttled, st.MemPercent)
	if len(logPatterns) > 0 {
		fmt.Fprintf(&b, "  deduplicated_log_patterns:\n")
		for _, p := range logPatterns {
			fmt.Fprintf(&b, "    - %s\n", p)
		}
	}

	b.WriteString("\nPolicy Directive:\n")
	switch dec.Choice {
	case container_policy.PolicyResourceSaturation:
		b.WriteString("  - Policy is RESOURCE_SATURATION. Prioritize auditing and containing runaway or abusive processes (e.g. `docker top`, capping CPU quota with `docker update --cpus` or memory bounds) rather than granting extra CPU to stress workloads.\n")
	case container_policy.PolicyNetworkDependencyFault:
		b.WriteString("  - Policy is NETWORK_DEPENDENCY_FAULT. Prioritize inspecting container networks, DNS resolution, and upstream service reachability.\n")
	case container_policy.PolicyConfigOrSyntaxPanic:
		b.WriteString("  - Policy is CONFIG_OR_SYNTAX_PANIC. Prioritize inspecting startup logs, missing environment variables, and entrypoint syntax.\n")
	case container_policy.PolicyStoragePermissionDenied:
		b.WriteString("  - Policy is STORAGE_PERMISSION_DENIED. Prioritize checking read-only volume mounts, filesystem capacity (`df -h`), and UID/GID ownership.\n")
	}
	b.WriteString("Return JSON with keys: what_happened, why_it_happened, and runbook (array of 3 exact docker CLI remediation commands).")
	return b.String()
}

type bedrockResponse struct {
	WhatHappened  string   `json:"what_happened"`
	WhyItHappened string   `json:"why_it_happened"`
	Runbook       []string `json:"runbook"`
}

// invokeBedrock calls Amazon Bedrock Converse API when AWS_BEARER_TOKEN_BEDROCK
// or BEDROCK_ENDPOINT_URL is configured in the environment. Returns false when
// unconfigured or unreachable so the caller uses the policy-guided local runbook.
func invokeBedrock(ctx context.Context, prompt string) (bedrockResponse, bool) {
	token := strings.TrimSpace(os.Getenv("AWS_BEARER_TOKEN_BEDROCK"))
	endpoint := strings.TrimSpace(os.Getenv("BEDROCK_ENDPOINT_URL"))
	if token == "" && endpoint == "" {
		return bedrockResponse{}, false
	}

	if endpoint == "" {
		region := strings.TrimSpace(os.Getenv("AWS_REGION"))
		if region == "" {
			region = "us-east-1"
		}
		modelID := strings.TrimSpace(os.Getenv("BEDROCK_MODEL_ID"))
		if modelID == "" {
			modelID = DefaultBedrockModel
		}
		endpoint = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/converse", region, modelID)
	}

	payload, err := json.Marshal(map[string]any{
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"text": prompt},
				},
			},
		},
		"inferenceConfig": map[string]any{
			"maxTokens":   350,
			"temperature": 0.1,
		},
	})
	if err != nil {
		return bedrockResponse{}, false
	}

	reqCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return bedrockResponse{}, false
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return bedrockResponse{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return bedrockResponse{}, false
	}

	var raw struct {
		Output struct {
			Message struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return bedrockResponse{}, false
	}
	if len(raw.Output.Message.Content) == 0 {
		return bedrockResponse{}, false
	}

	var parsed bedrockResponse
	if err := json.Unmarshal([]byte(raw.Output.Message.Content[0].Text), &parsed); err != nil {
		return bedrockResponse{}, false
	}
	if len(parsed.Runbook) == 0 {
		return bedrockResponse{}, false
	}
	return parsed, true
}
