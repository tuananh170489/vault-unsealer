package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	vaultURL             string
	namespace            string
	vaultRootTokenSecret string
	vaultKeysSecret      string
	log                  = logrus.New()
)

func init() {
	// Get environment variables
	vaultURL = getEnv("VAULT_ADDR", "http://localhost:8200")
	namespace = getEnv("NAMESPACE", "default")
	vaultRootTokenSecret = getEnv("VAULT_ROOT_TOKEN_SECRET", "vault-root-token")
	vaultKeysSecret = getEnv("VAULT_KEYS_SECRET", "vault-keys")
}

// getEnv returns the value of an environment variable specified by the key.
//
// If the environment variable is found, its value is returned.
// If the environment variable is not found, the fallback value is returned.
// The returned value is a string.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	vaultClient := createVaultClient(vaultURL)
	checkSealStatus(vaultClient)
}

func createVaultClient(vaultURL string) *vault.Client {
	config := &vault.Config{
		Address: vaultURL,
	}
	vaultClient, err := vault.NewClient(config)
	if err != nil {
		log.Errorf("Unable to create Vault client: %v", err)
	}
	return vaultClient
}

func createKubernetesClient() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Errorf("Unable to create Kubernetes client: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Errorf("Unable to create Kubernetes client: %v", err)
	}
	return clientset
}

func checkSealStatus(vaultClient *vault.Client) {
	log.Info("The Vault Unsealer is running...")

	for {
		// Get the current seal status of Vault
		log.Info("Checking Vault seal status...")
		sealStatusResponse, err := vaultClient.Sys().SealStatus()
		if err != nil {
			log.Errorf("Unable to get Vault seal status: %v", err)
		}

		// If Vault is not initialized, initialize it
		if !sealStatusResponse.Initialized {
			initializeVault(vaultClient)
		}

		// If Vault is sealed, unseal it
		if sealStatusResponse.Sealed {
			unsealVault(vaultClient)
		}

		// Sleep for 5 seconds before checking the seal status again
		time.Sleep(5 * time.Second)
	}
}

func initializeVault(vaultClient *vault.Client) {
	log.Info("Initializing Vault...")

	initRequest := &vault.InitRequest{
		SecretShares:    5,
		SecretThreshold: 5,
	}

	initResponse, err := vaultClient.Sys().Init(initRequest)
	if err != nil {
		log.Errorf("Unable to initialize Vault: %v", err)
	}

	rootToken := initResponse.RootToken
	vaultKeys := initResponse.Keys

	// Save the root token and keys in Kubernetes secrets
	saveRootTokenAndKeys(vaultKeys, rootToken, vaultRootTokenSecret, vaultKeysSecret, namespace)
	log.Info("Vault is initialized")
}

func unsealVault(vaultClient *vault.Client) {
	keys, err := getVaultKeys(vaultKeysSecret, namespace)
	if err != nil {
		log.Errorf("Unable to get Vault keys: %v", err)
	}

	// Unseal the Vault using the keys
	for _, key := range keys {
		response, err := vaultClient.Sys().Unseal(key)
		if err != nil {
			log.Errorf("Unable to unseal Vault: %v", err)
		}
		if response.Sealed {
			log.Info("Unsealing Vault...")
		} else {
			log.Info("Vault is unsealed")
		}
	}
}

func getVaultKeys(keysSecret, namespace string) ([]string, error) {
	// Get the Kubernetes clientset
	clientset := createKubernetesClient()

	// Check if the secret exists
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), keysSecret, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Errorf("Secret %s does not exist in namespace %s.", keysSecret, namespace)
		}
		log.Errorf("Unable to get secret: %v", err)
	}

	// Extract the Vault keys from the secret
	keys := make([]string, 0, len(secret.Data))
	for key := range secret.Data {
		keys = append(keys, string(secret.Data[key]))
	}
	return keys, nil
}

func checkSecretExist(clientset *kubernetes.Clientset, namespace, secretName string) (bool, error) {
	_, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
	}
	return true, nil
}

func saveRootTokenAndKeys(keys []string, rootToken, rootTokenSecret, keysSecret, namespace string) {
	// Get the Kubernetes clientset
	clientset := createKubernetesClient()

	// Check if both secrets exist
	existRootTokenSecret, err := checkSecretExist(clientset, namespace, rootTokenSecret)
	if err != nil {
		log.Errorf("Unable to get secret: %v", err)
	}
	existKeysSecret, err := checkSecretExist(clientset, namespace, keysSecret)
	if err != nil {
		log.Errorf("Unable to get secret: %v", err)
	}

	if existRootTokenSecret && existKeysSecret {
		log.Errorf("Both secrets %s and %s already exist in namespace %s. Please delete them first to proceed.", rootTokenSecret, keysSecret, namespace)
	}

	// Create a Kubernetes secret object with the root token as data
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

	// Create a Kubernetes secret object with the keys as data
	secretVaultKeys := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      keysSecret,
			Namespace: namespace,
		},
		Type:       v1.SecretTypeOpaque,
		StringData: convertListToMap(keys),
	}

	// Create both secrets in the desired namespace concurrently
	var wg sync.WaitGroup
	wg.Add(2)
	go createSecret(clientset, namespace, secretVaultRootToken, &wg)
	go createSecret(clientset, namespace, secretVaultKeys, &wg)
	wg.Wait()
	log.Info("Secrets created successfully")
}

func createSecret(clientset *kubernetes.Clientset, namespace string, secret *v1.Secret, wg *sync.WaitGroup) {
	defer wg.Done()
	_, err := clientset.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("Unable to create secret: %v", err)
	}
}

// convertListToMap converts a list of keys into a map with index as the key and the key value as the value.
//
// Parameters:
// - keys: a slice of strings representing the keys to be converted.
//
// Returns:
// - resultMap: a map[string]string where the keys are formatted as "Unseal Key {index}" and the values are the corresponding keys from the input slice.
func convertListToMap(keys []string) map[string]string {
	resultMap := make(map[string]string)
	for index, key := range keys {
		resultMap[fmt.Sprintf("key%d", index)] = key
	}
	return resultMap
}
