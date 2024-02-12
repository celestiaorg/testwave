package testwave

import (
	"context"
	"os"
	"time"

	"github.com/celestiaorg/testwave/pkg/deployer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	flagNamespace  = "namespace"
	flagContextDir = "context-dir"
	flagTimeout    = "timeout"
)

var flagsStart struct {
	kubeConfig string
	namespace  string
	contextDir string
	timeout    int
	logLevel   string
}

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   StartCmd,
		Short: "build images, setup the test and start it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			logrus.SetOutput(os.Stdout)
			level, err := logrus.ParseLevel(flagsStart.logLevel)
			if err != nil {
				logrus.Errorf("Error parsing log level: %v", err)
				os.Exit(1)
			}
			logrus.SetLevel(level)

			clientset, err := deployer.KubeClientset(flagsStart.kubeConfig)
			if err != nil {
				logrus.Errorf("Error building kubeconfig: %v", err)
				os.Exit(1)
			}

			dp := deployer.Deployer{
				Clientset: clientset,
				Namespace: flagsStart.namespace,
			}

			logrus.Info("Building the image...")

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(flagsStart.timeout)*time.Minute)
			defer cancel()

			logs, err := dp.Build(ctx, flagsStart.contextDir)
			printUnQuotedLogs(logs)
			if err != nil {
				logrus.Errorf("Error building dispatcher images: %v", err)
				os.Exit(1)
			}

			logrus.Infof("Image built: %s", dp.ImageName())

			logs, err = dp.Deploy(ctx)
			printUnQuotedLogs(logs)
			if err != nil {
				logrus.Errorf("Error deploying dispatcher image: %v", err)
				os.Exit(1)
			}

			logrus.Infof("Dispatcher node deployed with UID: %s . Test in progress...", dp.PodName())
			return nil
		},
	}

	homedir, err := os.UserHomeDir()
	if err != nil {
		logrus.Errorf("Error getting user home dir: %v", err)
		os.Exit(1)
	}

	defaultKubeConfig := homedir + "/.kube/config"
	cmd.Flags().StringVar(&flagsStart.kubeConfig, flagKubeConfig, defaultKubeConfig, "Path to kubeconfig file")
	cmd.Flags().StringVar(&flagsStart.namespace, flagNamespace, "default", "Kubernetes namespace")

	defaultContextDir, err := os.Getwd()
	if err != nil {
		logrus.Errorf("Error getting current dir: %v", err)
		os.Exit(1)
	}
	cmd.Flags().StringVar(&flagsStart.contextDir, flagContextDir, defaultContextDir, "Context directory")
	cmd.Flags().IntVar(&flagsStart.timeout, flagTimeout, 10, "Timeout for the test deployment (in minutes)")
	cmd.Flags().StringVar(&flagsStart.logLevel, flagLogLevel, "info", "Log level")

	return cmd
}
