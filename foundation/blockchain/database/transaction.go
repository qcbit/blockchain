package database

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/qcbit/blockchain/foundation/blockchain/signature"
)

// Tx represents a transaction.
type Tx struct {
	ChainID uint16    `json:"chain_id"` // Ethereum: The chain ID in the genesis file.
	FromID  AccountID `json:"from_id"`  // Ethereum: The transaction sender.
	ToID    AccountID `json:"to_id"`    // Ethereum: The transaction recipient.
	Value   uint64    `json:"value"`    // Ethereum: The unit amount to transfer.
	Nonce   uint64    `json:"nonce"`    // Ethereum: Unique number for the transaction.
	Tip     uint64    `json:"tip"`      // Ethereum: The unit amount to tip the miner.
	Data    []byte    `json:"data"`     // Ethereum: The input data for the transaction.
}

// NewTx creates a new transaction.
func NewTx(chainID uint16, fromID, toID AccountID, value, nonce, tip uint64, data []byte) (Tx, error) {
	if !fromID.IsAccountID() {
		return Tx{}, errors.New("invalid from ID")
	}
	if !toID.IsAccountID() {
		return Tx{}, errors.New("invalid to ID")
	}

	return Tx{
		ChainID: chainID,
		FromID:  fromID,
		ToID:    toID,
		Value:   value,
		Nonce:   nonce,
		Tip:     tip,
		Data:    data,
	}, nil
}

// Sign signs the transaction.
func (tx Tx) Sign(privateKey *ecdsa.PrivateKey) (SignedTx, error) {
	// Sign the transaction with the private key to produce a signature.
	v, r, s, err := signature.Sign(tx, privateKey)
	if err != nil {
		return SignedTx{}, err
	}

	// Return the signed transaction by adding the signature in the [R|S|V] format.
	return SignedTx{
		Tx: tx,
		V:  v,
		R:  r,
		S:  s,
	}, nil
}

// SignedTx represents a signed transaction.
type SignedTx struct {
	Tx
	V *big.Int `json:"v"` // Ethereum: The recovery ID.
	R *big.Int `json:"r"` // Ethereum: The first 32 bytes of the ECDSA signature.
	S *big.Int `json:"s"` // Ethereum: The second 32 bytes of the ECDSA signature.
}

// Validate verifies the transaction has a proper signature conforming to the standards.
// It checks that the from field matches the account that signed the transaction.
// It checks the format of the from and to fields.
func (tx SignedTx) Validate(chainID uint16) error {
	if tx.ChainID != chainID {
		return errors.New("invalid chain ID")
	}

	if !tx.FromID.IsAccountID() {
		return errors.New("invalid from ID")
	}

	if !tx.ToID.IsAccountID() {
		return errors.New("invalid to ID")
	}

	if tx.FromID == tx.ToID {
		return errors.New("from and to IDs are the same")
	}

	if err := signature.VerifySignature(tx.V, tx.R, tx.S); err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	address, err := signature.FromAddress(tx.Tx, tx.V, tx.R, tx.S)
	if err != nil {
		return fmt.Errorf("failed to get address: %w", err)
	}

	if address != string(tx.FromID) {
		return errors.New("from address does not match signature")
	}

	return nil
}

// SignatureString returns the signature as a string.
func (tx SignedTx) SignatureString() string {
	return signature.SignatureString(tx.V, tx.R, tx.S)
}

// String implements the Stringer interface.
func (tx SignedTx) String() string {
	return fmt.Sprintf("%s: %d", tx.FromID, tx.Nonce)
}