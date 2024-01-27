// Package state is the core API for the blockchain
// and implements all the business rules and processing.
package state

import (
	"sync"

	"github.com/qcbit/blockchain/foundation/blockchain/database"
	"github.com/qcbit/blockchain/foundation/blockchain/genesis"
	"github.com/qcbit/blockchain/foundation/blockchain/mempool"
)

// EventHandler defines a function that is called when events
// occur in the processing of persisting blocks.
type EventHandler func(v string, args ...any)

//------------------------------------------------------------

// Config represents the configuration required to
// start the blockchain node.
type Config struct {
	BeneficiaryID  database.AccountID
	Genesis        genesis.Genesis
	SelectStrategy string
	EvHandler      EventHandler
}

// State manages the blockchain database.
type State struct {
	mu sync.RWMutex

	beneficiaryID database.AccountID
	evHandler     EventHandler

	genesis genesis.Genesis
	mempool *mempool.Mempool
	db      *database.Database
}

// New constructs a blockchain for data management.
func New(cfg Config) (*State, error) {
	// Build a safe event handler function for use.
	ev := func(v string, args ...any) {
		if cfg.EvHandler != nil {
			cfg.EvHandler(v, args...)
		}
	}

	// Access the storage for the blockchain.
	db, err := database.New(cfg.Genesis, ev)
	if err != nil {
		return nil, err
	}

	// Construct a mempool with the specified sort strategy.
	mempool, err := mempool.NewWithStrategy(cfg.SelectStrategy)
	if err != nil {
		return nil, err
	}

	// Create the State to provide support for managing the blockchain.
	return &State{
		beneficiaryID: cfg.BeneficiaryID,
		evHandler:     ev,

		genesis: cfg.Genesis,
		mempool: mempool,
		db:      db,
	}, nil
}

// Shutdown cleanly brings the node down.
func (s *State) Shutdown() error {
	s.evHandler("state: shutdown: start")
	defer s.evHandler("state: shutdown: complete")
	return nil
}

// MempoolLength returns the number of transactions in the mempool.
func (s *State) MempoolLength() int {
	return s.mempool.Count()
}

// Mempool returns a copy of the mempool.
func (s *State) Mempool() []database.BlockTx {
	return s.mempool.PickBest()
}

// UpsertMempool adds a new transaction to the mempool
func (s *State) UpsertMempool(tx database.BlockTx) error {
	return s.mempool.Upsert(tx)
}

// Accounts returns a copy of the database accounts.
func (s *State) Accounts() map[database.AccountID]database.Account {
	return s.db.Copy()
}