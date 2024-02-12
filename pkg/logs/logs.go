package logs

import (
	"context"
	"io"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Logs struct {
	Clientset kubernetes.Interface
	Namespace string
}

func (l *Logs) AllPods(ctx context.Context) ([]v1.Pod, error) {
	podList, err := l.Clientset.CoreV1().Pods(l.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}

func (l *Logs) LogsReader(ctx context.Context, podName, containerName string) (io.ReadCloser, error) {
	options := &v1.PodLogOptions{
		Container: containerName,
		Follow:    true,
	}

	req := l.Clientset.CoreV1().Pods(l.Namespace).GetLogs(podName, options)

	logStream, err := req.Stream(ctx)
	if err != nil {
		// Check if the container is not running
		if strings.Contains(err.Error(), "container not found") || strings.Contains(err.Error(), "not found") {
			// Container is not running; attempt to retrieve logs from the terminated pod
			options.Follow = false
			return l.Clientset.CoreV1().Pods(l.Namespace).GetLogs(podName, options).Stream(ctx)
		}
		return nil, err
	}

	return logStream, nil
}
