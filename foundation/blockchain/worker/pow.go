package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/qcbit/blockchain/foundation/blockchain/state"
)

// CORE NOTE: The POW mining operation is managed by this function which runs on its
// own goroutine. When a startMining signal is received (mainly because a wallet
// transaction was received) a block is created and then the POW operation starts.
// This operation can be canceled if a proposed block is received and is validated.

// powOperations handles mining.
func (w *Worker) powOperations() {
	w.evHandler("worker: powOperations: goroutine started")
	defer w.evHandler("worker: powOperations: goroutine completed")

	for {
		select {
		case <-w.startMining:
			if !w.isShutdown() {
				w.runPowOperation()
			}
		case <-w.shut:
			w.evHandler("worker: powOperations: shutdown signal received")
			return
		}
	}
}

// runPowOperation takes all the transactions from the mempool and writes a new block to the database.
func (w *Worker) runPowOperation() {
	w.evHandler("worker: runPowOperation: MINING: started")
	defer w.evHandler("worker: runPowOperation: MINING: completed")

	// Ensure transactions in the mempool.
	length := w.state.MempoolLength()
	if length == 0 {
		w.evHandler("worker: runPowOperation: MINING: no transactions to mine: TXs: %d", length)
		return
	}

	// After running a mining operation, check if a new operation should be signaled again.
	defer func() {
		lenght := w.state.MempoolLength()
		if lenght > 0 {
			w.evHandler("worker: runPowOperation: MINING: signal new mining operation: TXs: %d", length)
			w.SignalStartMining()
		}
	}()

	// Drain the cancel mining channel before starting.
	select {
	case <-w.cancelMining:
		w.evHandler("worker: runPowOperation: MINING: drained cancel channel")
	default:
	}

	// Create a context so mining can be canceled.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Can't return from this function until these G's are complete.
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
			w.evHandler("worker: runPowOperation: MINING: CANCEL: requested")
		case <-ctx.Done():
		}
	}()

	// This goroutine is the mining operation.
	go func() {
		defer func() {
			cancel()
			wg.Done()
		}()

		t := time.Now()
		_, err := w.state.MineNewBlock(ctx)
		duration := time.Since(t)

		w.evHandler("worker: runPowOperation: MINING: mining duration[%v]", duration)

		if err != nil {
			switch {
			case errors.Is(err, state.ErrNoTransactions):
				w.evHandler("worker: runPowOperation: MINING: no transactions to mine")
			case ctx.Err() != nil:
				w.evHandler("worker: runPowOperation: MINING: CANCEL: complete")
			default:
				w.evHandler("worker: runPowOperation: MINING: error: %s", err)
			}
			return
		}
		// BLOCK MINED
	}()
	// Wait for both goroutines to complete.
	wg.Wait()
}
