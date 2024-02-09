package dispatcher

import (
	"context"
	"os"
	"sync"

	"github.com/celestiaorg/knuu/pkg/names"
	"github.com/celestiaorg/testwave/pkg/constants"
	"github.com/celestiaorg/testwave/pkg/playbook"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	Cmd       = "dispatcher"
	PodPrefix = "dispatcher"

	EnvWorkerImage = "WORKER_IMAGE"
	EnvUID         = "DISPATCHER_UID"
	EnvNamespace   = "NAMESPACE"
)

// Dispatcher orchestrates the test and communicates with worker nodes.
type Dispatcher struct {
	Clientset   kubernetes.Interface
	Namespace   string
	Playbook    playbook.Playbook
	WorkerImage string
}

// RunTest executes the large-scale blockchain test.
func (d *Dispatcher) RunTest() {
	if err := d.Playbook.Setup(); err != nil {
		logrus.Errorf("Error setting up master node: %v\n", err)
		return
	}

	ctx := context.TODO()
	var wg sync.WaitGroup
	for _, nodeSet := range d.Playbook.NodeSets() {
		wg.Add(1)
		go d.DeployNodeSet(ctx, &wg, nodeSet)
	}
	// Wait for all worker nodes to be deployed
	wg.Wait()

	logrus.Info("All worker nodes deployed. Test in progress...")
}

// func (d *Dispatcher) WorkerLogs(ctx context.Context, nodeSet *playbook.NodeSet) (string, error) {
// 	logrus.Infof("Fetching logs from worker `%s` node...", nodeSet.UID)

// 	if len(pod.Spec.Containers) == 0 {
// 		return "", fmt.Errorf("no containers in pod %s", pod.Name)
// 	}

// 	logOptions := v1.PodLogOptions{
// 		Container: pod.Spec.Containers[0].Name,
// 	}

// 	req := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &logOptions)
// 	logs, err := req.DoRaw(ctx)
// 	if err != nil {
// 		return "", err
// 	}

// 	return string(logs), nil
// }

func (d *Dispatcher) DispatcherPod(imageName string) (*v1.Pod, error) {
	dispatcherUID, err := names.NewRandomK8(PodPrefix)
	if err != nil {
		logrus.Errorf("Error generating dispatcher uid: %v\n", err)
		os.Exit(1)
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dispatcherUID,
			Namespace: d.Namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  dispatcherUID + "-container",
					Image: imageName,
					Env: []v1.EnvVar{
						{
							Name:  EnvNamespace,
							Value: d.Namespace,
						},
						{
							Name:  EnvUID,
							Value: dispatcherUID,
						},
						{
							Name:  EnvWorkerImage,
							Value: imageName,
						},
					},
					Args: []string{constants.DefaultBinPath, Cmd},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	return pod, nil
}
