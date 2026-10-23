# Strands Decider 2B — Container Operational Fault `#policy` Pattern

Reusable `#policy` decision pattern (`decider_patterns/container_policy`) used by **OkShield** (`/diagnose`) to classify container operational faults in ~115ms before routing to **Amazon Bedrock**.

## Pattern Specification

- **Pattern Tag:** `#policy`
- **Typed Question:**
  > `"Given the container resource telemetry and deduplicated log patterns, what is the primary operational fault policy category?"`
- **Closed Option Set:**
  1. `RESOURCE_SATURATION` — CPU quota saturation, runaway stress process, or cgroup OOM (`ExitCode=137`).
  2. `NETWORK_DEPENDENCY_FAULT` — Downstream connection refused, DNS resolution failure, or bridge isolation.
  3. `CONFIG_OR_SYNTAX_PANIC` — Startup crash-loop, missing environment variables, or configuration syntax panic.
  4. `STORAGE_PERMISSION_DENIED` — Read-only volume mount (`EROFS`), UID/GID permission denied (`EACCES`), or disk full (`ENOSPC`).
  5. `HEALTHY_NO_ACTION` — Normal operating envelope; triggers the **Amazon Bedrock Short-Circuit** when `confidence >= 0.80`.

## Amazon Bedrock Routing & Cost Short-Circuit

```text
[Docker Telemetry + Drain Log Clusters (/whatsup)]
                       │
                       ▼
        ┌──────────────────────────────┐
        │   Strands Decider 2B (#policy)│
        │   ~115ms · Calibrated Conf   │
        └──────────────┬───────────────┘
                       │
         ┌─────────────┴─────────────┐
         │                           │
HEALTHY_NO_ACTION (≥ 0.80)    Fault Policy Detected
  Bedrock Bypass: YES          Bedrock Bypass: NO
  Immediate 0-cost return     Invoke Amazon Bedrock with
                              policy choice + confidence
                              to generate Root Cause +
                              3-command remediation Runbook
```
