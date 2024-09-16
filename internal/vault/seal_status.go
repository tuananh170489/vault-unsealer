package vault

import (
	"time"
	"vault-unsealer/internal/utils"

	vault "github.com/hashicorp/vault/api"
)

// CheckSealStatus checks the seal status of Vault and unseals it if it is sealed
func CheckSealStatus(vaultClient *vault.Client) {
	log := utils.CreateLogger()
	defer log.Sync()
	log.Info("The Vault Unsealer is running...")

	for {
		log.Info("Checking Vault seal status...")
		sealStatusResponse, err := vaultClient.Sys().SealStatus()
		if err != nil {
			log.Error(err.Error())
		}

		if !sealStatusResponse.Initialized {
			Initialize(vaultClient)
		}

		if sealStatusResponse.Sealed {
			Unseal(vaultClient)
		}

		time.Sleep(5 * time.Second)
	}
}
