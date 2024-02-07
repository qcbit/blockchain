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
	// if err := s.validateUpdateDatabase(block); err != nil {
	// 	return database.Block{}, err
	// }

	return block, nil
}
