package qualification

import (
	"encoding/json"
	"fmt"
	"os"
)

type Pair struct {
	CoreBinary           string `json:"coreBinary"`
	CoreTag              string `json:"coreTag"`
	CoreSHA              string `json:"coreSha"`
	CoreVersion          string `json:"coreVersion"`
	CoreBinarySHA256     string `json:"coreBinarySha256"`
	ProviderRef          string `json:"providerRef"`
	ProviderSHA          string `json:"providerSha"`
	ProviderBinary       string `json:"providerBinary"`
	ProviderBinarySHA256 string `json:"providerBinarySha256"`
	HarnessSHA           string `json:"harnessSha"`
}

type Scenario struct {
	Name    string `json:"name"`
	Outcome string `json:"outcome"`
}

type Cell struct {
	Shell         string            `json:"shell"`
	User          string            `json:"user"`
	BuiltinSSH    bool              `json:"builtinSsh"`
	Workspace     string            `json:"workspace"`
	AgentDir      string            `json:"agentDir"`
	LoginShell    string            `json:"loginShell"`
	Scenarios     []Scenario        `json:"scenarios"`
	AgentSHA256   map[string]string `json:"agentSha256"`
	CleanupStatus string            `json:"cleanup"`
}

type Privilege struct {
	User           string `json:"user"`
	UserUID        uint64 `json:"userUid"`
	AgentDir       string `json:"agentDir"`
	DirectoryState string `json:"directoryState"`
	WriteDenied    bool   `json:"writeDenied"`
	WriteError     string `json:"writeError"`
	AgentUID       uint64 `json:"agentUid"`
	AgentVersion   string `json:"agentVersion"`
	AgentSHA256    string `json:"agentSha256"`
	LiveCommand    bool   `json:"liveCommand"`
	Outcome        string `json:"outcome"`
	CleanupStatus  string `json:"cleanup"`
}

type Report struct {
	SchemaVersion int        `json:"schemaVersion"`
	Pair          Pair       `json:"pair"`
	Cells         []*Cell    `json:"cells"`
	Privilege     *Privilege `json:"privilege"`
}

func Load(path string) (Report, error) {
	file, err := os.Open(path)
	if err != nil {
		return Report{}, fmt.Errorf("open report %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var report Report
	if err := decoder.Decode(&report); err != nil {
		return Report{}, fmt.Errorf("decode report %s: %w", path, err)
	}
	return report, nil
}
