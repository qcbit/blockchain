package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
)

type Tx struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	Value  uint64 `json:"value"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
}

func run() error {
	// privateKey, err := GenerateKey()
	privateKey, err := crypto.HexToECDSA("9f332e3700d8fc2446eaf6d15034cf96e0c2745e40353deef032a5dbf1dfed93")
	if err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}

	tx := Tx{
		FromID: "alice",
		ToID:   "bob",
		Value:  100,
	}

	data, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("failed to marshal tx: %w", err)
	}

	stamp := []byte(fmt.Sprintf("\x19Q Signed Message:\n%d", len(data)))

	v := crypto.Keccak256(stamp, data)

	// 32-byte hashed data
	sig, err := crypto.Sign(v, privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign tx: %w", err)
	}

	fmt.Println("Signature:", hex.EncodeToString(sig))

	publicKey, err := crypto.SigToPub(v, sig)
	if err != nil {
		return fmt.Errorf("failed to get public key: %w", err)
	}

	fmt.Println("Public address:", crypto.PubkeyToAddress(*publicKey).Hex())

	return nil
}

func GenerateKey() (*ecdsa.PrivateKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalln(err)
		return nil, err
	}

	return privateKey, nil
}
