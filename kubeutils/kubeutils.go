package kubeutils

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/Azure/adx-automation-agent/common"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// IsInCluster returns True if the current environment is in a Kubernetes cluster
func IsInCluster() bool {
	_, exists := os.LookupEnv(common.EnvKeyStoreName)
	return exists
}

// CreateKubeClientset creates a new kubernetes clientset
func CreateKubeClientset() (clientset *kubernetes.Clientset, err error) {
	var config *rest.Config

	if IsInCluster() {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("Fail to create in cluster kubernetes config | Error %s", err)
		}
	} else {
		currentUser, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("Fail to get the current user")
		}

		kubeconfigPath := filepath.Join(currentUser.HomeDir, ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("Fail to create kubernetes config from %s | Error %s", kubeconfigPath, err)
		}
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Fail to create kubernetes client | Error %s", err)
	}

	return
}

// TryCreateKubeClientset creates a new kubernetes clientset. If it fails return nil
func TryCreateKubeClientset() *kubernetes.Clientset {
	if client, err := CreateKubeClientset(); err == nil {
		return client
	}
	return nil
}
