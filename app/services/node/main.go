package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/qcbit/blockchain/foundation/logger"
	"go.uber.org/zap"
)

var build = "dev"

func main() {
	log, err := logger.New("QChain")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer log.Sync()

	if err := run(log); err != nil {
		fmt.Println(err)
	}
}

func run(log *zap.SugaredLogger) error {

	// ----------------------------------------------------------------
	log.Infow("startup", "GOMAXPROCS", runtime.GOMAXPROCS(0), "BUILD", build)
	// ----------------------------------------------------------------
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown
	log.Infow("shutdown", "status", "shutdown signal")
	defer log.Infow("shutdown", "status", "complete", "signal")

	return nil
}
