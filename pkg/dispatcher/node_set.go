package dispatcher

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/celestiaorg/knuu/pkg/names"
	"github.com/celestiaorg/testwave/pkg/constants"
	"github.com/celestiaorg/testwave/pkg/message"
	"github.com/celestiaorg/testwave/pkg/playbook"
	"github.com/celestiaorg/testwave/pkg/worker"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type dependentServicesResponse struct {
	redisPort              int32
	redisIP, minioEndpoint string
}

func (d *Dispatcher) DeployNodeSet(ctx context.Context, wg *sync.WaitGroup, ns *playbook.NodeSet) {
	defer wg.Done()

	pod, err := d.createPodForNodeSet(ctx, ns)
	if err != nil {
		logrus.Errorf("Error creating pod for NodeSet %s: %v", ns.UID, err)
		return
	}

	_, err = d.Clientset.CoreV1().Pods(d.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		logrus.Errorf("Error deploying pod for NodeSet %s : %v", ns.UID, err)
		return
	}

	logrus.Infof("NodeSet `%s` pod deployed", ns.UID)
}

func (d *Dispatcher) setupDependentServices(ctx context.Context) (*dependentServicesResponse, error) {
	var (
		res                  = &dependentServicesResponse{}
		wg                   sync.WaitGroup
		minioErr, minioEpErr error
		redisErr, redisIpErr error
	)

	// run these two independent tasks in two different go routines
	// for sake of optimization
	func() {
		wg.Add(1)
		defer wg.Done()
		if redisErr = d.deployRedis(ctx); redisErr != nil {
			return
		}
		res.redisIP, res.redisPort, redisIpErr = d.RedisIPPort(ctx)
	}()

	func() {
		wg.Add(1)
		defer wg.Done()
		if minioErr = d.deployMinio(ctx); minioErr != nil {
			return
		}
		res.minioEndpoint, minioEpErr = d.getMinioEndpoint(ctx)
	}()

	wg.Wait()

	for _, err := range []error{redisErr, minioErr, minioEpErr, redisIpErr} {
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (d *Dispatcher) createPodForNodeSet(ctx context.Context, nodeSet *playbook.NodeSet) (*v1.Pod, error) {
	if nodeSet == nil {
		return nil, ErrNilNodeSet
	}
	if len(nodeSet.Workers) == 0 {
		return nil, ErrNoWorkersInNodeSet.Wrap(fmt.Errorf("NodeSet %s", nodeSet.UID))
	}

	depServices, err := d.setupDependentServices(ctx)
	if err != nil {
		return nil, err
	}

	defaultEnvs := []v1.EnvVar{
		{Name: worker.EnvTestPlan, Value: d.Playbook.Name()},
		{Name: worker.EnvPodUID, Value: nodeSet.UID},
		{Name: worker.EnvMinioEndpoint, Value: depServices.minioEndpoint},
		{Name: worker.EnvMinioAccessKey, Value: minioAccessKey},
		{Name: worker.EnvMinioSecretKey, Value: minioSecretKey},
		{Name: message.EnvRedisAddr, Value: fmt.Sprintf("%s:%d", depServices.redisIP, depServices.redisPort)},
		{Name: message.EnvRedisDB, Value: RedisDB},
		{Name: message.EnvRedisPassword, Value: RedisPassword},
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeSet.UID,
			Namespace: d.Namespace,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	for i, w := range nodeSet.Workers {
		if w.UID == "" {
			w.UID = fmt.Sprintf("%s-w%d", nodeSet.UID, i)
		}

		wEnvs := defaultEnvs
		wEnvs = append(wEnvs, v1.EnvVar{Name: worker.EnvWorkerUID, Value: w.UID})
		for k, v := range w.Envs {
			wEnvs = append(wEnvs, v1.EnvVar{
				Name:  k,
				Value: v,
			})
		}

		pod, vols, err := d.AddFilesToPod(ctx, pod, w.Files)
		if err != nil {
			return nil, ErrAddingFileToPod.Wrap(err)
		}

		wc := v1.Container{
			Name:         w.UID,
			Image:        d.WorkerImage, // All workers in a nodeset share the same image
			Env:          wEnvs,
			Args:         []string{constants.DefaultBinPath, worker.Cmd},
			VolumeMounts: vols,
			// We need these privileges to allow BitTwister to shape the traffic
			SecurityContext: &v1.SecurityContext{
				Capabilities: &v1.Capabilities{Add: []v1.Capability{"NET_ADMIN"}},
				Privileged:   ptr.To[bool](true),
			},
		}
		pod.Spec.Containers = append(pod.Spec.Containers, wc)
	}
	return pod, nil
}

// AddFilesToPod adds files to a pod and returns the updated pod and the volume mounts.
// The files are added to the pod as a ConfigMap.
// The mount points are determined by the `values` of the filesMap.
// Please note that, the files must be small in size, as they are stored in the ConfigMap.
// For large files, we should use either Minio or copy them directly into the docker image.
func (m *Dispatcher) AddFilesToPod(ctx context.Context, pod *v1.Pod, filesMap map[string]string) (*v1.Pod, []v1.VolumeMount, error) {
	// first group files by their mount points to cover
	// multiple files in the same mount directory
	gFilesMap := groupMountPoints(filesMap)

	vols := []v1.VolumeMount{}
	for mountDir, fMaps := range gFilesMap {
		configMapMountPoint := mountDir

		configMapName, err := names.NewRandomK8(pod.Name + "-confmap")
		if err != nil {
			return nil, nil, err
		}

		configMap := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: m.Namespace,
			},
			BinaryData: map[string][]byte{},
		}
		for srcPath, mountPath := range fMaps {
			filename := filepath.Base(mountPath)
			content, err := fileContent(srcPath)
			if err != nil {
				return nil, nil, err
			}
			configMap.BinaryData[filename] = content
		}

		_, err = m.Clientset.CoreV1().ConfigMaps(m.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
		if err != nil {
			return nil, nil, err
		}

		pod.Spec.Volumes = append(pod.Spec.Volumes,
			v1.Volume{
				Name: configMapName, // we might change this in the future
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: configMapName,
						},
					},
				},
			})

		vols = append(vols, v1.VolumeMount{
			Name:      configMapName,
			MountPath: configMapMountPoint,
		})
	}

	return pod, vols, nil
}

func fileContent(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}

// groupMountPoints groups mount points by their parent directory.
// This is useful for creating a single volume for each directory.
// For example, if we have the following files:
//
//	{
//		"/home/user1/file1.txt": "/opt/file1.txt",
//		"/home/user1/file2.txt": "/opt/file2.txt",
//		"/home/user2/file3.txt": "/opt/file3.txt",
//		"/home/user2/file4.txt": "/etc/file4.txt",
//	}
//
// Then the result will be:
//
//	{
//		"/opt/": {
//			"/home/user1/file1.txt": "/opt/file1.txt",
//			"/home/user1/file2.txt": "/opt/file2.txt",
//			"/home/user2/file3.txt": "/opt/file3.txt",
//		},
//		"/etc/": {
//			"/home/user2/file4.txt": "/etc/file4.txt",
//		},
//	}
func groupMountPoints(filesMap map[string]string) map[string]map[string]string {
	resMap := make(map[string]map[string]string)

	for srcPath, mountPath := range filesMap {
		mountDir := filepath.Dir(mountPath)
		if resMap[mountDir] == nil {
			resMap[mountDir] = make(map[string]string)
		}
		resMap[mountDir][srcPath] = mountPath
	}

	return resMap
}
