package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

type Tx struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	Value uint64 `json:"value"`
}

func main() {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	msg := []byte("hello, world")
	hash := sha256.Sum256(msg)

	sig, err := ecdsa.SignASN1(rand.Reader, privateKey, hash[:])
	if err != nil {
		panic(err)
	}
	fmt.Println("Signature:", base64.StdEncoding.EncodeToString(sig))

	valid := ecdsa.VerifyASN1(&privateKey.PublicKey, hash[:], sig)
	publicKey := privateKey.PublicKey
	fmt.Println("Public key:", base64.StdEncoding.EncodeToString(elliptic.Marshal(publicKey.Curve, publicKey.X, publicKey.Y)))
	fmt.Println("Valid signature:", valid)
}

