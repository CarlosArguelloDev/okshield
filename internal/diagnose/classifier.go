// Package diagnose implements the lightweight decision-tree classifier and
// remediation runbook generator for the /diagnose command. It evaluates only
// compact feature vectors produced by the deterministic summarizer.
package diagnose

import (
	"fmt"
	"strings"

	"github.com/CarlosArguelloDev/okshield/internal/docker"
	"github.com/CarlosArguelloDev/okshield/internal/summarizer"
)

// Category is the canonical incident taxonomy (AGENTS.md §4B & spec §1.4).
type Category string

const (
	CatMemoryExhaustion  Category = "MEMORY_EXHAUSTION"          // OOMKilled / memory leak
	CatNetworkUnreach    Category = "NETWORK_UNREACHABLE"        // NetworkTimeout / DNS / bridge
	CatStoragePermission Category = "STORAGE_PERMISSION_OR_FULL" // VolumePermissionError / RO FS / disk full
	CatDependencyTimeout Category = "DEPENDENCY_TIMEOUT"         // DBConnectionRefused / upstream down
	CatCPUThrottling     Category = "CPU_THROTTLING"             // CPUThrottling / quota saturated
	CatConfigPanic       Category = "CONFIG_PANIC"               // CrashLoop / missing ENV / panic on startup
	CatHealthy           Category = "HEALTHY"
)

// Diagnosis is the structured output of the decision model for one container.
type Diagnosis struct {
	Container     string
	Category      Category
	FaultLabel    string // e.g. OOMKilled, NetworkTimeout, VolumePermissionError, DBConnectionRefused, CPUThrottling, CrashLoop
	Confidence    float64
	WhatHappened  string
	WhyItHappened string
	RootCauseTree []string // visual tree nodes explaining the decision path
	Runbook       []string // exact commands to remediate the issue
}

// Classify evaluates the feature vector of a container and returns a
// deterministic Diagnosis with a root-cause decision tree and runbook.
func Classify(in summarizer.ContainerInput) Diagnosis {
	d := in.Detail
	st := in.Stats
	name := d.Name
	if name == "" {
		name = d.ShortID()
	}
	clusters := summarizer.ClusterLogs(in.Logs)
	joined := joinTemplates(clusters)

	// 1. MEMORY_EXHAUSTION (OOMKilled / Exit 137 / Memory pressure)
	if d.OOMKilled || d.ExitCode == 137 || st.MemPercent >= 92.0 ||
		containsAny(joined, "out of memory", "oom", "cannot allocate memory", "fatal error: runtime: out of memory") {
		limitMB := int64(256)
		if d.MemoryLimit > 0 {
			limitMB = (d.MemoryLimit >> 20) * 2
		}
		return Diagnosis{
			Container:     name,
			Category:      CatMemoryExhaustion,
			FaultLabel:    "OOMKilled",
			Confidence:    0.96,
			WhatHappened:  fmt.Sprintf("El contenedor %q agotó su cuota de RAM (ExitCode=%d, OOMKilled=%v, RAM=%.1f%%).", name, d.ExitCode, d.OOMKilled, st.MemPercent),
			WhyItHappened: "El kernel Linux invocó al OOM killer (SIGKILL / 137) porque el working set superó el límite cgroup configurado o existe una fuga de memoria.",
			RootCauseTree: []string{
				fmt.Sprintf("├─ Estado: %s (ExitCode=%d, OOMKilled=%v)", d.State, d.ExitCode, d.OOMKilled),
				fmt.Sprintf("├─ Uso RAM: %.1f%% (límite actual: %d MB)", st.MemPercent, d.MemoryLimit>>20),
				"└─ Clasificación: MEMORY_EXHAUSTION [OOMKilled]",
			},
			Runbook: []string{
				fmt.Sprintf("docker inspect %s --format '{{.State.OOMKilled}} {{.HostConfig.Memory}}'", name),
				fmt.Sprintf("docker update --memory %dm --memory-swap %dm %s", limitMB, limitMB*2, name),
				fmt.Sprintf("docker restart %s", name),
			},
		}
	}

	// 2. STORAGE_PERMISSION_OR_FULL (Volume permission error / Read-only mount / Disk full)
	if hasStorageError(joined) || (hasReadOnlyMount(d.Mounts) && containsAny(joined, "read-only", "permission denied", "eacces", "eroofs")) {
		mountDest := "/data"
		if len(d.Mounts) > 0 {
			mountDest = d.Mounts[0].Destination
		}
		return Diagnosis{
			Container:     name,
			Category:      CatStoragePermission,
			FaultLabel:    "VolumePermissionError",
			Confidence:    0.92,
			WhatHappened:  fmt.Sprintf("El contenedor %q falló al escribir en disco o volumen montado (%s).", name, mountDest),
			WhyItHappened: "El volumen/bind mount está montado como solo lectura (RO), el UID del proceso no tiene permisos en el host o el filesystem alcanzó el 100%.",
			RootCauseTree: []string{
				fmt.Sprintf("├─ Mounts detectados: %d (destino principal: %s)", len(d.Mounts), mountDest),
				"├─ Patrón de log: EACCES / Read-only file system / No space left",
				"└─ Clasificación: STORAGE_PERMISSION_OR_FULL [VolumePermissionError]",
			},
			Runbook: []string{
				fmt.Sprintf("docker inspect %s --format '{{json .Mounts}}'", name),
				fmt.Sprintf("docker exec %s df -h %s", name, mountDest),
				fmt.Sprintf("docker exec -u 0 %s chown -R $(id -u):$(id -g) %s", name, mountDest),
			},
		}
	}

	// 3. DEPENDENCY_TIMEOUT (DB/Cache connection refused or upstream deadlock)
	if containsAny(joined, "connection refused", "econnrefused", "deadlock", "too many clients", "redis", "postgres", "mysql", "dial tcp") &&
		containsAny(joined, "refused", "timeout", "deadlock", "unreachable", "failed to connect") {
		return Diagnosis{
			Container:     name,
			Category:      CatDependencyTimeout,
			FaultLabel:    "DBConnectionRefused",
			Confidence:    0.90,
			WhatHappened:  fmt.Sprintf("El contenedor %q no puede conectarse a una dependencia aguas abajo (DB/Cache/API).", name),
			WhyItHappened: "El servicio destino (PostgreSQL/Redis/upstream) está caído, no escucha en la interfaz correcta o está en una red Docker distinta.",
			RootCauseTree: []string{
				fmt.Sprintf("├─ Reinicios: %d (ExitCode=%d)", d.RestartCount, d.ExitCode),
				"├─ Patrón de log: Connection refused / upstream timeout hacia <IP>:<PORT>",
				"└─ Clasificación: DEPENDENCY_TIMEOUT [DBConnectionRefused]",
			},
			Runbook: []string{
				fmt.Sprintf("docker inspect %s --format '{{json .NetworkSettings.Networks}}'", name),
				"docker network ls && docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'",
				fmt.Sprintf("docker exec %s sh -c 'nslookup db || nc -zv db 5432'", name),
			},
		}
	}

	// 4. NETWORK_UNREACHABLE (DNS lookup failure, network timeout, bridge isolation)
	if containsAny(joined, "i/o timeout", "network is unreachable", "no such host", "temporary failure in name resolution", "etimedout", "context deadline exceeded", "connection timed out") ||
		(len(d.Networks) == 0 && d.Running) {
		return Diagnosis{
			Container:     name,
			Category:      CatNetworkUnreach,
			FaultLabel:    "NetworkTimeout",
			Confidence:    0.88,
			WhatHappened:  fmt.Sprintf("El contenedor %q presenta fallos de conectividad de red o resolución DNS.", name),
			WhyItHappened: "El contenedor está desconectado del bridge esperado, el DNS interno (127.0.0.11) falla o un firewall bloquea el tráfico saliente.",
			RootCauseTree: []string{
				fmt.Sprintf("├─ Redes adjuntas: %d", len(d.Networks)),
				"├─ Patrón de log: DNS / I/O timeout / Network unreachable",
				"└─ Clasificación: NETWORK_UNREACHABLE [NetworkTimeout]",
			},
			Runbook: []string{
				fmt.Sprintf("docker network connect bridge %s", name),
				fmt.Sprintf("docker exec %s cat /etc/resolv.conf", name),
				fmt.Sprintf("docker inspect %s --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}}={{$v.IPAddress}} {{end}}'", name),
			},
		}
	}

	// 5. CPU_THROTTLING (CFS bandwidth quota throttling or extreme CPU spike)
	if st.Throttled > 0 || st.CPUPercent >= 90.0 || containsAny(joined, "cpu throttled", "throttling") {
		return Diagnosis{
			Container:     name,
			Category:      CatCPUThrottling,
			FaultLabel:    "CPUThrottling",
			Confidence:    0.89,
			WhatHappened:  fmt.Sprintf("El contenedor %q sufre estrangulamiento de CPU (CPU=%.1f%%, periodos throttled=%d).", name, st.CPUPercent, st.Throttled),
			WhyItHappened: "El planificador CFS del kernel está limitando los ciclos del contenedor por superar su cuota NanoCPUs o por un bucle intensivo.",
			RootCauseTree: []string{
				fmt.Sprintf("├─ Uso CPU: %.1f%% (NanoCPUs=%d)", st.CPUPercent, d.NanoCPUs),
				fmt.Sprintf("├─ Periodos CFS throttled: %d", st.Throttled),
				"└─ Clasificación: CPU_THROTTLING [CPUThrottling]",
			},
			Runbook: []string{
				fmt.Sprintf("docker stats --no-stream %s", name),
				fmt.Sprintf("docker update --cpus 2.0 %s", name),
				fmt.Sprintf("docker top %s", name),
			},
		}
	}

	// 6. CONFIG_PANIC / CrashLoop (non-zero exit, restarting, missing env or syntax error)
	if d.Restarting || d.RestartCount >= 2 || (!d.Running && d.ExitCode != 0) ||
		containsAny(joined, "panic:", "missing env", "invalid config", "syntax error", "fatal:", "segmentation fault") {
		return Diagnosis{
			Container:     name,
			Category:      CatConfigPanic,
			FaultLabel:    "CrashLoop",
			Confidence:    0.85,
			WhatHappened:  fmt.Sprintf("El contenedor %q aborta al iniciar (ExitCode=%d, Reinicios=%d).", name, d.ExitCode, d.RestartCount),
			WhyItHappened: "Falta una variable de entorno obligatoria, un archivo de configuración tiene sintaxis inválida o el binario lanzó un panic al arrancar.",
			RootCauseTree: []string{
				fmt.Sprintf("├─ Estado: %s (ExitCode=%d, RestartCount=%d)", d.State, d.ExitCode, d.RestartCount),
				fmt.Sprintf("├─ Comando: %v (vars de entorno: %d)", d.Cmd, d.EnvCount),
				"└─ Clasificación: CONFIG_PANIC [CrashLoop]",
			},
			Runbook: []string{
				fmt.Sprintf("docker logs --tail 50 %s", name),
				fmt.Sprintf("docker inspect %s --format '{{json .Config.Cmd}} {{json .State}}'", name),
				fmt.Sprintf("docker run --rm -it --entrypoint sh %s", d.Image),
			},
		}
	}

	return Diagnosis{
		Container:     name,
		Category:      CatHealthy,
		FaultLabel:    "Healthy",
		Confidence:    0.99,
		WhatHappened:  fmt.Sprintf("El contenedor %q opera dentro de parámetros normales.", name),
		WhyItHappened: "No se detectaron códigos de salida anómalos, OOM, throttling ni patrones de error críticos en el ring buffer.",
		RootCauseTree: []string{
			fmt.Sprintf("├─ Estado: %s (CPU=%.1f%%, RAM=%.1f%%)", d.State, st.CPUPercent, st.MemPercent),
			"└─ Clasificación: HEALTHY",
		},
		Runbook: []string{
			fmt.Sprintf("docker stats --no-stream %s", name),
		},
	}
}

func joinTemplates(clusters []summarizer.LogCluster) string {
	var b strings.Builder
	for _, c := range clusters {
		b.WriteString(strings.ToLower(c.Template))
		b.WriteByte('\n')
		b.WriteString(strings.ToLower(c.Sample))
		b.WriteByte('\n')
	}
	return b.String()
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

func hasStorageError(joined string) bool {
	return containsAny(joined,
		"permission denied",
		"read-only file system",
		"no space left on device",
		"disk quota exceeded",
		"eacces",
		"enospc",
	)
}

func hasReadOnlyMount(mounts []docker.Mount) bool {
	for _, m := range mounts {
		if !m.RW {
			return true
		}
	}
	return false
}
