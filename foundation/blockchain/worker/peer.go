package worker

import "github.com/qcbit/blockchain/foundation/blockchain/peer"

// CORE NOTE: The p2p network is managed by this goroutine. There is
// a single node that is considered the origin node. The defaults in
// main.go represent the origin node. That node must be running first.
// All new peer nodes connect to the origin node to identify all other
// peers on the network. The topology is all nodes having a connection
// to all other nodes. If a node does not respond to a network call,
// they are removed from the peer list until the next peer operation.

// peerOperations handles finding new peers.
func (w *Worker) peerOperations() {
	w.evHandler("worker: peerOperations: Goroutine started")
	defer w.evHandler("worker: peerOperations: Goroutine completed")

	// On startup talk to the origin node and get an updated
	// peers list. Then share with the network that this node
	// is available for transaction and block submissions.
	// Maybe handled by sync; therefore, duplication.
	// w.runPeersOperation()

	for {
		select {
		case <-w.ticker.C:
			if !w.isShutdown() {
				w.runPeersOperation()
			}
		case <-w.shut:
			w.evHandler("worker: peerOperations: received shutdown signal")
			return
		}
	}
}

// runPeersOperation updates the peer list.
func (w *Worker) runPeersOperation() {
	w.evHandler("worker: runPeersOperation: started")
	defer w.evHandler("worker: runPeersOperation: completed")

	for _, peer := range w.state.KnownExternalPeers() {
		// Retrieve the status of the peer.
		status, err := w.state.NetRequestPeerStatus(peer)
		if err != nil {
			w.evHandler("worker: runPeersOperation: NetRequestPeerStatus: %s: ERROR: %s", peer.Host, err)

			// Since this peer is unavailable, remove it form the list.
			w.state.RemoveKnownPeer(peer)

			continue
		}

		// Add missing peers form this node's peer list.
		w.addNewPeers(status.KnownPeers)
	}

	// Share with peers this node is available to participate in the network.
	w.state.NetSendNodeAvailableToPeers()
}

// addNewPeers takes the list of known peers and makes sure they are
// included in the nodes list of known peers.
func (w *Worker) addNewPeers(peers []peer.Peer) error {
	w.evHandler("worker: runPeerUpdatesOperation: addNewPeers: started")
	defer w.evHandler("worker: runPeerUpdatesOperation: addNewPeers: completed")

	for _, peer := range peers {
		// Don't add this running node to the peer list.
		if peer.Match(w.state.Host()) {
			continue
		}

		// Only log when the peer is new.
		if w.state.AddKnownPeer(peer) {
			w.evHandler("worker: runPeerUpdatesOperation: addNewPeers: add peer nodes: adding peer node %s", peer.Host)
		}
	}

	return nil
}
