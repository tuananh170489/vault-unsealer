package vault

import (
	"vault-unsealer/internal/utils"

	vault "github.com/hashicorp/vault/api"
)

// CreateVaultClient creates a new Vault client
func CreateVaultClient(vaultURL string) *vault.Client {
	log := utils.CreateLogger()
	defer log.Sync()
	config := &vault.Config{
		Address: vaultURL,
	}
	vaultClient, err := vault.NewClient(config)
	if err != nil {
		log.Error(err.Error())
	}
	return vaultClient
}
