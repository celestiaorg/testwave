package testwave

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/celestiaorg/testwave/pkg/logs"

	"github.com/celestiaorg/testwave/pkg/deployer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// ANSI escape codes for colors
const (
	colorGreen = "\x1b[32m"
	colorBlue  = "\x1b[94m"
	colorRed   = "\x1b[91m"
	colorReset = "\x1b[0m"
)

var flagsLogs struct {
	kubeConfig string
	namespace  string
	timeout    int
}

func logsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   LogsCmd,
		Short: "shows the logs interactively",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientset, err := deployer.KubeClientset(flagsStart.kubeConfig)
			if err != nil {
				logrus.Errorf("Error building kubeconfig: %v", err)
				os.Exit(1)
			}

			cLog := logs.Logs{
				Clientset: clientset,
				Namespace: flagsLogs.namespace,
			}
			scanner := bufio.NewScanner(os.Stdin)

			for {
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(flagsLogs.timeout)*time.Minute)
				defer cancel()

				pods, err := cLog.AllPods(ctx)
				if err != nil {
					logrus.Errorf("Error getting pods: %v", err)
					os.Exit(1)
				}

				containerSearchMap := make(map[int64]string)
				podSearchMap := make(map[string]string) // key: container name, value: pod name
				counter := int64(1)
				for _, p := range pods {
					pName := p.Name + " - " + timeAgo(p.CreationTimestamp.Time)
					fmt.Printf("pod: %s%s%s\n", colorGreen, pName, colorReset)
					for _, c := range p.Status.ContainerStatuses {
						containerSearchMap[counter] = c.Name
						podSearchMap[c.Name] = p.Name

						var status string
						if c.State.Running != nil {
							status = "Running"
						} else if c.State.Terminated != nil {
							status = fmt.Sprintf("Terminated (Exit Code: %d)", c.State.Terminated.ExitCode)
						} else if c.State.Waiting != nil {
							status = fmt.Sprintf("Waiting (%s)", c.State.Waiting.Reason)
						} else {
							status = "Unknown"
						}

						fmt.Printf("%s[%3d] - %s - %s%s\n", colorBlue, counter, c.Name, colorReset, status)
						counter++
					}
				}
				fmt.Printf("\n%s[%3d] - Exit%s\n", colorRed, 0, colorReset)

				fmt.Print("Enter the container number to see the logs (press Enter to refresh): ")

				scanner.Scan()
				input := scanner.Text()

				// empty string Enter key press
				if strings.TrimSpace(input) == "" {
					fmt.Println("Refreshing menu...")
					continue
				}

				containerNumber, err := strconv.ParseInt(input, 10, 64)
				if err != nil {
					log.Printf("Invalid input: %v", err)
					continue
				}

				if containerNumber == 0 {
					return nil
				}

				containerName, ok := containerSearchMap[containerNumber]
				if !ok {
					logrus.Errorf("Invalid container number: %d", containerNumber)
					continue
				}

				podName, ok := podSearchMap[containerName]
				if !ok {
					logrus.Errorf("Invalid pod name for container: %s", containerName)
					continue
				}

				logrus.Infof("Logs for pod: %s container: %s", podName, containerName)

				logsReader, err := cLog.LogsReader(ctx, podName, containerName)
				if err != nil {
					logrus.Errorf("Error getting logs reader: %v", err)
					continue
				}

				stop := make(chan os.Signal, 1)
				signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

				go func() {
					_, err := io.Copy(os.Stdout, logsReader)
					if err != nil {
						logrus.Errorf("Error copying logs: %v", err)
					}
				}()

				<-stop // Wait for the stop signal
				logsReader.Close()
			}
		},
	}

	homedir, err := os.UserHomeDir()
	if err != nil {
		logrus.Errorf("Error getting user home dir: %v", err)
		os.Exit(1)
	}

	defaultKubeConfig := homedir + "/.kube/config"
	cmd.Flags().StringVar(&flagsLogs.kubeConfig, flagKubeConfig, defaultKubeConfig, "Path to kubeconfig file")
	cmd.Flags().StringVar(&flagsLogs.namespace, flagNamespace, "default", "Kubernetes namespace")
	cmd.Flags().IntVar(&flagsLogs.timeout, flagTimeout, 5, "Timeout for fetching data (in minutes)")

	return cmd
}
