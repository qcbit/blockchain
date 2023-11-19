package main

import (
	"errors"
	"expvar"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/ardanlabs/conf/v3"
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

	// ----------------------------------------------------------------
	// Configuration
	// ----------------------------------------------------------------
	cfg := struct {
		conf.Version
		Web struct {
			ReadTimeout     string `conf:"default:5s"`
			WriteTimeout    string `conf:"default:10s"`
			IdleTimeout     string `conf:"default:120s"`
			ShutdownTimeout string `conf:"default:20s,mask"`
			APIHost         string `conf:"default:0.0.0.0:8080"`
			DebugHost       string `conf:"default:0.0.0.0:9080"`
		}
	}{
		Version: conf.Version{
			Build: build,
			Desc:  "© 2023 WTFPL – Do What the Fuck You Want to Public License.",
		},
	}

	const prefix = "QCHAIN"
	help, err := conf.Parse(prefix, &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			return nil
		}
		return fmt.Errorf("parsing config: %w", err)
	}

	// ----------------------------------------------------------------
	// App Starting
	// ----------------------------------------------------------------

	log.Infow("starting service", "version", build)
	defer log.Infow("shutdown complete")

	out, err := conf.String(&cfg)
	if err != nil {
		return fmt.Errorf("generating config for output: %w", err)
	}
	log.Infow("startup", "config", out)

	expvar.NewString("build").Set(build)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown
	log.Infow("shutdown", "status", "shutdown signal")
	defer log.Infow("shutdown", "status", "complete", "signal")

	return nil
}
