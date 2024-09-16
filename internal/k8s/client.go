package k8s

import (
	"vault-unsealer/internal/utils"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// CreateKubernetesClient creates a new Kubernetes client
func CreateKubernetesClient() *kubernetes.Clientset {
	log := utils.CreateLogger()
	defer log.Sync()
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error("Failed to create in-cluster config", zap.Error(err))
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error("Failed to create Kubernetes client", zap.Error(err))
	}
	return clientset
}
