package k8s

import (
	"context"
	"fmt"
	"sync"
	"vault-unsealer/internal/utils"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// StoreSecrets stores the root token and keys in a Kubernetes secret
func StoreSecrets(keys []string, rootToken, rootTokenSecret, keysSecret, namespace string) {
	log := utils.CreateLogger()
	defer log.Sync()
	clientset := CreateKubernetesClient()

	secretVaultRootToken := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rootTokenSecret,
			Namespace: namespace,
		},
		Type: v1.SecretTypeOpaque,
		StringData: map[string]string{
			"rootToken": rootToken,
		},
	}

	secretVaultKeys := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      keysSecret,
			Namespace: namespace,
		},
		Type:       v1.SecretTypeOpaque,
		StringData: utils.ConvertListToMap(keys),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := createSecret(clientset, namespace, secretVaultRootToken)
		if err != nil {
			log.Error(err.Error())
		}
	}()

	go func() {
		defer wg.Done()
		err := createSecret(clientset, namespace, secretVaultKeys)
		if err != nil {
			log.Error(err.Error())
		}
	}()

	wg.Wait()
	log.Info("Secrets created successfully")
}

func createSecret(clientset *kubernetes.Clientset, namespace string, secret *v1.Secret) error {
	_, err := clientset.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return fmt.Errorf("secret %s already exists in namespace %s", secret.Name, namespace)
		}
		return fmt.Errorf("failed to create secret %s in namespace %s: %v", secret.Name, namespace, err)
	}
	return nil
}
