package main

import (
	"vault-unsealer/internal/config"
	"vault-unsealer/internal/vault"
)

func main() {
	config.LoadEnvVariables()
	vaultClient := vault.CreateVaultClient(config.VaultURL)
	vault.CheckSealStatus(vaultClient)
}
