package testwave

import (
	"os"

	"github.com/celestiaorg/testwave/pkg/deployer"
	"github.com/celestiaorg/testwave/pkg/dispatcher"
	"github.com/celestiaorg/testwave/pkg/playbook"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/util/homedir"
)

const (
	flagKubeConfig = "kubeconfig"
	flagLogLevel   = "log-level"
)

var flagsDispatcher struct {
	kubeConfig string
	logLevel   string
}

func dispatcherCmd(testPlanPlaybook playbook.Playbook) *cobra.Command {
	if testPlanPlaybook == nil {
		logrus.Errorf("Error: test plan playbook is nil")
		os.Exit(1)
	}
	cmd := &cobra.Command{
		Use:   DispatcherCmd,
		Short: "start the dispatcher node",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientset, err := deployer.KubeClientset(flagsDispatcher.kubeConfig)
			if err != nil {
				logrus.Errorf("Error building kubeconfig: %v\n", err)
				os.Exit(1)
			}

			workerImage := os.Getenv(dispatcher.EnvWorkerImage)
			if workerImage == "" {
				logrus.Errorf("Error reading worker image from environment variable %s", dispatcher.EnvWorkerImage)
				os.Exit(1)
			}

			namespace := os.Getenv(dispatcher.EnvNamespace)
			if namespace == "" {
				namespace = "default"
				logrus.Info("Using `default` k8s namespace")
			}

			d := &dispatcher.Dispatcher{
				Clientset:   clientset,
				Namespace:   namespace,
				Playbook:    testPlanPlaybook,
				WorkerImage: workerImage,
			}

			d.RunTest()
			return nil
		},
	}

	defaultKubeConfig := ""
	if home := homedir.HomeDir(); home != "" {
		defaultKubeConfig = home + "/.kube/config"
	}

	cmd.Flags().StringVar(&flagsDispatcher.kubeConfig, flagKubeConfig, defaultKubeConfig, "Path to kubeconfig file")
	// cmd.Flags().StringVar(&flagsDispatcher.logLevel, flagLogLevel, "info", "Log level (debug, info, warn, error, fatal, panic)")

	return cmd
}
