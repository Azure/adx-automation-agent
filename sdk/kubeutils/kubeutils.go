package kubeutils

import (
	"fmt"
	"os/user"
	"path/filepath"

	"github.com/Azure/adx-automation-agent/sdk/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// TryGetSystemConfig retrieves the value of given key in a01 system config.
func TryGetSystemConfig(key string) (value string, exists bool) {
	clientset := TryCreateKubeClientset()
	if clientset == nil {
		return "", false
	}

	configmap, err := clientset.CoreV1().ConfigMaps(common.GetCurrentNamespace("default")).Get(common.SystemConfigMapName, metav1.GetOptions{})
	if err != nil {
		return "", false
	}

	value, exists = configmap.Data[key]
	return
}

// TryGetSecretInBytes retrieves the value of given key in the given secret in current namespace.
func TryGetSecretInBytes(secret string, key string) (value []byte, exists bool) {
	clientset := TryCreateKubeClientset()
	if clientset == nil {
		return nil, false
	}

	sec, err := clientset.CoreV1().Secrets(common.GetCurrentNamespace("default")).Get(secret, metav1.GetOptions{})
	if err != nil {
		return nil, false
	}

	value, exists = sec.Data[key]
	return
}
