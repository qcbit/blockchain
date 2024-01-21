// Package database handles all lower level support for maintaining the
// blockchain database.  This includes the management of an in-memory database
// of account information.
package database

import (
	"errors"
	"sync"

	"github.com/qcbit/blockchain/foundation/blockchain/genesis"
)

// Database manages data related to accounts who have transacted on the blockchain.
type Database struct {
	mu       sync.RWMutex
	genesis  genesis.Genesis
	accounts map[AccountID]Account
}

// New constructs a new database and applies account genesis information.
// It reads/writes the blockchain database on disk if a dbPath is provided.
func New(genesis genesis.Genesis, evHandler func(v string, args ...any)) (*Database, error) {
	db := Database{
		genesis:  genesis,
		accounts: make(map[AccountID]Account),
	}

	// Update the database with account balance informaton from the genesis block.
	for accountStr, balance := range genesis.Balances {
		accountID, err := ToAccountID(accountStr)
		if err != nil {
			return nil, err
		}
		db.accounts[accountID] = newAccount(accountID, balance)

		evHandler("Account: %s, Balance: %d", accountID, balance)
	}

	return &db, nil
}

// Remove removes an account from the database.
func (db *Database) Remove(accountID AccountID) {
	db.mu.Lock()
	defer db.mu.Unlock()

	delete(db.accounts, accountID)
}

// Query returns a queried account from the database.
func (db *Database) Query(accountID AccountID) (Account, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	account, exists := db.accounts[accountID]
	if !exists {
		return Account{}, errors.New("account does not exist")
	}

	return account, nil
}

// Copy makes a copy of the current accounts in the database.
func (db *Database) Copy() map[AccountID]Account {
	db.mu.RLock()
	defer db.mu.RUnlock()

	accounts := make(map[AccountID]Account)
	for accountID, account := range db.accounts {
		accounts[accountID] = account
	}
	return accounts
}
