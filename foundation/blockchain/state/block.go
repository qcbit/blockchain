package state

import (
	"context"
	"errors"

	"github.com/qcbit/blockchain/foundation/blockchain/database"
)

// ErrNoTransactions is returned when there are no transactions
var ErrNoTransactions = errors.New("no transactions in the mempool")

// MineNewBlock attempts to create a new block with a
// proper hash that can become the next block in the chain.
func (s *State) MineNewBlock(ctx context.Context) (database.Block, error) {
	defer s.evHandler("viewer: MineNewBlock: MINING: completed")

	s.evHandler("state: MineNewBlock: MINING: check mempool count")

	// Are there enough transactions in the pool?
	if s.mempool.Count() == 0 {
		return database.Block{}, ErrNoTransactions
	}

	// Pick the best transactions from the mempool.
	trans := s.mempool.PickBest(s.genesis.TransPerBlock)

	difficulty := s.genesis.Difficulty

	// Attempt to create a new block by solving the POW puzzle. This can be canceled.
	block, err := database.POW(ctx, database.POWArgs{
		BeneficiaryID: s.beneficiaryID,
		Difficulty:    difficulty,
		MiningReward:  s.genesis.MiningReward,
		PrevBlock:     s.db.LatestBlock(),
		StateRoot:     s.db.HashState(),
		Trans:         trans,
		EvHandler:     s.evHandler,
	})
	if err != nil {
		return database.Block{}, err
	}

	// Check one more time we were not canceled.
	if ctx.Err() != nil {
		return database.Block{}, ctx.Err()
	}

	s.evHandler("state: MineNewBlock: MINING: validate and update database")

	// Validate the block and then update the blockchain database.
	if err := s.validateUpdateDatabase(block); err != nil {
		return database.Block{}, err
	}

	return block, nil
}

// ProcessProposedBlock takes a block received from a peer, validates,
// if valid, adds the block to the local blockchain.
func (s *State) ProcessProposedBlock(block database.Block) error {
	s.evHandler("state: ProcessProposedBlock: started: prevBlk[%s]: newBlk[%s]: numTrans[%d]", 
		block.Header.PrevBlockHash, block.Hash(), len(block.MerkleTree.Values()))
	defer s.evHandler("state: ProcessProposedBlock: completed: newBlk[%s]", block.Hash())

	// Validate the block and then update the blockchain database.
	if err := s.validateUpdateDatabase(block); err != nil {
		return err
	}

	// Stop runMiningOperation
	s.Worker.SignalCancelMining()

	return nil
}

//---------------------------------------------------

// validateUpdateDatabase takes the block and validates the block against the
// consensus rules. If the block passes, then the state of the node is updated
// including adding the block to disk.
func (s *State) validateUpdateDatabase(block database.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.evHandler("state: validateUpdateDatabase: validate block")

	// CORE NOTE: Logic could be added to determine which node mined the block.
	// If the block is mined by this node, even if a peer beats it to this method
	// for the same block number, the peer block could be replaced with this node's
	// and attempt to have other peers accept its block instead.

	if err := block.ValidateBlock(s.db.LatestBlock(), s.db.HashState(), s.evHandler); err != nil {
		return err
	}

	s.evHandler("state: validateUpdateDatabase: write to disk")

	// Write the new block to the chain on disk.
	if err := s.db.Write(block); err != nil {
		return err
	}
	s.db.UpdateLatestBlock(block)

	s.evHandler("state: validateUpdateDatabase: update accounts and remove from mempool")

	// Process the transactions and update the accounts.
	for _, tx := range block.MerkleTree.Values() {
		s.evHandler("state: validateUpdateDatabase: tx[%s] update and remove", tx)

		// Remove this transaction from the mempool.
		s.mempool.Delete(tx)

		// Apply the balance changes based on this transaction.
		if err := s.db.ApplyTransaction(block, tx); err != nil {
			s.evHandler("state: validateUpdateDatabase: WARNING: %s", err)
			continue
		}
	}

	s.evHandler("state: validateUpdateDatabase: apply mining reward")

	// Apply the mining reward for this block.
	s.db.ApplyMiningReward(block)

	// Send an event about this new block
	// s.blockEvent(block)

	return nil
}
