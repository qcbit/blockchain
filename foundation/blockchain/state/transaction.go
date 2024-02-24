package state

import (
	"github.com/qcbit/blockchain/foundation/blockchain/database"
)

// UpsertWalletTransaction adds a transaction to the mempool.
func (s *State) UpsertWalletTransaction(signedTx database.SignedTx) error {

	// CORE NOTE: It's up the wallet to ensure the account has a proper balance and nonce.
	// Fees will be taken regardless.

	// Check the signed transaction has a proper signature, the from matches the signature,
	// and the from and to fields are properly formatted.
	if err := signedTx.Validate(s.genesis.ChainID); err != nil {
		return err
	}

	const oneUnitOfGas = 1
	tx := database.NewBlockTx(signedTx, s.genesis.GasPrice, oneUnitOfGas)
	if err := s.mempool.Upsert(tx); err != nil {
		return err
	}

	s.Worker.SignalStartMining()

	return nil
}

// UpsertNodeTransaction accepts a transaction from a node for inclusion.
func (s *State) UpsertNodeTransaction(tx database.BlockTx) error {
	// Check the signed transaction has a proper signature, the from matches
	// the signature, and the from and to fields are properly formatted.
	if err := tx.Validate(s.genesis.ChainID); err != nil {
		return err
	}

	if err := s.mempool.Upsert(tx); err != nil {
		return err
	}

	s.Worker.SignalStartMining()

	return nil
}
