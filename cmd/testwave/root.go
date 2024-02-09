package testwave

import (
	"github.com/celestiaorg/testwave/pkg/dispatcher"
	"github.com/celestiaorg/testwave/pkg/playbook"
	"github.com/celestiaorg/testwave/pkg/worker"
	"github.com/spf13/cobra"
)

const (
	DefaultBinPath = "/app/testwave"
	StartCmd       = "start"
	DispatcherCmd  = dispatcher.Cmd
	WorkerCmd      = worker.Cmd
)

func rootCmd(testPlanPlaybook playbook.Playbook) *cobra.Command {
	rootCmd := &cobra.Command{}
	rootCmd.AddCommand(
		dispatcherCmd(testPlanPlaybook),
		workerCmd(testPlanPlaybook),
		startCmd(),
	)
	return rootCmd
}
