package models

type SignedToken struct {
	Nonce     []byte
	Signature []byte
}
