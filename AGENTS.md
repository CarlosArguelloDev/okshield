# AGENTS.md — Guía Operativa y de Arquitectura para OkShield

Este documento define las reglas obligatorias para cualquier agente de IA o desarrollador que trabaje en **OkShield**. Léelo completo antes de escribir código o ejecutar comandos.

---

## 1. Visión General de OkShield
**OkShield** es una TUI (Terminal User Interface) de alto rendimiento y bajo consumo escrita en Go puro para inspección de infraestructura Docker (contenedores, redes, volúmenes), métricas en tiempo real y síntesis determinista de logs (*Drain-like clustering* con detección de anomalías vía `/whatsup`). Funciona 100% en memoria, con cero escrituras a disco, protegiendo medios flash (como tarjetas MicroSD en Raspberry Pi) y consumiendo menos de 20MB de RAM.

---

## 2. Regla #1: Flujo de Trabajo Micro-Iterativo y Commits Atómicos
- **PROHIBIDO hacer commits gigantes.** Una funcionalidad solicitada por el usuario DEBE dividirse en múltiples commits pequeños y incrementales.
- **Secuencia obligatoria para cada tarea:**
  1. Implementar un sub-componente mínimo (ej. definir structs, crear interfaz, agregar estilo, crear query).
  2. Verificar que compila (`go build ./...`) y pasa tests (`go test ./...`).
  3. Hacer commit inmediato usando **Conventional Commits**:
     - `feat(tui): add rounded border layout inspired by antigravity`
     - `feat(docker): implement container network and volume inspector`
     - `feat(summarizer): add deterministic log fingerprinting without AI`
  4. Continuar con el siguiente sub-paso de la misma solicitud.

---

## 3. Arquitectura del Sistema
```text
+-----------------------------------------------------------------------+
|                       OkShield TUI (Bubble Tea)                       |
|  [Header: Host/RAM] [Sidebar: Containers/Net/Vol] [Main: Resumen/Logs]|
|  [Command Bar: /whatsup | /containers | /networks | /volumes | /lang]  |
+-----------------------------------+-----------------------------------+
                                    |
                                    | tea.Msg / tea.Cmd (Async)
                                    v
+-----------------------------------------------------------------------+
|                     Core Processing Layer (Go)                        |
|  Deterministic Summarizer (Sin IA / 100% Go):                         |
|  - Drain-like Log Clustering (extrae plantillas de miles de logs)     |
|  - Correlador de estados (ExitCodes, OOM, Restart Counts, Spikes)     |
|  - Reporte compacto de salud y anomalías por contenedor               |
+-----------------------------------+-----------------------------------+
                                    |
                                    v
+-----------------------------------------------------------------------+
|                     Docker Engine API (/var/run)                      |
|  - Contenedores, Redes, Volúmenes                                     |
|  - Stats Stream (CPU/RAM/Net/Disk IO) & Tail Log Stream               |
|  - 100% en memoria: cero dependencias CGO, cero escrituras a disco   |
+-----------------------------------------------------------------------+
```

## 4. Motores de Análisis: Resumen Determinista (`/whatsup`)
OkShield sintetiza lo que ocurre con algoritmos deterministas en Go puro:
1. **Normalización y Clustering de Logs:** Reemplazar UUIDs, IPs, timestamps, rutas y números por comodines (`<IP>`, `<NUM>`, `<HEX>`) para agrupar 10,000 líneas repetitivas en patrones únicos con su contador de frecuencia y severidad (`[ERROR] x432: Connection refused to <IP>:<PORT>`).
2. **Detección de Anomalías Estadísticas:** Detectar picos de CPU/RAM y cambios de estado en Docker (`Restarting`, `OOMKilled`, `ExitCode != 0`, CPU throttling).

---

## 5. Presupuesto Estricto de Memoria (Low Footprint Rules)
Dado que OkShield corre incluso en SBCs como Raspberry Pi:
1. **Límite de RAM:** El proceso de Go debe mantenerse por debajo de 20MB en uso normal.
2. **Ring Buffers y Vistas acotadas:** Nunca usar slices que crezcan indefinidamente (`append` sin límite) para guardar logs en la TUI. Máximo 500 líneas renderizables por contenedor en el `viewport`.
3. **Cero escrituras a disco:** Operación 100% en memoria para no degradar memorias flash o tarjetas MicroSD.
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
  - Compilar para ARM64 (`CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/okshield-linux-arm64 ./cmd/okshield`).
  - Puedes levantar contenedores ligeros de prueba en la Pi (`alpine`, `nginx`, `redis`) simulando fallos para verificar que `/whatsup` y las métricas responden correctamente.

---

## 8. Estructura del Repositorio y Convenciones de Código
```text
cmd/okshield/        -> main (wiring, flags, arranque de la TUI)
internal/tui/        -> modelos Bubble Tea, estilos lipgloss, overlays, command bar
internal/docker/     -> cliente Docker SDK (contenedores, redes, volúmenes, stats, logs)
internal/summarizer/ -> normalización + clustering drain-like + detección de anomalías
internal/version/    -> metadata de build (inyectada con -ldflags)
```
- **Todo bajo `internal/`** salvo `cmd/`: no exponemos API pública.
- **I/O siempre asíncrono:** Docker se ejecuta como `tea.Cmd` o goroutines que emiten `tea.Msg`. `Update()` y `View()` jamás bloquean.
- **Context first:** toda función que haga I/O recibe `context.Context` como primer parámetro.
- **Tests:** cada paquete de lógica pura (`summarizer`, `docker`, `tui`) debe tener tests de tabla; incluir benchmarks para el hot path.
- **Calidad antes de commit:** `gofmt -l .` vacío, `go vet ./...`, `go build ./...` y `go test ./...` en verde.

## 9. Secretos y Entorno Local
- `.env.example` (trackeado) documenta las variables necesarias sin valores reales.
- `.env` (ignorado por Git) contiene los valores reales; nunca se commitea, ni se copia a logs, commits, issues o mensajes.
- Compilación cruzada para la Pi: `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/okshield-linux-arm64 ./cmd/okshield` (100% Go puro, sin dependencias CGO ni toolchain cruzado de C++).
