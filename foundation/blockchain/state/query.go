package state

import "github.com/qcbit/blockchain/foundation/blockchain/database"

// QueryLatest represents to query the latest block in the chain.
const QueryLatest = ^uint64(0) >> 1

//-----------------------------------------------------------------------------

// QueryAccount returns a copy of the account from the database.
func (s *State) QueryAccount(account database.AccountID) (database.Account, error) {
	return s.db.Query(account)
}

// QueryBlocksByNumber returns the set of blocks based on block numbers.
// This function reads the blockchain from disk first.
func (s *State) QueryBlocksByNumber(from, to uint64) []database.Block {
	if from == QueryLatest {
		from = s.db.LatestBlock().Header.Number
		to = from
	}
	if to == QueryLatest {
		to = s.db.LatestBlock().Header.Number
	}

	var out []database.Block
	for i := from; i <= to; i++ {
		block, err := s.db.GetBlock(i)
		if err != nil {
			s.evHandler("state: getblock: ERROR: %s", err)
			return nil
		}
		out = append(out, block)
	}

	return out
}
