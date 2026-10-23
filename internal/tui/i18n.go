package tui

import "strings"

// Lang identifies the UI display language.
type Lang int

const (
	LangES Lang = iota // Español (default)
	LangEN             // English
)

// Code returns the uppercase 2-letter ISO code ("ES" or "EN").
func (l Lang) Code() string {
	if l == LangEN {
		return "EN"
	}
	return "ES"
}

// ParseLang parses a language code ("en", "english", "es", "espanol", "español").
// It returns (Lang, true) on a recognized language or (LangES, false) otherwise.
func ParseLang(s string) (Lang, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "en", "eng", "english", "ingles", "inglés":
		return LangEN, true
	case "es", "esp", "spanish", "espanol", "español":
		return LangES, true
	default:
		return LangES, false
	}
}

// Catalog holds all localized strings for a single language.
type Catalog struct {
	Starting         string
	ConnectingDocker string
	DockerConnected  string
	DockerDown       string
	Loading          string
	Empty            string
	NoSelection      string
	NoResource       string

	Views   [3]string
	Tabs    [5]string
	Metrics [4]string

	CtrlScroll   string
	CtrlViews    string
	CtrlTabs     string
	CtrlFocus    string
	CtrlWhatsup  string
	CtrlDiagnose string
	CtrlLang     string
	CtrlCommand  string
	CtrlQuit     string

	NoAnomalies     string
	HelpPlaceholder string
	HelpTitle       string
	HelpPromptLabel string
	HelpAvailTitle  string
	HelpDescTitle   string
	HelpFooterCmd   string
	HelpFooterAny   string

	PulseHint         string
	PulseTitle        string
	PulseNoContainers string

	MetricsNoContainers string
	MetricsTitlePrefix  string
	MetricsSwitchHint   string
	MetricsNow          string
	MetricsRead         string
	MetricsWrite        string

	GeneralSummaryTitle string
	GeneralContainers   string
	GeneralNetworks     string
	GeneralVolumes      string
	SectionPorts        string
	SectionNetworks     string
	SectionMounts       string
	Unpublished         string
	ConnectedContainers string
	None                string
	UsedBy              string
	OrphanVolume        string
	NoLimit             string
	Yes                 string

	FieldImage    string
	FieldState    string
	FieldRestarts string
	FieldPolicy   string
	FieldRAMLimit string
	FieldCPULimit string
	FieldInternal string
	FieldCreated  string

	LogsSelectHint string
	LogsTitle      string
	LogsRingCount  string
	LogsEmpty      string
	LogsRecent     string

	DiagHint       string
	DiagTitle      string
	DiagConfidence string
	DiagDeciderHdr string
	DiagCalibConf  string
	DiagLatency    string
	DiagWhat       string
	DiagWhy        string
	DiagTreeTitle  string
	DiagRunbook    string

	OtelTitle     string
	OtelInactive  string
	OtelListening string
	OtelReceiver  string
	OtelRing      string
	OtelBatch     string
	OtelRows      string
	OtelSize      string
	OtelLimits    string
}

var catalogES = Catalog{
	Starting:         "iniciando okshield…",
	ConnectingDocker: "conectando con docker…",
	DockerConnected:  "docker conectado · %d contenedores",
	DockerDown:       "docker no disponible",
	Loading:          "cargando…",
	Empty:            "(vacío)",
	NoSelection:      "sin selección",
	NoResource:       "Sin recurso seleccionado",

	Views:   [3]string{"Contenedores", "Redes", "Volúmenes"},
	Tabs:    [5]string{"Resumen", "Métricas", "Detalles", "Logs", "Diagnóstico"},
	Metrics: [4]string{"CPU %", "Memoria %", "Red RX/TX", "Disco I/O"},

	CtrlScroll:   "Scroll",
	CtrlViews:    "Vistas",
	CtrlTabs:     "Pestañas",
	CtrlFocus:    "Foco",
	CtrlWhatsup:  "Whatsup",
	CtrlDiagnose: "Diagnóstico",
	CtrlLang:     "Idioma",
	CtrlCommand:  "Comando",
	CtrlQuit:     "Salir",

	NoAnomalies:     "Sin anomalías activas (? para /whatsup)",
	HelpPlaceholder: "escribe /help para ver comandos",
	HelpTitle:       "✦ Comandos de OkShield",
	HelpPromptLabel: "Comando: ",
	HelpAvailTitle:  "Comandos Disponibles",
	HelpDescTitle:   "Descripción y Detalles",
	HelpFooterCmd:   "enter ejecutar  ·  tab autocompletar  ·  esc cerrar",
	HelpFooterAny:   "pulsa cualquier tecla para cerrar",

	PulseHint:         "Pulsa ? o escribe /whatsup para sintetizar el estado de tus contenedores (sin IA).",
	PulseTitle:        "Pulso Determinista (/whatsup)",
	PulseNoContainers: "No se encontraron contenedores en el host.",

	MetricsNoContainers: "Sin contenedores activos para graficar métricas en tiempo real.",
	MetricsTitlePrefix:  "Métricas en Tiempo Real · ",
	MetricsSwitchHint:   "(↑/↓ con foco o 'm' cambia gráfica)",
	MetricsNow:          "ahora",
	MetricsRead:         "lect",
	MetricsWrite:        "escr",

	GeneralSummaryTitle: "Resumen General de Infraestructura",
	GeneralContainers:   "contenedores",
	GeneralNetworks:     "redes",
	GeneralVolumes:      "volúmenes",
	SectionPorts:        "Puertos",
	SectionNetworks:     "Redes",
	SectionMounts:       "Mounts",
	Unpublished:         "no publicado",
	ConnectedContainers: "Contenedores conectados",
	None:                "  ninguno",
	UsedBy:              "Usado por",
	OrphanVolume:        "  ningún contenedor (huérfano)",
	NoLimit:             "sin límite",
	Yes:                 "sí",

	FieldImage:    "imagen",
	FieldState:    "estado",
	FieldRestarts: "reinicios",
	FieldPolicy:   "política",
	FieldRAMLimit: "límite RAM",
	FieldCPULimit: "límite CPU",
	FieldInternal: "interna",
	FieldCreated:  "creado",

	LogsSelectHint: "Selecciona un contenedor para inspeccionar sus logs deduplicados.",
	LogsTitle:      "Plantillas Drain-like",
	LogsRingCount:  "(%d líneas en ring buffer → %d patrones únicos)",
	LogsEmpty:      "Sin logs recientes para este contenedor.",
	LogsRecent:     "Últimas líneas (Ring Buffer)",

	DiagHint:       "Pulsa d o ejecuta /diagnose [container] para clasificar la causa raíz y generar el runbook.",
	DiagTitle:      "Diagnóstico: ",
	DiagConfidence: "confianza %.0f%%",
	DiagDeciderHdr: "Strands Decider 2B (#policy)",
	DiagCalibConf:  "Confianza Calibrada: %.1f%%",
	DiagLatency:    "Latencia: %dms",
	DiagWhat:       "Qué pasó:",
	DiagWhy:        "Por qué pasó:",
	DiagTreeTitle:  "Causa Raíz (Árbol de Decisión)",
	DiagRunbook:    "Guía de Resolución (Runbook)",

	OtelTitle:     "Estado de Telemetría OTLP & DuckDB",
	OtelInactive:  "inactivo",
	OtelListening: "escuchando",
	OtelReceiver:  "receptor",
	OtelRing:      "%d / %d líneas (total ingerido: %d)",
	OtelBatch:     "%d lotes volcados (cada 2s o 500 regs)",
	OtelRows:      "%d registros",
	OtelSize:      "tamaño DB",
	OtelLimits:    "límites DB",
}

var catalogEN = Catalog{
	Starting:         "starting okshield…",
	ConnectingDocker: "connecting to docker…",
	DockerConnected:  "docker connected · %d containers",
	DockerDown:       "docker unavailable",
	Loading:          "loading…",
	Empty:            "(empty)",
	NoSelection:      "no selection",
	NoResource:       "No resource selected",

	Views:   [3]string{"Containers", "Networks", "Volumes"},
	Tabs:    [5]string{"Summary", "Metrics", "Details", "Logs", "Diagnose"},
	Metrics: [4]string{"CPU %", "Memory %", "Net RX/TX", "Disk I/O"},

	CtrlScroll:   "Scroll",
	CtrlViews:    "Views",
	CtrlTabs:     "Tabs",
	CtrlFocus:    "Focus",
	CtrlWhatsup:  "Whatsup",
	CtrlDiagnose: "Diagnose",
	CtrlLang:     "Lang",
	CtrlCommand:  "Command",
	CtrlQuit:     "Quit",

	NoAnomalies:     "No active anomalies (? for /whatsup)",
	HelpPlaceholder: "type /help to see commands",
	HelpTitle:       "✦ OkShield Commands",
	HelpPromptLabel: "Command: ",
	HelpAvailTitle:  "Available Commands",
	HelpDescTitle:   "Description & Details",
	HelpFooterCmd:   "enter run  ·  tab autocomplete  ·  esc close",
	HelpFooterAny:   "press any key to close",

	PulseHint:         "Press ? or type /whatsup to synthesize container health deterministically (no AI).",
	PulseTitle:        "Deterministic Pulse (/whatsup)",
	PulseNoContainers: "No containers found on host.",

	MetricsNoContainers: "No active containers to plot real-time metrics.",
	MetricsTitlePrefix:  "Real-Time Metrics · ",
	MetricsSwitchHint:   "(↑/↓ when focused or 'm' switches chart)",
	MetricsNow:          "now",
	MetricsRead:         "read",
	MetricsWrite:        "write",

	GeneralSummaryTitle: "General Infrastructure Overview",
	GeneralContainers:   "containers",
	GeneralNetworks:     "networks",
	GeneralVolumes:      "volumes",
	SectionPorts:        "Ports",
	SectionNetworks:     "Networks",
	SectionMounts:       "Mounts",
	Unpublished:         "unpublished",
	ConnectedContainers: "Connected containers",
	None:                "  none",
	UsedBy:              "Used by",
	OrphanVolume:        "  no container (orphan)",
	NoLimit:             "unlimited",
	Yes:                 "yes",

	FieldImage:    "image",
	FieldState:    "state",
	FieldRestarts: "restarts",
	FieldPolicy:   "policy",
	FieldRAMLimit: "RAM limit",
	FieldCPULimit: "CPU limit",
	FieldInternal: "internal",
	FieldCreated:  "created",

	LogsSelectHint: "Select a container to inspect its deduplicated logs.",
	LogsTitle:      "Drain-like Templates",
	LogsRingCount:  "(%d lines in ring buffer → %d unique patterns)",
	LogsEmpty:      "No recent logs for this container.",
	LogsRecent:     "Recent Lines (Ring Buffer)",

	DiagHint:       "Press d or run /diagnose [container] to classify root cause and generate the runbook.",
	DiagTitle:      "Diagnosis: ",
	DiagConfidence: "confidence %.0f%%",
	DiagDeciderHdr: "Strands Decider 2B (#policy)",
	DiagCalibConf:  "Calibrated Confidence: %.1f%%",
	DiagLatency:    "Latency: %dms",
	DiagWhat:       "What happened:",
	DiagWhy:        "Why it happened:",
	DiagTreeTitle:  "Root Cause (Decision Tree)",
	DiagRunbook:    "Resolution Guide (Runbook)",

	OtelTitle:     "OTLP & DuckDB Telemetry Status",
	OtelInactive:  "inactive",
	OtelListening: "listening",
	OtelReceiver:  "receiver",
	OtelRing:      "%d / %d lines (total ingested: %d)",
	OtelBatch:     "%d batches flushed (every 2s or 500 recs)",
	OtelRows:      "%d records",
	OtelSize:      "DB size",
	OtelLimits:    "DB limits",
}

func (m Model) tr() Catalog {
	if m.lang == LangEN {
		return catalogEN
	}
	return catalogES
}
