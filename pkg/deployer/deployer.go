package deployer

import (
	"context"

	"github.com/celestiaorg/knuu/pkg/builder"
	"github.com/celestiaorg/knuu/pkg/builder/kaniko"
	"github.com/celestiaorg/knuu/pkg/minio"
	"github.com/celestiaorg/testwave/pkg/dispatcher"
	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Deployer struct {
	Clientset             kubernetes.Interface
	Namespace             string
	imageNameWithRegistry string
	podName               string
}

func (d *Deployer) ImageName() string {
	return d.imageNameWithRegistry
}

func (d *Deployer) PodName() string {
	return d.podName
}

func (d *Deployer) Build(ctx context.Context, contextDir string) (logs string, err error) {
	kb := &kaniko.Kaniko{
		K8sClientset: d.Clientset,
		K8sNamespace: d.Namespace,
		Minio: &minio.Minio{
			Clientset: d.Clientset,
			Namespace: d.Namespace,
		},
	}

	newUUID, err := uuid.NewRandom()
	if err != nil {
		return "", ErrUUIDGeneration.Wrap(err)
	}
	imageName := newUUID.String()
	d.imageNameWithRegistry = GetDefaultRegistryImageName(imageName)

	return kb.Build(ctx, &builder.BuilderOptions{
		ImageName:    imageName,
		BuildContext: builder.DirContext{Path: contextDir}.BuildContext(),
		Destination:  d.ImageName(),
		Cache:        &builder.CacheOptions{},
	})
}

func (d *Deployer) Deploy(ctx context.Context) (logs string, err error) {
	dsp := &dispatcher.Dispatcher{
		Clientset: d.Clientset,
		Namespace: d.Namespace,
	}
	pod, err := dsp.DispatcherPod(d.ImageName())
	if err != nil {
		return "", ErrGettingDispatcherPodConfig.Wrap(err)
	}
	d.podName = pod.Name

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: dsp.Namespace,
			Labels:    map[string]string{"app": pod.Name},
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"*"},
				APIGroups: []string{"*"},
				Resources: []string{"*"},
			},
		},
	}

	_, err = d.Clientset.RbacV1().Roles(d.Namespace).Create(ctx, role, metav1.CreateOptions{})
	if err != nil {
		return "", ErrCreateDispatcherRole.Wrap(err)
	}

	_, err = d.Clientset.CoreV1().Pods(d.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", ErrCreateDispatcherPod.Wrap(err)
	}

	return d.containerLogs(ctx, pod.Spec.Containers[0].Name)
}

func (d *Deployer) containerLogs(ctx context.Context, name string) (string, error) {
	logOptions := v1.PodLogOptions{
		Container: name,
	}

	req := d.Clientset.CoreV1().Pods(d.Namespace).GetLogs(d.PodName(), &logOptions)
	logs, err := req.DoRaw(ctx)
	if err != nil {
		return "", ErrGettingContainerLogs.Wrap(err)
	}

	return string(logs), nil
}
