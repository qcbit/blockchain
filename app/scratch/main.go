package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/qcbit/blockchain/foundation/blockchain/database"
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
	privateKey, err := crypto.LoadECDSA("zblock/accounts/q.ecdsa")
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

	fmt.Println("Signature:", hexutil.Encode(sig))

	publicKey, err := crypto.SigToPub(v, sig)
	if err != nil {
		return fmt.Errorf("failed to get public key: %w", err)
	}

	fmt.Println("Public address:", crypto.PubkeyToAddress(*publicKey).Hex())

	vv, r, s, err := ToVRSFromHexSignature(hexutil.Encode(sig))
	if err != nil {
		return fmt.Errorf("failed to convert signature: %w", err)
	}
	fmt.Println("V:", vv, "R:", r, "S:", s)

	// ----------------------------------------------------------------------------

	fmt.Println("============== TX ==============")

	testTx, err := database.NewTx(1,
		"0x42548bd7370Db094311A38E3d6ADC673F355514D",
		"0x6Fe6CF3c8fF57c58d24BfC869668F48BCbDb3BD9",
		100, 0, 0, nil)
	if err != nil {
		return fmt.Errorf("failed to create tx: %w", err)
	}

	signedTx, err := testTx.Sign(privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign tx: %w", err)
	}

	fmt.Println("Signed tx:", signedTx)

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

// ToVRSFromHexSignature converts a hex-encoded signature to the V, R, S components.
func ToVRSFromHexSignature(sig string) (v, r, s *big.Int, err error) {
	sigBytes, err := hex.DecodeString(sig[2:])
	if err != nil {
		return nil, nil, nil, err
	}

	r = new(big.Int).SetBytes(sigBytes[:32])
	s = new(big.Int).SetBytes(sigBytes[32:64])
	v = new(big.Int).SetBytes([]byte{sigBytes[64]})

	return v, r, s, nil
}
