// Package signature handles all lower level support for signing transactions.
package signature

import (
	"crypto/sha256"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// ZeroHash is the hash of an empty string.
const ZeroHash = "0x0000000000000000000000000000000000000000000000000000000000000000"

// QID is an arbitrary value added to the v component of the signature similar to Ethereum and Bitcoin.
const QID = 29

// Hash returns a unique hash for the data.
func Hash(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ZeroHash
	}

	hash := sha256.Sum256(data)
	return hexutil.Encode(hash[:])
}

// Sign uses the specified private key to sign the data.
func Sign(value any, privateKey *ecdsa.PrivateKey) (v, r, s *big.Int, err error) {
	// Prepare the data to be signed.
	data, err := stamp(value)
	if err != nil {
		return nil, nil, nil, err
	}

	// Sign the hash with the private key to produce a signature.
	sig, err := crypto.Sign(data, privateKey)
	if err != nil {
		return nil, nil, nil, err
	}

	// Extract the bytes for the original public key.
	publicKeyOrg := privateKey.Public()
	publicKeyECDSA, ok := publicKeyOrg.(*ecdsa.PublicKey)
	if !ok {
		return nil, nil, nil, err
	}
	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)

	// Data and signature check with public key.
	rs := sig[:crypto.RecoveryIDOffset]
	if !crypto.VerifySignature(publicKeyBytes, data, rs) {
		return nil, nil, nil, errors.New("signature verification failed")
	}

	// Convert the signature bytes into the v, r, s components.
	v, r, s = toSignature(sig)

	return v, r, s, nil
}

// ToSignatureBytes converts the v, r, s components into the original 65 bytes signature without QID.
func ToSignatureBytes(v, r, s *big.Int) []byte {
	sig := make([]byte, crypto.SignatureLength)

	rBytes := make([]byte, 32)
	r.FillBytes(rBytes)
	copy(sig, rBytes)

	sBytes := make([]byte, 32)
	s.FillBytes(sBytes)
	copy(sig[32:], sBytes)

	sig[64] = byte(v.Uint64() - QID)

	return sig
}

// SignatureString returns the signature in the [R|S|V] format.
func SignatureString(v, r, s *big.Int) string {
	return hexutil.Encode(ToSignatureBytesWithQID(v, r, s))
}

// ToSignatureBytesWithQID converts the v, r, s components into the original 65 bytes signature with QID.
func ToSignatureBytesWithQID(v, r, s *big.Int) []byte {
	sig := ToSignatureBytes(v, r, s)
	sig[64] = byte(v.Uint64())
	return sig
}

// ----------------------------------------------------------------------------

// stamp returns a 32-byte hash of the data with the stamp embedded.
func stamp(value any) ([]byte, error) {
	// Marshal the data.
	v, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	// This stamp is used to identify the data as being signed by the blockchain.
	stamp := []byte(fmt.Sprintf("\x19Q Signed Message:\n%d", len(v)))

	// Stamp the data outputting a 32-byte hash.
	data := crypto.Keccak256(stamp, v)

	return data, nil
}

// VerifySignature verifies the signature conforms to the standards.
func VerifySignature(v, r, s *big.Int) error {
	// Check the recovery id is either 0 or 1.
	uintV := v.Uint64() - QID
	if uintV != 0 && uintV != 1 {
		return errors.New("invalid recovery ID")
	}

	// Check the signature values are valid.
	if !crypto.ValidateSignatureValues(byte(uintV), r, s, false) {
		return errors.New("invalid signature values")
	}

	return nil
}

// FromAddress extracts the address from the signature that signed the data.
func FromAddress(value any, v, r, s *big.Int) (string, error) {
	// Prepare the data for public key extraction.
	data, err := stamp(value)
	if err != nil {
		return "", err
	}

	// Convert the R,S,V format into the original 65 bytes signature.
	sig := ToSignatureBytes(v, r, s)

	// Capture the public key associated with the data and signature.
	publicKey, err := crypto.SigToPub(data, sig)
	if err != nil {
		return "", err
	}

	// Extract the account address from the public key.
	return string(crypto.PubkeyToAddress(*publicKey).Hex()), nil
}

// toSignature converts the signature bytes into the v, r, s components.
func toSignature(sig []byte) (v, r, s *big.Int) {
	r = big.NewInt(0).SetBytes(sig[:32])
	s = big.NewInt(0).SetBytes(sig[32:64])
	v = big.NewInt(0).SetBytes([]byte{sig[64] + QID})
	return v, r, s
}
