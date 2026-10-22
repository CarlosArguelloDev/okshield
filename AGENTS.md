# AGENTS.md — Guía Operativa y de Arquitectura para OkShield

Este documento define las reglas obligatorias para cualquier agente de IA o desarrollador que trabaje en **OkShield**. Léelo completo antes de escribir código o ejecutar comandos.

---

## 1. Visión General de OkShield
**OkShield** es una TUI (Terminal User Interface) de alto rendimiento escrita en Go para visualización de infraestructura Docker (contenedores, redes, volúmenes), ingesta de telemetría vía OpenTelemetry + DuckDB, síntesis determinista de logs y diagnóstico de errores basado en modelos de decisión.

---

## 2. Regla #1: Flujo de Trabajo Micro-Iterativo y Commits Atómicos
- **PROHIBIDO hacer commits gigantes.** Una funcionalidad solicitada por el usuario DEBE dividirse en múltiples commits pequeños y incrementales.
- **Secuencia obligatoria para cada tarea:**
  1. Implementar un sub-componente mínimo (ej. definir structs, crear interfaz, agregar estilo, crear query).
  2. Verificar que compila (`go build ./...`) y pasa tests (`go test ./...`).
  3. Hacer commit inmediato usando **Conventional Commits**:
     - `feat(tui): add rounded border layout inspired by antigravity`
     - `feat(docker): implement container network and volume inspector`
     - `feat(telemetry): add deterministic log fingerprinting without AI`
     - `perf(duckdb): implement batch writer with sync.Pool buffer`
  4. Continuar con el siguiente sub-paso de la misma solicitud.

---

## 3. Arquitectura del Sistema
```text
+-----------------------------------------------------------------------+
|                       OkShield TUI (Bubble Tea)                       |
|  [Header: Host/RAM] [Sidebar: Containers/Net/Vol] [Main: Pulse/Diag]  |
|  [Command Bar / Overlay: /whatsup | /diagnose | /networks | /volumes] |
+-----------------------------------+-----------------------------------+
                                    |
                                    | tea.Msg / tea.Cmd (Async)
                                    v
+-----------------------------------------------------------------------+
|                     Core Processing Layer (Go)                        |
|  1. Deterministic Summarizer (Sin IA):                                |
|     - Drain-like Log Clustering (extrae plantillas de miles de logs)  |
|     - Correlador de estados (ExitCodes, OOM, Restart Counts, Spikes)  |
|  2. Decision & Classification Engine (IA / Decision Models):          |
|     - Recibe SOLO el resumen compacto (nunca logs crudos masivos)     |
|     - Clasifica la anomalía por taxonomía de error                    |
|     - Genera árbol de causa raíz + Runbook de solución                |
+-----------------+-----------------------------------+-----------------+
                  |                                   |
                  v                                   v
+-----------------------------------+ +---------------------------------+
|   Docker Engine API (/var/run)    | |   OTLP Receiver + DuckDB        |
| - Contenedores, Redes, Volúmenes  | | - gRPC :4317 / HTTP :4318       |
| - Stats Stream & Log Stream       | | - Ring Buffer en RAM (Hot Path) |
|                                   | | - DuckDB Batch Appender (Warm)  |
+-----------------------------------+ +---------------------------------+
```

## 4. Diseño de los Motores de Análisis

### Fase A: Resumen Determinista (Sin IA) — Comando `/whatsup`
Antes de usar cualquier modelo de IA, OkShield sintetiza lo que ocurre con algoritmos puros en Go:
1. **Normalización y Clustering de Logs:** Reemplazar UUIDs, IPs, timestamps, rutas y números por comodines (`<IP>`, `<NUM>`, `<HEX>`) para agrupar 10,000 líneas repetitivas en 5 patrones únicos con su contador de frecuencia y severidad (`[ERROR] x432: Connection refused to <IP>:<PORT>`).
2. **Detección de Anomalías Estadísticas:** Detectar picos de CPU/RAM (Z-score o umbrales sobre media móvil) y cambios de estado en Docker (`Restarting`, `OOMKilled`, `ExitCode != 0`).

### Fase B: Clasificación y Diagnóstico (Decision Models) — Comando `/diagnose`
Una vez sintetizado el problema, el clasificador evalúa el vector de características `(LogPattern, ExitCode, MemoryDelta, NetworkErrors, VolumeMounts)` para clasificar el incidente en categorías accionables:
- `MEMORY_EXHAUSTION` (OOM / Memory Leak)
- `NETWORK_UNREACHABLE` (DNS fail, Port mismatch, Bridge isolation)
- `STORAGE_PERMISSION_OR_FULL` (Volume mount error, Read-only FS, Disk full)
- `DEPENDENCY_TIMEOUT` (DB/Cache down, Deadlock)
- `CONFIG_PANIC` (Missing ENV, Invalid syntax on startup)
Cada clasificación devuelve una **Guía de Resolución** estructurada (Qué pasó -> Por qué pasó -> Comandos exactos para arreglarlo).

---

## 5. Presupuesto Estricto de Memoria (Low Footprint Rules)
Dado que OkShield corre incluso en SBCs como Raspberry Pi:
1. **Límite de RAM:** El proceso de Go debe mantenerse por debajo de 50MB en uso normal.
2. **Ring Buffers fijos:** Nunca usar slices que crezcan indefinidamente (`append` sin límite) para guardar logs en la TUI. Máximo 500 líneas renderizables por contenedor en el `viewport`.
3. **DuckDB controlado:** Configurar `SET memory_limit = '64MB';` y `SET threads = 2;` al inicializar la conexión en DuckDB.
4. **Cero fugas de Goroutines:** Todo canal y stream de Docker (`ContainerLogs`, `ContainerStats`) debe cerrarse con `context.WithCancel` al cambiar de vista o salir de la app.

---

## 6. Guía de Estilo Visual (Superfile & Antigravity CLI Inspired)
- Usar bordes estilizados con títulos y contadores incrustados en los marcos y `lipgloss.RoundedBorder()` para modales superpuestos.
- **Sistema de Labels / Badges de Estado (tipo píldora):** Reemplazar glifos sueltos por etiquetas rectangulares compactas con fondo de color sólido y texto en negrita de alto contraste:
  - **` OK ` / ` RUN `**: Fondo verde (`#06D6A0`) con texto oscuro (`#1D2021`) para contenedores activos y sistemas sanos.
  - **` WARN `**: Fondo ámbar (`#FFD166`) con texto oscuro (`#1D2021`) para advertencias o recursos degradados.
  - **` ERR ` / ` DOWN `**: Fondo rojo coral (`#EF476F`) con texto blanco (`#FFFFFF`) para errores críticos, caídas o `OOMKilled`.
  - **` INFO `**: Fondo cian/azul (`#00F5D4`) con texto oscuro (`#1D2021`) para estados informativos.
- Mostrar siempre el consumo de memoria actual de OkShield (`runtime.MemStats.Alloc`) en la interfaz.

---

## 7. Entorno de Pruebas Remoto (Raspberry Pi)
- Las pruebas de integración con contenedores reales y medición de consumo de recursos se ejecutan en la Raspberry Pi local (host y usuario definidos en `RPI_HOST` / `RPI_USER`).
- **Seguridad:** NUNCA escribir contraseñas ni IPs privadas dentro de archivos `.go` o scripts trackeados por Git. Leer siempre desde variables de entorno (`.env` incluido en `.gitignore`): `RPI_HOST`, `RPI_USER`, `RPI_PASS`.
- Para desplegar y probar en la Raspberry Pi:
  - Compilar para ARM64 (`GOOS=linux GOARCH=arm64`).
  - Puedes levantar contenedores ligeros de prueba en la Pi (`alpine`, `nginx`, `redis`) simulando fallos (ej. `stress-ng` para límite de memoria, comandos que fallen con exit 1, redes desconectadas) para verificar que `/whatsup` y `/diagnose` detectan y resumen todo correctamente.


---

## 8. Estructura del Repositorio y Convenciones de Código
```text
cmd/okshield/        -> main (wiring, flags, arranque de la TUI)
internal/tui/        -> modelos Bubble Tea, estilos lipgloss, overlays, command bar
internal/docker/     -> cliente Docker SDK (contenedores, redes, volúmenes, stats, logs)
internal/telemetry/  -> receptor OTLP, ring buffer, batch writer DuckDB, TTL
internal/summarizer/ -> normalización + clustering drain-like + detección de anomalías
internal/diagnose/   -> decision models, taxonomía de errores, runbooks
internal/version/    -> metadata de build (inyectada con -ldflags)
```
- **Todo bajo `internal/`** salvo `cmd/`: no exponemos API pública.
- **I/O siempre asíncrono:** Docker, DuckDB, OTLP e inferencia se ejecutan como `tea.Cmd` o goroutines que emiten `tea.Msg`. `Update()` y `View()` jamás bloquean.
- **Context first:** toda función que haga I/O recibe `context.Context` como primer parámetro.
- **Hot path sin allocations:** usar `sync.Pool` para buffers de parsing y registros OTLP; preasignar slices con capacidad fija.
- **Tests:** cada paquete de lógica pura (`summarizer`, `diagnose`, ring buffer) debe tener tests de tabla; incluir benchmarks (`go test -bench . -benchmem`) para el hot path.
- **Calidad antes de commit:** `gofmt -l .` vacío, `go vet ./...`, `go build ./...` y `go test ./...` en verde.

## 9. Secretos y Entorno Local
- `.env.example` (trackeado) documenta las variables necesarias sin valores reales.
- `.env` (ignorado por Git) contiene los valores reales; nunca se commitea, ni se copia a logs, commits, issues o mensajes.
- Compilación cruzada para la Pi: `GOOS=linux GOARCH=arm64 CGO_ENABLED=1` (DuckDB requiere CGO; usar toolchain cruzado o compilar en la propia Pi).
