package tui

// Catalog holds all UI display strings in English.
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
	Tabs    [4]string
	Metrics [4]string

	CtrlScroll  string
	CtrlViews   string
	CtrlTabs    string
	CtrlFocus   string
	CtrlWhatsup string
	CtrlCommand string
	CtrlQuit    string

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
}

var uiCatalog = Catalog{
	Starting:         "starting okshield…",
	ConnectingDocker: "connecting to docker…",
	DockerConnected:  "docker connected · %d containers",
	DockerDown:       "docker unavailable",
	Loading:          "loading…",
	Empty:            "(empty)",
	NoSelection:      "no selection",
	NoResource:       "No resource selected",

	Views:   [3]string{"Containers", "Networks", "Volumes"},
	Tabs:    [4]string{"Summary", "Metrics", "Details", "Logs"},
	Metrics: [4]string{"CPU %", "Memory %", "Net RX/TX", "Disk I/O"},

	CtrlScroll:  "Scroll",
	CtrlViews:   "Views",
	CtrlTabs:    "Tabs",
	CtrlFocus:   "Focus",
	CtrlWhatsup: "Whatsup",
	CtrlCommand: "Command",
	CtrlQuit:    "Quit",

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
}

func (m Model) tr() Catalog {
	return uiCatalog
}
