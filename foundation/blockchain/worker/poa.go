package worker

import (
	"context"
	"errors"
	"hash/fnv"
	"sort"
	"sync"
	"time"

	"github.com/qcbit/blockchain/foundation/blockchain/state"
)

// CORE NOTE: The POA mining operation is managed by this function which runs on
// its own goroutine. The node starts a loop that is on a 12 second timer. At
// the beginning of each cycle the selection algorithm is executed which determines
// if this node needs to mine the next block. If this node is not selected, it
// waits for the next cycle to check the selection algorithm again.

// cycleDuration sets the mining operation to happen every 5 seconds
const secondsPerCycle = 5
const cycleDuration = secondsPerCycle * time.Second

// poaOperations handles mining.
func (w *Worker) poaOperations() {
	w.evHandler("worker: poaOperations: Goroutine started")
	defer w.evHandler("worker: poaOperations: Goroutine completed")

	ticker := time.NewTicker(cycleDuration)

	// Start on a secondsPerCycle mark: e.g. MM.00, MM.05, MM.10, MM.15, etc.
	resetTicker(ticker, secondsPerCycle*time.Second)

	for {
		select {
		case <-ticker.C:
			if !w.isShutdown() {
				w.runPoaOperation()
			}
		case <-w.shut:
			w.evHandler("worker: poaOperations: shutdown signal received")
			return
		}

		// Reset the ticker for the next cycle.
		resetTicker(ticker, 0)
	}
}

// runPoaOperation takes all the transactions from the
//
//	mempool and writes a new block to the database.
func (w *Worker) runPoaOperation() {
	w.evHandler("worker: runPoaOperation: started")
	defer w.evHandler("worker: runPoaOperation: completed")

	// Run the selection algorithm.
	peer := w.selection()
	w.evHandler("worker: runPoaOperation: SELECTED: %s", peer)

	// If not selected, return and wait for the new block.
	if peer != w.state.Host() {
		return
	}

	// Ensure transactions are in the mempool.
	length := w.state.MempoolLength()
	if length == 0 {
		w.evHandler("worker: runPoaOperation: MINING: no transactions to mine.")
		return
	}

	// Drain the cancel mining channel before starting.
	select {
	case <-w.cancelMining:
		w.evHandler("worker: runPoaOperation: MINING: drained cancel mining channel")
	default:
	}

	// Create a context so mining can be canceled.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Can't return from this function until these goroutines are done.
	var wg sync.WaitGroup
	wg.Add(2)

	// This goroutine exists to cancel the mining operation.
	go func() {
		defer func() {
			cancel()
			wg.Done()
		}()

		select {
		case <-w.cancelMining:
			w.evHandler("worker: runPoaOperation: MINING: CANCEL: requested")
		case <-ctx.Done():
		}
	}()

	// This goroutine exists to mine the block.
	go func() {
		defer func() {
			cancel()
			wg.Done()
		}()

		t := time.Now()
		block, err := w.state.MineNewBlock(ctx)
		duration := time.Since(t)

		w.evHandler("worker: runPoaOperation: MINING: duration: %v", duration)

		if err != nil {
			switch {
			case errors.Is(err, state.ErrNoTransactions):
				w.evHandler("worker: runPoaOperation: MINING: WARNING: no transactions to mine")
			case ctx.Err() != nil:
				w.evHandler("worker: runPoaOperation: MINING: CANCEL: complete")
			default:
				w.evHandler("worker: runPoaOperation: MINING: ERROR: %v", err)
			}
			return
		}

		// The block is mined. Propose it to the network.
		if err := w.state.NetSendBlockToPeers(block); err != nil {
			w.evHandler("worker: runPoaOperation: MINING: proposeBlockToPeers: WARNING: %v", err)
		}
	}()

	// Wait for goroutines to complete.
	wg.Wait()
}

// selection selects a peer to mine the next block.
func (w *Worker) selection() string {
	// Retrieve the known peers list which includes this node.
	peers := w.state.KnownPeers()

	// Log info
	w.evHandler("worker: selection: Host %s, known peers: %v", w.state.Host(), peers)

	// Sort the current list of peers by host.
	names := make([]string, len(peers))
	for i, peer := range peers {
		names[i] = peer.Host
	}
	sort.Strings(names)

	// Based on the latest block, pick an index number from the registry.
	h := fnv.New32a()
	h.Write([]byte(w.state.LatestBlock().Hash()))
	integerHash := h.Sum32()
	i := integerHash % uint32(len(names))

	// Return the name of the node selected.
	return names[i]
}

// resetTicker ensures the next tick occurs on the described cadence.
func resetTicker(ticker *time.Ticker, waitOnSecond time.Duration) {
	nextTick := time.Now().Add(cycleDuration).Round(waitOnSecond)
	diff := time.Until(nextTick)
	ticker.Reset(diff)
}
