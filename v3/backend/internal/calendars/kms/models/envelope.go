package models

type Envelope struct {
	Version    int    `json:"version"`
	KMSKeyName string `json:"kms_key_name"`
	WrappedDEK string `json:"wrapped_dek"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}
