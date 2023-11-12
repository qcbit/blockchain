// Package public maintains the public handlers for the node service.
package public

import (
	"context"
	"net/http"

	"github.com/qcbit/blockchain/foundation/web"
	"go.uber.org/zap"
)

// Handlers manages the set of bar ledger endpoints.
type Handlers struct {
	Log *zap.SugaredLogger
}

// Sample just provides a starting point for the class.
func (h *Handlers) Sample(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	resp := struct {
		Status string
	}{
		Status: "ok",
	}

	return web.Respond(ctx, w, resp, http.StatusOK)
}