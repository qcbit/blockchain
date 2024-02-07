package database

import (
	"crypto/ecdsa"
	"errors"
	"unicode"

	"github.com/ethereum/go-ethereum/crypto"
)

// Account represents an account on the blockchain.
type Account struct {
	AccountID AccountID
	Nonce     uint64
	Balance   uint64
}

// newAccount creates a new account with the given account ID and balance.
func newAccount(accountID AccountID, balance uint64) Account {
	return Account{
		AccountID: accountID,
		Balance:   balance,
	}
}

// ---------------------------------------------------------------------------

// AccountID represents an account ID that is used to sign transactions.
// It is associated with transactions on the blockchain. This is the last
// 20 bytes of the hash of the public key.
type AccountID string

// ToAccountID converts a hex-encoded string to an account ID and validates
// the string is formatted correctly.
func ToAccountID(hex string) (AccountID, error) {
	a := AccountID(hex)
	if !a.IsAccountID() {
		return "", errors.New("invalid format")
	}

	return a, nil
}

// PublicKeyToAccountID converts a public key to an account ID.
func PublicKeyToAccountID(publicKey ecdsa.PublicKey) AccountID {
	return AccountID(crypto.PubkeyToAddress(publicKey).String())
}

// IsAccountID returns true if the account ID is valid.
func (a AccountID) IsAccountID() bool {
	const addressLength = 20

	if has0xPrefix(a) {
		a = a[2:]
	}

	return len(a) == 2*addressLength && isHex(a)
}

// has0xPrefix returns true if the string has a 0x prefix.
func has0xPrefix(a AccountID) bool {
	return len(a) >= 2 && a[0] == '0' && unicode.ToLower(rune(a[1])) == 'x'
}

// isHex returns true if the string is a hex-encoded string.
func isHex(a AccountID) bool {
	if len(a)%2 != 0 {
		return false
	}

	for _, c := range []byte(a) {
		if !isHexChar(c) {
			return false
		}
	}

	return true
}

// isHexChar returns true if the byte is a hex character.
func isHexChar(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

//---------------------------------------------------------------------------

// byAccount provides sorting support by the account id value.
type byAccount []Account

// Len returns the number of transactions in the list.
func (ba byAccount) Len() int {
	return len(ba)
}

// Less helps sort the list by account id in ascending order
// to keep the accounts in a consistent order.
func (ba byAccount) Less(i, j int) bool {
	return ba[i].AccountID < ba[j].AccountID
}

// Swap moves accounts in the order of the account id value.
func (ba byAccount) Swap(i, j int) {
	ba[i], ba[j] = ba[j], ba[i]
}
