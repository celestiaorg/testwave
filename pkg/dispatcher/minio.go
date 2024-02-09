package dispatcher

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	MinioServiceName         = "minio-service"
	MinioServicePort         = 9000
	MinioServiceExternalPort = 30505 // some random port
	MinioDeploymentName      = "minio"
	MinioImage               = "minio/minio:RELEASE.2024-01-28T22-35-53Z"
	MinioStorageClassName    = "standard" // standard | gp2 | default
	MinioVolumeClaimName     = "minio-data"
	MinioVolumeMountPath     = "/data"
	MinioPVCStorageSize      = "1Gi"

	// The minio service is used internally, so not sure if it is ok to use constant key/secret
	minioAccessKey = "minioaccesskey"
	minioSecretKey = "miniosecretkey"

	minioWaitRetry            = 5 * time.Second
	minioPvPrefix             = "minio-pv-"
	minioPvHostPath           = "/tmp/minio-pv"
	minioDeploymentAppLabel   = "app"
	minioDeploymentMinioLabel = "minio"

	EnvMinioAccessKey = "MINIO_ACCESS_KEY"
	EnvMinioSecretKey = "MINIO_SECRET_KEY"
)

func (d *Dispatcher) deployMinio(ctx context.Context) error {
	deploymentClient := d.Clientset.AppsV1().Deployments(d.Namespace)

	deployed, err := d.isMinioDeployed(ctx)
	if err != nil {
		return err
	}
	if deployed {
		return nil
	}

	if err := d.createPVC(ctx, MinioVolumeClaimName, MinioPVCStorageSize, metav1.CreateOptions{}); err != nil {
		return err
	}

	// Create Minio deployment
	minioDeployment := &appsV1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MinioDeploymentName,
			Namespace: d.Namespace,
		},
		Spec: appsV1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{minioDeploymentAppLabel: minioDeploymentMinioLabel},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{minioDeploymentAppLabel: minioDeploymentMinioLabel}},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:  MinioDeploymentName,
						Image: MinioImage,
						Env: []v1.EnvVar{
							{Name: EnvMinioAccessKey, Value: minioAccessKey},
							{Name: EnvMinioSecretKey, Value: minioSecretKey},
						},
						Ports: []v1.ContainerPort{{ContainerPort: MinioServicePort}},
						VolumeMounts: []v1.VolumeMount{{
							Name:      MinioVolumeClaimName,
							MountPath: MinioVolumeMountPath,
						}},
						Command: []string{"minio", "server", MinioVolumeMountPath},
					}},
					Volumes: []v1.Volume{{
						Name: MinioVolumeClaimName,
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: MinioVolumeClaimName,
							},
						},
					}},
				},
			},
		},
	}

	_, err = deploymentClient.Create(ctx, minioDeployment, metav1.CreateOptions{})
	if err != nil {
		return ErrCreatingDeployment.Wrap(err)
	}

	if err = d.waitForMinioDeployment(ctx); err != nil {
		return ErrWaitingForMinio.Wrap(err)
	}

	if err := d.createMinioService(ctx); err != nil {
		return ErrCreatingService.Wrap(err)
	}

	if err := d.waitForMinioService(ctx); err != nil {
		return ErrWaitingForMinioService.Wrap(err)
	}

	logrus.Info("Minio deployed successfully.")
	return nil
}

func (d *Dispatcher) isMinioDeployed(ctx context.Context) (bool, error) {
	deploymentClient := d.Clientset.AppsV1().Deployments(d.Namespace)
	_, err := deploymentClient.Get(ctx, MinioDeploymentName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, ErrGettingDeployment.Wrap(err)
	}
	return true, nil
}

func (d *Dispatcher) createMinioService(ctx context.Context) error {
	serviceClient := d.Clientset.CoreV1().Services(d.Namespace)

	// Check if Minio service already exists
	_, err := serviceClient.Get(ctx, MinioServiceName, metav1.GetOptions{})
	if err == nil {
		logrus.Debugf("Service `%s` already exists.", MinioServiceName)
		return nil
	}

	// Create Minio service
	minioService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MinioServiceName,
			Namespace: d.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{minioDeploymentAppLabel: minioDeploymentMinioLabel},
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       MinioServicePort,
					TargetPort: intstr.FromInt(MinioServicePort),
					NodePort:   MinioServiceExternalPort,
				},
			},
			// Expose the service port outside the cluster, so client can push their data to Minio
			Type: v1.ServiceTypeNodePort,
		},
	}

	_, err = serviceClient.Create(ctx, minioService, metav1.CreateOptions{})
	if err != nil {
		return ErrCreatingService.Wrap(err)
	}

	logrus.Debugf("Service %s created successfully.", MinioServiceName)
	return nil
}

func (d *Dispatcher) getMinioEndpoint(ctx context.Context) (string, error) {
	service, err := d.Clientset.CoreV1().Services(d.Namespace).Get(ctx, MinioServiceName, metav1.GetOptions{})
	if err != nil {
		return "", ErrGettingService.Wrap(err)
	}

	if service.Spec.Type == v1.ServiceTypeLoadBalancer {
		// Use the LoadBalancer's external IP
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			return fmt.Sprintf("%s:%d", service.Status.LoadBalancer.Ingress[0].IP, service.Spec.Ports[0].Port), nil
		}
		return "", ErrLoadBalancerIPNotAvailable
	}

	if service.Spec.Type == v1.ServiceTypeNodePort {
		// Use the Node IP and NodePort
		nodes, err := d.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", ErrGettingNodes.Wrap(err)
		}
		if len(nodes.Items) == 0 {
			return "", ErrNoNodesFound
		}

		// Use the first node for simplicity, you might need to handle multiple nodes
		nodeIP := nodes.Items[0].Status.Addresses[0].Address
		return fmt.Sprintf("%s:%d", nodeIP, service.Spec.Ports[0].NodePort), nil
	}

	return fmt.Sprintf("%s:%d", service.Spec.ClusterIP, service.Spec.Ports[0].Port), nil
}

func (d *Dispatcher) waitForMinioDeployment(ctx context.Context) error {
	for {
		deployment, err := d.Clientset.AppsV1().Deployments(d.Namespace).Get(ctx, MinioDeploymentName, metav1.GetOptions{})
		if err == nil && deployment.Status.ReadyReplicas > 0 {
			break
		}

		select {
		case <-ctx.Done():
			return ErrTimeout
		case <-time.After(minioWaitRetry):
			// Retry after some seconds
		}
	}

	return nil
}

func (d *Dispatcher) waitForMinioService(ctx context.Context) error {
	for {
		service, err := d.Clientset.CoreV1().Services(d.Namespace).Get(ctx, MinioServiceName, metav1.GetOptions{})
		if err == nil &&
			(service.Spec.Type == v1.ServiceTypeLoadBalancer ||
				service.Spec.Type == v1.ServiceTypeNodePort) &&
			// Check if LoadBalancer IP, NodePort, or externalIPs are available
			(len(service.Status.LoadBalancer.Ingress) > 0 ||
				service.Spec.Ports[0].NodePort > 0 ||
				len(service.Spec.ExternalIPs) > 0) {

			// Check if Minio is reachable
			endpoint, err := d.getMinioEndpoint(ctx)
			if err != nil {
				return ErrGettingMinioEndpoint.Wrap(err)
			}

			if err := checkServiceConnectivity(endpoint); err == nil {
				break
			}
		}

		select {
		case <-ctx.Done():
			return ErrTimeout
		case <-time.After(minioWaitRetry):
			// Retry after some seconds
		}
	}

	return nil
}

func (d *Dispatcher) createPVC(ctx context.Context, pvcName string, storageSize string, createOptions metav1.CreateOptions) error {
	storageQt, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return ErrParsingStorageSize.Wrap(err)
	}

	pvcClient := d.Clientset.CoreV1().PersistentVolumeClaims(d.Namespace)

	// Check if PVC already exists
	_, err = pvcClient.Get(ctx, pvcName, metav1.GetOptions{})
	if err == nil {
		logrus.Debugf("PersistentVolumeClaim `%s` already exists.", pvcName)
		return nil
	}

	// Create a simple PersistentVolume if no suitable one is found
	pvList, err := d.Clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ErrListingPersistentVolumes.Wrap(err)
	}

	var existingPV *v1.PersistentVolume
	for _, pv := range pvList.Items {
		// Not sure if this condition is ok
		if pv.Spec.Capacity[v1.ResourceStorage].Equal(storageQt) {
			existingPV = &pv
			break
		}
	}

	if existingPV == nil {
		// Create a simple PV if no existing PV is suitable
		_, err = d.Clientset.CoreV1().PersistentVolumes().Create(ctx, &v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: minioPvPrefix,
			},
			Spec: v1.PersistentVolumeSpec{
				Capacity: v1.ResourceList{
					v1.ResourceStorage: storageQt,
				},
				AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				PersistentVolumeSource: v1.PersistentVolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: minioPvHostPath,
					},
				},
			},
		}, createOptions)
		if err != nil {
			return ErrCreatingPersistentVolume.Wrap(err)
		}

		logrus.Debugf("PersistentVolume `%s` created successfully.", existingPV.Name)
	}

	// Create PVC with the existing or newly created PV
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: d.Namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: storageQt,
				},
			},
		},
	}

	_, err = pvcClient.Create(ctx, pvc, createOptions)
	if err != nil {
		return ErrCreatingPersistentVolumeClaim.Wrap(err)
	}

	logrus.Debugf("PersistentVolumeClaim `%s` created successfully.", pvcName)
	return nil
}
