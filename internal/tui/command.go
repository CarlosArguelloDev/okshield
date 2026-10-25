package tui

import "strings"

// CommandKind identifies a slash command understood by the command bar.
type CommandKind int

const (
	CmdUnknown CommandKind = iota
	CmdWhatsup
	CmdContainers
	CmdNetworks
	CmdVolumes
	CmdLang
	CmdHelp
	CmdQuit
)

// Command is the parsed form of a command bar input.
type Command struct {
	Kind CommandKind
	Name string   // canonical name, e.g. "whatsup"
	Args []string // positional arguments
	Raw  string
}

// commandSpec describes a command for parsing and help rendering.
type commandSpec struct {
	Kind      CommandKind
	Names     []string // first is canonical, the rest are aliases
	Usage     string
	Summary   string
	SummaryEN string
	Key       string
}

// commandTable is the single source of truth for available commands.
var commandTable = []commandSpec{
	{CmdWhatsup, []string{"whatsup", "pulse"}, "/whatsup", "Resumen determinista de lo que ocurre", "Deterministic summary of container health", "?"},
	{CmdContainers, []string{"containers", "c"}, "/containers", "Vista de contenedores", "Containers view", "1"},
	{CmdNetworks, []string{"networks", "n"}, "/networks", "Vista de redes", "Networks view", "2"},
	{CmdVolumes, []string{"volumes", "v"}, "/volumes", "Vista de volúmenes", "Volumes view", "3"},
	{CmdLang, []string{"lang", "language", "idioma"}, "/lang [es|en]", "Cambia el idioma entre Español e Inglés", "Switch language between Spanish and English", "i"},
	{CmdHelp, []string{"help", "h"}, "/help", "Muestra esta ayuda", "Show this help overlay", ""},
	{CmdQuit, []string{"quit", "q", "exit"}, "/quit", "Salir de OkShield", "Exit OkShield", "ctrl+c"},
}

// ParseCommand parses a command bar input such as "/diagnose api". The
// leading slash is optional. Unknown commands yield Kind == CmdUnknown.
func ParseCommand(input string) Command {
	raw := strings.TrimSpace(input)
	fields := strings.Fields(strings.TrimPrefix(raw, "/"))
	if len(fields) == 0 {
		return Command{Kind: CmdUnknown, Raw: raw}
	}
	name := strings.ToLower(fields[0])
	args := fields[1:]

	for _, spec := range commandTable {
		for _, n := range spec.Names {
			if n != name {
				continue
			}
			return Command{Kind: spec.Kind, Name: spec.Names[0], Args: args, Raw: raw}
		}
	}
	return Command{Kind: CmdUnknown, Name: name, Args: args, Raw: raw}
}

// Suggest returns canonical usages whose name starts with the given prefix,
// used for command bar autocompletion hints.
func Suggest(prefix string) []string {
	p := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(prefix), "/"))
	out := make([]string, 0, len(commandTable))
	for _, spec := range commandTable {
		if strings.HasPrefix(spec.Names[0], p) {
			out = append(out, spec.Usage)
		}
	}
	return out
}
