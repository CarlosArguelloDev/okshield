package container_policy

import (
	"context"
	"strings"
	"testing"
)

func TestEvaluateAllPolicyChoices(t *testing.T) {
	tests := []struct {
		name       string
		in         TelemetryInput
		wantChoice PolicyChoice
		wantBypass bool
	}{
		{
			name: "resource_saturation_cpu",
			in: TelemetryInput{
				Container: "billing-worker",
				Status:    "running",
				CPUPct:    96.4,
				MemPct:    22.0,
				Throttled: 34,
			},
			wantChoice: PolicyResourceSaturation,
			wantBypass: false,
		},
		{
			name: "resource_saturation_oom",
			in: TelemetryInput{
				Container: "mem-hog",
				Status:    "exited",
				ExitCode:  137,
				OOMKilled: true,
				MemPct:    99.1,
			},
			wantChoice: PolicyResourceSaturation,
			wantBypass: false,
		},
		{
			name: "network_dependency_fault",
			in: TelemetryInput{
				Container:   "auth-api",
				Status:      "running",
				LogPatterns: []string{"[ERROR] x42: dial tcp <IP>:<PORT>: connect: connection refused"},
			},
			wantChoice: PolicyNetworkDependencyFault,
			wantBypass: false,
		},
		{
			name: "config_or_syntax_panic",
			in: TelemetryInput{
				Container:    "broken-config",
				Status:       "restarting",
				ExitCode:     2,
				RestartCount: 5,
				LogPatterns:  []string{"panic: missing ENV variable JWT_SECRET"},
			},
			wantChoice: PolicyConfigOrSyntaxPanic,
			wantBypass: false,
		},
		{
			name: "storage_permission_denied",
			in: TelemetryInput{
				Container:     "storage-sync",
				Status:        "exited",
				ExitCode:      1,
				ReadOnlyMount: true,
				LogPatterns:   []string{"ERROR: cannot write /data/state.db: read-only file system"},
			},
			wantChoice: PolicyStoragePermissionDenied,
			wantBypass: false,
		},
		{
			name: "healthy_no_action_short_circuit",
			in: TelemetryInput{
				Container:   "web-gateway",
				Status:      "running",
				CPUPct:      1.8,
				MemPct:      11.2,
				LogPatterns: []string{"[INFO] x12: GET /healthz 200"},
			},
			wantChoice: PolicyHealthyNoAction,
			wantBypass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := Evaluate(context.Background(), tt.in)
			if dec.Choice != tt.wantChoice {
				t.Errorf("Choice = %q, want %q", dec.Choice, tt.wantChoice)
			}
			if dec.BedrockBypass != tt.wantBypass {
				t.Errorf("BedrockBypass = %v, want %v (conf=%.2f)", dec.BedrockBypass, tt.wantBypass, dec.Confidence)
			}
			if dec.Confidence < 0.80 || dec.Confidence > 1.0 {
				t.Errorf("unexpected Confidence = %.2f", dec.Confidence)
			}
			if dec.DeciderLatencyMs <= 0 {
				t.Errorf("expected positive DeciderLatencyMs, got %d", dec.DeciderLatencyMs)
			}
			if dec.Question != PolicyQuestion {
				t.Errorf("Question = %q, want %q", dec.Question, PolicyQuestion)
			}
		})
	}
}

func TestFormatPromptIncludesPolicyTagAndOptions(t *testing.T) {
	p := FormatPrompt(TelemetryInput{
		Container:   "auth-api",
		Status:      "running",
		LogPatterns: []string{"[ERROR] x10: connection refused"},
	})
	for _, want := range []string{PolicyTag, PolicyQuestion, "RESOURCE_SATURATION", "HEALTHY_NO_ACTION", "auth-api"} {
		if !strings.Contains(p, want) {
			t.Errorf("FormatPrompt missing %q", want)
		}
	}
}
