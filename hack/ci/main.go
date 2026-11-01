package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

var commands = map[string]func() error{
	"cleanup-qualification-accounts":   cleanupQualificationAccounts,
	"missing-candidate-guard":          requireExplicitCandidate,
	"provision-accounts":               provisionAccounts,
	"provision-qualification-accounts": provisionQualificationAccounts,
	"qualify-core":                     qualifyCore,
	"verify-provider":                  verifyProvider,
	"verify-report":                    verifyReport,
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	if len(args) != 1 {
		_, _ = fmt.Fprintf(stderr, "usage: go run ./hack/ci <%s>\n", commandNames())
		return 2
	}
	handler, ok := commands[args[0]]
	if !ok {
		_, _ = fmt.Fprintf(
			stderr,
			"unknown command %q; expected one of %s\n",
			args[0],
			commandNames(),
		)
		return 2
	}
	if err := handler(); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func commandNames() string {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, "|")
}
