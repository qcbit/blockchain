package database

import (
	"context"
	"crypto/rand"
	"errors"
	"math"
	"math/big"
	"time"

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

// POWArgs represents the arguments required to solve the proof of work.
type POWArgs struct {
	BeneficiaryID AccountID
	Difficulty    uint16
	MiningReward  uint64
	PrevBlock     Block
	StateRoot     string
	Trans         []BlockTx
	EvHandler     func(v string, args ...any)
}

// POW constructs a new Block and performs the work to find a nonce
// that solves the cryptographic hash puzzle.
func POW(ctx context.Context, args POWArgs) (Block, error) {

	// When mining the first block, the previous block's hash will be zero.
	prevBlockHash := signature.ZeroHash
	if args.PrevBlock.Header.Number > 0 {
		prevBlockHash = args.PrevBlock.Hash()
	}

	// Construct a merkle tree from the transaction for this block.
	// The root of this tree will be part of the block to be mined.
	tree, err := merkle.NewTree(args.Trans)
	if err != nil {
		return Block{}, err
	}

	// Construct the block to be mined.
	block := Block{
		Header: BlockHeader{
			Number:        args.PrevBlock.Header.Number + 1,
			PrevBlockHash: prevBlockHash,
			TimeStamp:     uint64(time.Now().UTC().UnixMilli()),
			BeneficiaryID: args.BeneficiaryID,
			Difficulty:    args.Difficulty,
			MiningReward:  args.MiningReward,
			StateRoot:     args.StateRoot,
			TransRoot:     tree.RootHex(),
			Nonce:         0, // Will be identified by the POW algorithm.
		},
		MerkleTree: tree,
	}

	// Perform the POW algorithm to find the nonce that solves the hash puzzle.
	if err := block.performPOW(ctx, args.EvHandler); err != nil {
		return Block{}, err
	}

	return block, nil
}

// performPOW solves the proof of work algorithm to find the nonce that
// solves the cryptographic hash puzzle. Pointer semantics are used since a
// nonce is being identified and the block is being updated.
func (b *Block) performPOW(ctx context.Context, ev func(v string, args ...any)) error {
	ev("database: PerformPOW: MINING: started")
	defer ev("database: PerformPOW: MINING: completed")

	// Log the transactions that are part of this potential block.
	for _, tx := range b.MerkleTree.Values() {
		ev("database: PerformPOW: MINING: transaction: %s", tx)
	}

	// Choose a random starting point for the nonce. After this, the nonce
	// will be incremented by 1 until a solution is found by us or another node.
	nBig, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return ctx.Err()
	}
	b.Header.Nonce = nBig.Uint64()

	ev("viewer: PerformPOW: MINING: running")

	// Loop until a solution is found for the next block.
	var attempts uint64
	for {
		attempts++
		if attempts%1_000_000 == 0 {
			ev("viewer: PerformPOW: MINING: running: attempts: %d", attempts)
		}

		// Did we timeout trying to solve the puzzle?
		if ctx.Err() != nil {
			ev("database: PerformPOW: MINING: CANCELLED")
			return ctx.Err()
		}

		// Hash the block and check if we have solved the puzzle.
		hash := b.Hash()
		if !isHashSolved(b.Header.Difficulty, hash) {
			b.Header.Nonce++
			continue
		}

		ev("database: PerformPOW: MINING: SOLVED: prevBlk[%s]: newBlk[%s]", b.Header.PrevBlockHash, hash)
		ev("database: PerformPOW: MINING: attempts: %d", attempts)

		return nil
	}
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

// isHashSolved checks the hash to make sure it complies with
// the POW rules. We need to match a difficulty number of 0's.
func isHashSolved(difficulty uint16, hash string) bool {
	const match = "0x00000000000000000"

	if len(hash) != 66 {
		return false
	}

	difficulty += 2
	return hash[:difficulty] == match[:difficulty]
}
