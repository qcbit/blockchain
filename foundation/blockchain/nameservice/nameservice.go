// Package nameservice reads the zblock accounts and provides a name service lookup for them.
package nameservice

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/qcbit/blockchain/foundation/blockchain/database"
)

// NameService maintains a map of accounts for name lookup.
type NameService struct {
	accounts map[database.AccountID]string
}

// New constructs a new NameService.
func New(root string) (*NameService, error) {
	ns := NameService{
		accounts: make(map[database.AccountID]string),
	}

	fn := func(filename string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walkdir failure: %w", err)
		}

		if path.Ext(filename) != ".ecdsa" {
			return nil
		}

		privateKey, err := crypto.LoadECDSA(filename)
		if err != nil {
			return fmt.Errorf("load private key failure: %w", err)
		}

		accountID := database.PublicKeyToAccountID(privateKey.PublicKey)
		ns.accounts[accountID] = strings.TrimSuffix(path.Base(filename), ".ecdsa")

		return nil
	}

	if err := filepath.Walk(root, fn); err != nil {
		return nil, fmt.Errorf("walkdir failure: %w", err)
	}

	return &ns, nil
}

// Lookup returns the account name for the given account ID.
func (ns *NameService) Lookup(accountID database.AccountID) string {
	name, exists := ns.accounts[accountID]
	if !exists {
		return string(accountID)
	}
	return name
}

// Copy returns a copy of the NameService.
func (ns *NameService) Copy() map[database.AccountID]string {
	accounts := make(map[database.AccountID]string, len(ns.accounts))
	for account, name := range ns.accounts {
		accounts[account] = name
	}
	return accounts
}
