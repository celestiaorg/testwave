package testwave

import (
	"context"
	"fmt"
	"os"

	"github.com/celestiaorg/testwave/pkg/playbook"
	"github.com/spf13/cobra"
)

type TestWave struct {
	RootCmd *cobra.Command
}

func New(testPlanPlaybook playbook.Playbook) *TestWave {
	return &TestWave{
		RootCmd: rootCmd(testPlanPlaybook),
	}
}

func (t *TestWave) Execute() {
	if len(os.Args) > 0 {
		t.RootCmd.Use = os.Args[0]
	}

	ctx := context.Background()
	if err := t.RootCmd.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
