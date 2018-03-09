package kubeutils

import (
	"fmt"
	"os/user"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// CreateKubeClientset creates a new kubernetes clientset
func CreateKubeClientset() (clientset *kubernetes.Clientset, err error) {
	var config *rest.Config

	// Always try to get in-cluster config first
	config, err = rest.InClusterConfig()
	if err != nil {
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
