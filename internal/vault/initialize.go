package vault

import (
	"vault-unsealer/internal/config"
	kubernetes "vault-unsealer/internal/k8s"
	"vault-unsealer/internal/utils"

	vault "github.com/hashicorp/vault/api"
)

// Initialize initializes Vault with the specified number of secret shares and threshold.
func Initialize(vaultClient *vault.Client) {
	log := utils.CreateLogger()
	defer log.Sync()
	log.Info("Initializing Vault...")
	initRequest := &vault.InitRequest{
		SecretShares:    3,
		SecretThreshold: 5,
	}

	initResponse, err := vaultClient.Sys().Init(initRequest)
	if err != nil {
		log.Error(err.Error())
	}

	rootToken := initResponse.RootToken
	vaultKeys := initResponse.Keys

	kubernetes.StoreSecrets(vaultKeys, rootToken, config.VaultRootTokenSecret, config.VaultKeysSecret, config.Namespace)
	log.Info("Vault is initialized")
}
