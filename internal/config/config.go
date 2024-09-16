package config

import (
	"os"
)

var (
	VaultURL             string
	Namespace            string
	VaultRootTokenSecret string
	VaultKeysSecret      string
)

func LoadEnvVariables() {
	VaultURL = getEnv("VAULT_ADDR", "http://localhost:8200")
	Namespace = getEnv("NAMESPACE", "default")
	VaultRootTokenSecret = getEnv("VAULT_ROOT_TOKEN_SECRET", "vault-root-token")
	VaultKeysSecret = getEnv("VAULT_KEYS_SECRET", "vault-keys")
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
