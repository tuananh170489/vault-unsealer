package k8s

import (
	"context"
	"vault-unsealer/internal/utils"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetVaultKeys retrieves the keys from the specified secret in the specified namespace.
func GetVaultKeys(keysSecret, namespace string) ([]string, error) {
	log := utils.CreateLogger()
	defer log.Sync()
	clientset := CreateKubernetesClient()

	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), keysSecret, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Error("Secret not found")
			return nil, nil
		}
		log.Error(err.Error())
		return nil, err
	}

	keys := make([]string, 0, len(secret.Data))
	for _, value := range secret.Data {
		keys = append(keys, string(value))
	}
	return keys, nil
}
