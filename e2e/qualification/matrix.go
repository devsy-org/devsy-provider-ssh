package qualification

const SchemaVersion = 1

const PrivilegeUser = "devsy-sudo"

const (
	ScenarioColdDownload        = "installs a cold agent by remote download"
	ScenarioCurrentReuse        = "reuses the current agent when the workspace is opened again"
	ScenarioStaleReplacement    = "replaces a deliberately old agent"
	ScenarioStdinTransfer       = "installs a cold agent by stdin transfer and keeps stdin for the live stream"
	ScenarioLoginShellUnchanged = "leaves the account login shell unchanged"
)

var LifecycleScenarios = []string{
	ScenarioColdDownload,
	ScenarioCurrentReuse,
	ScenarioStaleReplacement,
	ScenarioStdinTransfer,
	ScenarioLoginShellUnchanged,
}

type Transport struct {
	Shell   string
	Builtin bool
}

var Transports = []Transport{
	{Shell: "bash"},
	{Shell: "bash", Builtin: true},
	{Shell: "zsh"},
	{Shell: "zsh", Builtin: true},
	{Shell: "fish"},
	{Shell: "fish", Builtin: true},
}

func Shells() []string {
	var shells []string
	seen := make(map[string]bool)
	for _, transport := range Transports {
		if seen[transport.Shell] {
			continue
		}
		seen[transport.Shell] = true
		shells = append(shells, transport.Shell)
	}
	return shells
}
