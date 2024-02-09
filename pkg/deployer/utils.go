package deployer

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// TODO: improve this function
func GetDefaultRegistryImageName(imageName string) string {
	return "ttl.sh/" + imageName + ":24h"
}

func KubeClientset(kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, ErrBuildingKubeconfig.Wrap(err)
	}

	return kubernetes.NewForConfig(config)
}
