// Package worker implements implements mining, peer updates, and transactions sharing for the blockchain.
package worker

import (
	"sync"

	"github.com/qcbit/blockchain/foundation/blockchain/database"
	"github.com/qcbit/blockchain/foundation/blockchain/state"
)

// Worker manages the POW workflows for the blockchain.

// Add this line

type Worker struct {
	state        *state.State
	wg           sync.WaitGroup
	shut         chan struct{}
	startMining  chan bool
	cancelMining chan bool
	txSharing    chan database.BlockTx
	evHandler    state.EventHandler
}

// Run creates a worker, registers the worker with the state,
// and starts all the background processes.
func Run(state *state.State, evHandler state.EventHandler) {
	w := Worker{
		state:        state,
		shut:         make(chan struct{}),
		startMining:  make(chan bool, 1),
		cancelMining: make(chan bool, 1),
		txSharing:    make(chan database.BlockTx, maxTxShareRequests),
		evHandler:    evHandler,
	}

	// Register the worker with the state.
	state.Worker = &w

	// Update this node before starting any support goroutines.
	w.Sync()

	// Load the set of operations to run.
	operations := []func(){
		w.shareTxOperations,
		w.powOperations,
	}

	// Set the wait group to match the number of goroutines needed for the set of operations.
	g := len(operations)
	w.wg.Add(g)

	// We don't want to return until all the G's are up and running.
	hasStarted := make(chan bool)

	// Start the operations.
	for _, op := range operations {
		go func(op func()) {
			defer w.wg.Done()
			hasStarted <- true
			op()
		}(op)
	}

	// Wait for all the operations to start.
	for i := 0; i < g; i++ {
		<-hasStarted
	}
}

//------------------------------------------------------------------------------
// These methods implement the state.Worker interface.

// Shutdown terminates the goroutine performing work.
func (w *Worker) Shutdown() {
	w.evHandler("worker: shutdown: started")
	defer w.evHandler("worker: shutdown: completed")

	w.evHandler("worker: shutdown: signal cancel mining")
	w.SignalCancelMining()

	w.evHandler("worker: shutdown: terminate goroutine")
	close(w.shut)
	w.wg.Wait()
}

// SignalStartMining starts a mining operation. If there is already a signal
// pending in the channel, return since a mining operation will start.
func (w *Worker) SignalStartMining() {
	// if !w.state.IsMiningAllowed() {
	// 	w.evHandler("state: MinePeerBlock: accepting blocks turned off")
	// 	return
	// }

	// if w.state.Consensus() != state.ConsensusPOW {
	// 	return
	// }

	select {
	case w.startMining <- true:
	default:
	}
	w.evHandler("worker: SignalStartMining: mining signaled")
}

// SignalCancelMining signals the goroutine executing the runMiningOperation() to stop immediately.
func (w *Worker) SignalCancelMining() {

	// Only POW requires signaling to cancel mining.
	// if w.state.Consensus() != state.ConsensusPOW {
	// 	return
	// }

	select {
	case w.cancelMining <- true:
	default:
	}
	w.evHandler("worker: SignalCancelMining: CANCEL: signaled")
}

// SignalShareTx signals a share transaction operation. If maxTxShareRequests
// signals exists in the channel, we won't send these.
func (w *Worker) SignalShareTx(blockTx database.BlockTx) {
	select {
	case w.txSharing <- blockTx:
		w.evHandler("worker: SignalShareTx: share Tx signaled")
	default:
		w.evHandler("worker: SignalShareTx: queue full, transaction dropped and transactions won't be shared.")
	}
}

// ------------------------------------------------------------------------------
// isShutdown is used to test if a shutdown has been signaled.
func (w *Worker) isShutdown() bool {
	select {
	case <-w.shut:
		return true
	default:
		return false
	}
}
