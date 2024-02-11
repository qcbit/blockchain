// Package database handles all lower level support for maintaining the
// blockchain database.  This includes the management of an in-memory database
// of account information.
package database

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/qcbit/blockchain/foundation/blockchain/genesis"
	"github.com/qcbit/blockchain/foundation/blockchain/signature"
)

// Database manages data related to accounts who have transacted on the blockchain.
type Database struct {
	mu          sync.RWMutex
	genesis     genesis.Genesis
	latestBlock Block
	accounts    map[AccountID]Account
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

// UpdateLatestBlock provides safe access to update the latest block.
func (db *Database) UpdateLatestBlock(block Block) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.latestBlock = block
}

// LatestBlock returns the latest block.
func (db *Database) LatestBlock() Block {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.latestBlock
}

// HashState returns a hash based on the contents of the accounts and
// their balances. This is added to each block and checked by peers.
func (db *Database) HashState() string {
	accounts := make([]Account, 0, len(db.accounts))
	db.mu.RLock()
	{
		for _, account := range db.accounts {
			accounts = append(accounts, account)
		}
	}
	db.mu.RUnlock()

	sort.Sort(byAccount(accounts))
	return signature.Hash(accounts)
}

// ApplyMiningReward gives the specified account the mining reward.
func (db *Database) ApplyMiningReward(block Block) {
	db.mu.Lock()
	defer db.mu.Unlock()

	account := db.accounts[block.Header.BeneficiaryID]
	account.Balance += block.Header.MiningReward

	db.accounts[block.Header.BeneficiaryID] = account
}

// ApplyTransaction performs the business logic for applying a transaction to the database.
func (db *Database) ApplyTransaction(block Block, tx BlockTx) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Capture these accounts from the database.
	from, exists := db.accounts[tx.FromID]
	if !exists {
		from = newAccount(tx.FromID, 0)
	}

	to, exists := db.accounts[tx.ToID]
	if !exists {
		to = newAccount(tx.ToID, 0)
	}

	bnfc, exists := db.accounts[block.Header.BeneficiaryID]
	if !exists {
		bnfc = newAccount(block.Header.BeneficiaryID, 0)
	}

	// The account needs to pay the gas fee regardless. Take the
	// remaining balance if the account doesn't hold enough for the
	// full amount of gas. This is the only way to stop bad actors.
	gasFee := tx.GasPrice * tx.GasUnits
	if gasFee > from.Balance {
		gasFee = from.Balance
	}
	from.Balance -= gasFee
	bnfc.Balance += gasFee

	// Make sure these changes get applied.
	db.accounts[tx.FromID] = from
	db.accounts[block.Header.BeneficiaryID] = bnfc

	// Perform basic accounting checks.
	{
		if tx.Nonce != (from.Nonce + 1) {
			return fmt.Errorf("invalid transaction nonce: got %d, expected %d", tx.Nonce, from.Nonce+1)
		}

		if from.Balance == 0 || from.Balance < (tx.Value+tx.Tip) {
			return fmt.Errorf("invalid transaction, insufficient funds: balance %d, needed %d", from.Balance, (tx.Value + tx.Tip))
		}
	}

	// Update the balances between the two parties.
	from.Balance -= tx.Value
	to.Balance += tx.Value

	// Give the beneficiary the tip.
	from.Balance -= tx.Tip
	bnfc.Balance += tx.Tip

	// Update the nonce for the next transaction check.
	from.Nonce = tx.Nonce

	// Update the final changes to these accounts.
	db.accounts[tx.FromID] = from
	db.accounts[tx.ToID] = to
	db.accounts[block.Header.BeneficiaryID] = bnfc

	return nil
}
