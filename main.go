package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/vault/api"
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
)

func init() {
	// Get environment variables
	vaultURL = getEnv("VAULT_URL", "http://localhost:8200")
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
	clientset := createKubernetesClient()

	checkSealStatus(vaultClient, clientset, namespace, vaultKeysSecret)

}

func createVaultClient(vaultURL string) *api.Client {
	vaultClient, err := api.NewClient(&api.Config{Address: vaultURL})
	if err != nil {
		log.Fatal(err)
	}
	return vaultClient
}

func createKubernetesClient() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	return clientset
}

func checkSealStatus(vaultClient *api.Client, clientset *kubernetes.Clientset, namespace, secretName string) {
	for {
		// Check if Vault is initialized, and if not, initialize it
		initializeVault(vaultClient)

		// Get the current seal status of Vault
		sealStatusResponse, err := vaultClient.Sys().SealStatus()
		if err != nil {
			log.Fatal(err)
		}

		// If Vault is sealed, unseal it
		if sealStatusResponse.Sealed {
			unsealVault(vaultClient)
		}

		// Sleep for 5 seconds before checking the seal status again
		time.Sleep(5 * time.Second)
	}
}

func initializeVault(vaultClient *api.Client) {
	// Check if Vault is initialized, and if not, initialize it
	initialized, err := vaultClient.Sys().InitStatus()
	if err != nil {
		log.Fatal(err)
	}
	if !initialized {
		log.Println("Vault is not initialized, initializing...")
		initRequest := &api.InitRequest{
			SecretShares:    5,
			SecretThreshold: 3,
		}

		initResponse, err := vaultClient.Sys().Init(initRequest)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Vault has been initialized")

		rootToken := initResponse.RootToken
		keys := initResponse.Keys

		// Save the root token and keys in Kubernetes
		log.Println("Saving root token and keys...")
		saveRootTokenAndKeys(rootToken, keys, vaultRootTokenSecret, vaultKeysSecret, namespace)
	}
}

func unsealVault(vaultClient *api.Client) {
	keys, err := getVaultKeys(vaultKeysSecret, namespace)
	if err != nil {
		log.Fatal(err)
	}

	// Unseal the Vault using the keys
	for _, key := range keys {
		if _, err := vaultClient.Sys().Unseal(key); err != nil {
			log.Fatal(err)
		}
	}

	// Get the seal status response
	sealStatusResponse, err := vaultClient.Sys().SealStatus()
	if err != nil {
		log.Fatal(err)
	}

	// Check if Vault is unsealed
	if sealStatusResponse.Sealed {
		log.Println("Vault is not unsealed")
	}

	log.Println("Vault has been unsealed")
}

func getVaultKeys(keysSecret, namespace string) ([]string, error) {
	// Get the Kubernetes clientset
	clientset := createKubernetesClient()

	// Check if the secret exists
	exist, err := checkSecretExist(clientset, namespace, keysSecret)
	if err != nil {
		return nil, err
	}

	if !exist {
		return nil, fmt.Errorf("secret %s does not exist", keysSecret)
	}

	// Check if the secret exists
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), keysSecret, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Extract the keys from the secret
	keys := convertMapToList(secret.StringData)

	return keys, nil
}

func checkSecretExist(clientset *kubernetes.Clientset, namespace string, secretName string) (bool, error) {
	_, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func saveRootTokenAndKeys(rootToken string, keys []string, rootTokenSecret string, keysSecret string, namespace string) {
	// Get the Kubernetes clientset
	clientset := createKubernetesClient()
	ctx := context.Background()

	// Check if both secrets exist
	existRootTokenSecret, err := checkSecretExist(clientset, namespace, rootTokenSecret)
	if err != nil {
		log.Fatal(err)
	}
	existKeysSecret, err := checkSecretExist(clientset, namespace, keysSecret)
	if err != nil {
		log.Fatal(err)
	}

	if existRootTokenSecret && existKeysSecret {
		log.Println("Secrets already exist. Skipping creation.")
		return
	}

	// Get both secrets in the desired namespace concurrently
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := clientset.CoreV1().Secrets(namespace).Get(ctx, rootTokenSecret, metav1.GetOptions{})
		if err != nil {
			log.Fatal(err)
		}
	}()
	go func() {
		defer wg.Done()
		_, err := clientset.CoreV1().Secrets(namespace).Get(ctx, keysSecret, metav1.GetOptions{})
		if err != nil {
			log.Fatal(err)
		}
	}()
	wg.Wait()

	// Create a Kubernetes secret object with the root token as data
	secretVaultRootToken := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rootTokenSecret,
			Namespace: namespace,
		},
		Type: v1.SecretTypeOpaque,
		StringData: map[string]string{
			"Initial Root Token": rootToken,
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
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secretVaultRootToken, metav1.CreateOptions{})
		if err != nil {
			log.Fatal(err)
		}
	}()
	go func() {
		defer wg.Done()
		_, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secretVaultKeys, metav1.CreateOptions{})
		if err != nil {
			log.Fatal(err)
		}
	}()
	wg.Wait()

	log.Println("Root token and keys saved to Kubernetes secret")
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
		resultMap[fmt.Sprintf("Unseal Key %d", index)] = key
	}
	return resultMap
}

// convertMapToList converts a map[string]string into a list of strings.
//
// The data parameter is a map with string keys and string values.
// It represents the input data to be converted.
//
// The function returns a list of strings, which is the result of
// converting the map values into a list.
func convertMapToList(data map[string]string) []string {
	var keys []string
	for _, v := range data {
		keys = append(keys, v)
	}
	return keys
}
