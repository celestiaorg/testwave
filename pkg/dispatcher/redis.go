package dispatcher

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	redisDeploymentName = "redis-deployment"
	redisServiceName    = "redis-service"
	redisReplicas       = 1
	redisMaxReplicas    = 5
	redisCPUutilization = 80 // percentage
	redisAutoScalerKind = "Deployment"
	redisAutoScalerName = "redis-deployment"
	redisAutoScalerAPI  = "apps/v1"
	redisWaitRetry      = 5 * time.Second

	redisAppLabel      = "app"
	redisAppLabelValue = "redis"
	redisContainerName = "redis"

	envRedisPassword = "REDIS_PASSWORD"
	envRedisDB       = "REDIS_DB"

	RedisImage    = "redis:latest"
	RedisPort     = 6379
	RedisPassword = "redisPassword"
	RedisDB       = "0" // the db to use
	RedisTotalDBs = "2" // the total number of databases
)

func (d *Dispatcher) deployRedis(ctx context.Context) error {
	deployed, err := d.isRedisDeployed(ctx)
	if err != nil {
		return err
	}
	if deployed {
		return nil
	}

	redisDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: redisDeploymentName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](redisReplicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					redisAppLabel: redisAppLabelValue,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						redisAppLabel: redisAppLabelValue,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  redisContainerName,
							Image: RedisImage,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: RedisPort,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  envRedisPassword,
									Value: RedisPassword,
								},
								{
									Name:  envRedisDB,
									Value: RedisTotalDBs,
								},
							},
							// Apparently the password and db are not set by the env vars
							Args: []string{"--requirepass", RedisPassword, "--databases", RedisTotalDBs},
						},
					},
				},
			},
		},
	}

	deploymentClient := d.Clientset.AppsV1().Deployments(d.Namespace)
	_, err = deploymentClient.Create(ctx, redisDeployment, metav1.CreateOptions{})
	if err != nil {
		return ErrCreatingDeployment.Wrap(err)
	}

	hpa := &autoscalingv1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis-hpa",
		},
		Spec: autoscalingv1.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
				Kind:       redisAutoScalerKind,
				Name:       redisAutoScalerName,
				APIVersion: redisAutoScalerAPI,
			},
			MinReplicas:                    ptr.To[int32](redisReplicas),
			MaxReplicas:                    redisMaxReplicas,
			TargetCPUUtilizationPercentage: ptr.To[int32](redisCPUutilization),
		},
	}

	hpaClient := d.Clientset.AutoscalingV1().HorizontalPodAutoscalers(d.Namespace)
	_, err = hpaClient.Create(ctx, hpa, metav1.CreateOptions{})
	if err != nil {
		return ErrCreatingAutoScaler.Wrap(err)
	}

	if err = d.waitForRedisDeployment(ctx); err != nil {
		return ErrWaitingForRedis.Wrap(err)
	}

	if err := d.createRedisService(ctx); err != nil {
		return ErrCreatingService.Wrap(err)
	}

	if err := d.waitForRedisService(ctx); err != nil {
		return ErrWaitingForRedisService.Wrap(err)
	}

	logrus.Info("Redis deployed successfully.")
	return nil
}

func (d *Dispatcher) isRedisDeployed(ctx context.Context) (bool, error) {
	deploymentClient := d.Clientset.AppsV1().Deployments(d.Namespace)

	_, err := deploymentClient.Get(ctx, redisDeploymentName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, ErrGettingDeployment.Wrap(err)
	}

	return true, nil
}

func (d *Dispatcher) waitForRedisDeployment(ctx context.Context) error {
	for {
		deployment, err := d.Clientset.AppsV1().Deployments(d.Namespace).Get(ctx, redisDeploymentName, metav1.GetOptions{})
		if err == nil && deployment.Status.ReadyReplicas > 0 {
			break
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for Minio to be ready")
		case <-time.After(redisWaitRetry):
			// Retry after some seconds
		}
	}
	return nil
}

func (d *Dispatcher) createRedisService(ctx context.Context) error {
	serviceClient := d.Clientset.CoreV1().Services(d.Namespace)

	// Check if service already exists
	_, err := serviceClient.Get(ctx, redisServiceName, metav1.GetOptions{})
	if err == nil {
		logrus.Debugf("Service `%s` already exists.", redisServiceName)
		return nil
	}

	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      redisServiceName,
			Namespace: d.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{redisAppLabel: redisAppLabelValue},
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       RedisPort,
					TargetPort: intstr.FromInt(RedisPort),
				},
			},
			// Accessible only from within the cluster
			Type: v1.ServiceTypeClusterIP,
		},
	}

	_, err = serviceClient.Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	logrus.Debugf("Service %s created successfully.", redisServiceName)
	return nil
}

func (d *Dispatcher) waitForRedisService(ctx context.Context) error {
	for {
		redisService, err := d.Clientset.CoreV1().Services(d.Namespace).Get(ctx, redisServiceName, metav1.GetOptions{})
		if err == nil && redisService.Spec.Type == v1.ServiceTypeClusterIP {
			// Check if Redis is reachable
			ip, port, err := d.RedisIPPort(ctx)
			if err != nil {
				return err
			}

			if err := checkServiceConnectivity(fmt.Sprintf("%s:%d", ip, port)); err == nil {
				break
			}
		}

		select {
		case <-ctx.Done():
			return ErrTimeout.Wrap(fmt.Errorf("waiting for service %s to be ready", redisServiceName))
		case <-time.After(redisWaitRetry):
			// Retry after some seconds
		}
	}

	return nil
}

func checkServiceConnectivity(serviceEndpoint string) error {
	conn, err := net.DialTimeout("tcp", serviceEndpoint, 2*time.Second)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return ErrTimeout.Wrap(fmt.Errorf("connecting to %s: %v", serviceEndpoint, err))
		}
		return ErrFailedConnection.Wrap(fmt.Errorf("connecting to %s: %v", serviceEndpoint, err))
	}
	defer conn.Close()
	return nil // success
}

func (d *Dispatcher) RedisIPPort(ctx context.Context) (string, int32, error) {
	redisService, err := d.Clientset.CoreV1().Services(d.Namespace).Get(ctx, redisServiceName, metav1.GetOptions{})
	if err != nil {
		return "", 0, ErrGettingService.Wrap(err)
	}

	return redisService.Spec.ClusterIP, redisService.Spec.Ports[0].TargetPort.IntVal, nil
}
