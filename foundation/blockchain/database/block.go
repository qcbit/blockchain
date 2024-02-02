package database

import (
	"errors"

	"github.com/qcbit/blockchain/foundation/blockchain/merkle"
	"github.com/qcbit/blockchain/foundation/blockchain/signature"
)

// ErrChainForked is returned from validateNextBlock if another node's chain
// is two or more blocks ahead of ours.
var ErrChainForked = errors.New("blockchain forked, start resync")

//-----------------------------------------------------------------------------

// BlockData represents what can be serialized to disk and over the network.
type BlockData struct {
	Hash   string      `json:"hash"`
	Header BlockHeader `json:"block"`
	Trans  []BlockTx   `json:"trans"`
}

// NewBlockData creates a new block data.
func NewBlockData(block Block) BlockData {
	blockData := BlockData{
		Hash:   block.Hash(),
		Header: block.Header,
		Trans:  block.MerkleTree.Values(),
	}

	return blockData
}

// ToBlock converts a storage block into a database block.
func ToBlock(blockData BlockData) (Block, error) {
	tree, err := merkle.NewTree(blockData.Trans)
	if err != nil {
		return Block{}, err
	}

	block := Block{
		Header:     blockData.Header,
		MerkleTree: tree,
	}

	return block, nil
}

//-----------------------------------------------------------------------------

// BlockHeader represents common information required for each block.
type BlockHeader struct {
	Number        uint64    `json:"number"`          // Ethereum: Block number in the chain.
	PrevBlockHash string    `json:"prev_block_hash"` // Bitcoin: Hash of the previous block.
	TimeStamp     uint64    `json:"timestamp"`       // Bitcoin: Time the block was mined.
	BeneficiaryID AccountID `json:"beneficiary"`     // Ethereum: The account who is receiving fees and tips.
	Difficulty    uint16    `json:"difficulty"`      // Ethereum: The number of 0's needed to solve the hash solution.
	MiningReward  uint64    `json:"mining_reward"`   // Ethereum: The reward for mining this block.
	StateRoot     string    `json:"state_root"`      // Ethereum: Represents the hash of the accounts and their balances.
	TransRoot     string    `json:"trans_root"`      // Both: Represents the merkle root hash for the transactions.
	Nonce         uint64    `json:"nonce"`           // Both: Value identified to solve the hash solution.
}

// Block represents a group of transactions bundled together.
type Block struct {
	Header     BlockHeader
	MerkleTree *merkle.Tree[BlockTx]
}

// Hash returns the unique hash for the Block.
func (b Block) Hash() string {
	if b.Header.Number == 0 {
		return signature.ZeroHash
	}

	// CORE NOTE: Hashing the block header and not the whole block so the blockchain
	// can be cryptographically checked by only needing block headers and not full
	// blocks with the transaction data. This will support the ability to have pruned
	// nodes and light clients in the future.
	// - A pruned node stores all the block headers, but only a small number of full
	//   blocks (maybe the last 1000 blocks). This allows for full cryptographic
	//   validation of blocks and transactions without all the extra storage.
	// - A light client keeps block headers and just enough sufficient information
	//   to follow the latest set of blocks being produced. The do not validate
	//   blocks, but can prove a transaction is in a block.

	return signature.Hash(b.Header)
}
