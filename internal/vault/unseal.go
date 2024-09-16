package vault

import (
	"vault-unsealer/internal/config"
	kubernetes "vault-unsealer/internal/k8s"
	"vault-unsealer/internal/utils"

	vault "github.com/hashicorp/vault/api"
)

// Unseal unseals Vault using the stored keys
func Unseal(vaultClient *vault.Client) {
	log := utils.CreateLogger()
	defer log.Sync()
	keys, err := kubernetes.GetVaultKeys(config.VaultKeysSecret, config.Namespace)
	if err != nil {
		log.Error(err.Error())
	}

	for _, key := range keys {
		response, err := vaultClient.Sys().Unseal(key)
		if err != nil {
			log.Error(err.Error())
		}
		if response.Sealed {
			log.Info("Unsealing Vault...")
		} else {
			log.Info("Vault is unsealed")
		}
	}
}
