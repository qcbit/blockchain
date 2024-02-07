package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/common-nighthawk/go-figure"
	"github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/zap"

	"github.com/qcbit/blockchain/app/services/node/handlers"
	"github.com/qcbit/blockchain/foundation/blockchain/database"
	"github.com/qcbit/blockchain/foundation/blockchain/genesis"
	"github.com/qcbit/blockchain/foundation/blockchain/nameservice"
	"github.com/qcbit/blockchain/foundation/blockchain/state"
	"github.com/qcbit/blockchain/foundation/blockchain/worker"
	"github.com/qcbit/blockchain/foundation/logger"
)

// build is the git version of this program. It is set using build flags in the makefile.
var build = "develop"

func main() {

	// Construct the application logger.
	log, err := logger.New("NODE")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer log.Sync()

	// Perform the startup and shutdown sequence.
	if err := run(log); err != nil {
		log.Errorw("startup", "ERROR", err)
		log.Sync()
		os.Exit(1)
	}
}

func run(log *zap.SugaredLogger) error {

	// =========================================================================
	// Configuration

	// This is all the configuration for the application and the default values.
	// Configuration values will be passed through the application as individual
	// values.
	cfg := struct {
		conf.Version
		Web struct {
			ReadTimeout     time.Duration `conf:"default:5s"`
			WriteTimeout    time.Duration `conf:"default:10s"`
			IdleTimeout     time.Duration `conf:"default:120s"`
			ShutdownTimeout time.Duration `conf:"default:20s"`
			DebugHost       string        `conf:"default:0.0.0.0:7080"`
			PublicHost      string        `conf:"default:0.0.0.0:8080"`
			PrivateHost     string        `conf:"default:0.0.0.0:9080"`
		}
		State struct {
			Beneficiary    string `conf:"default:miner1"`
			SelectStrategy string `conf:"default:Tip"`
		}
		NameService struct {
			Folder string `conf:"default:zblock/accounts/"`
		}
	}{
		Version: conf.Version{
			Build: build,
			Desc:  "© 2023 WTFPL",
		},
	}

	// Parse will set the defaults and then look for any overriding values
	// in environment variables and command line flags.
	const prefix = "NODE"
	help, err := conf.Parse(prefix, &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			return nil
		}
		return fmt.Errorf("parsing config: %w", err)
	}

	// =========================================================================
	// App Starting

	qQhainArt := figure.NewFigure("QChain", "", true)
	qQhainArt.Print()

	log.Infow("starting service", "version", build)
	defer log.Infow("shutdown complete")

	// Display the current configuration to the logs.
	out, err := conf.String(&cfg)
	if err != nil {
		return fmt.Errorf("generating config for output: %w", err)
	}
	log.Infow("startup", "config", out)

	// ----------------------------------------------------------------
	// NameService Support
	// ----------------------------------------------------------------

	// The NameService package provides name resolution for the account addresses.
	// The names come from the file names in the zblock/accounts folder.
	ns, err := nameservice.New(cfg.NameService.Folder)
	if err != nil {
		return fmt.Errorf("unable to create name service: %w", err)
	}

	// Logging the accounts and their names.
	for account, name := range ns.Copy() {
		log.Infow("startup", "status", "nameservice", "name", name, "account", account)
	}

	// ----------------------------------------------------------------
	// Blockchain Support
	// ----------------------------------------------------------------

	files, err := os.ReadDir(cfg.NameService.Folder)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		log.Infow(f.Name())
	}

	// Need to load the private key file for the configured beneficiary
	// so the account can get credited with fees and tips.
	path := fmt.Sprintf("%s%s.ecdsa", cfg.NameService.Folder, cfg.State.Beneficiary)
	privateKey, err := crypto.LoadECDSA(path)
	if err != nil {
		return fmt.Errorf("unable to load private key for node: %w", err)
	}

	ev := func(v string, args ...any) {
		s := fmt.Sprintf(v, args...)
		log.Infow(s, "traceid", "00000000-0000-0000-0000-000000000000")
	}

	// Load the genesis file.
	genesis, err := genesis.Load()
	if err != nil {
		return err
	}

	// The state value represents the blockchain node and manages the blockchain database
	// and provides the API for the application support.
	state, err := state.New(state.Config{
		BeneficiaryID:  database.PublicKeyToAccountID(privateKey.PublicKey),
		Genesis:        genesis,
		SelectStrategy: cfg.State.SelectStrategy,
		EvHandler:      ev,
	})
	if err != nil {
		return err
	}
	defer state.Shutdown()

	// The worker package implements the different workflows such as mining, transaction
	// peer sharing, and peer updates. The worker will register itself with the state.
	worker.Run(state, ev)

	// =========================================================================
	// Start Debug Service

	log.Infow("startup", "status", "debug v1 router started", "host", cfg.Web.DebugHost)

	// The Debug function returns a mux to listen and serve on for all the debug
	// related endpoints. This includes the standard library endpoints.

	// Construct the mux for the debug calls.
	debugMux := handlers.DebugMux(build, log)

	// Start the service listening for debug requests.
	// Not concerned with shutting this down with load shedding.
	go func() {
		if err := http.ListenAndServe(cfg.Web.DebugHost, debugMux); err != nil {
			log.Errorw("shutdown", "status", "debug v1 router closed", "host", cfg.Web.DebugHost, "ERROR", err)
		}
	}()

	// =========================================================================
	// Service Start/Stop Support

	// Make a channel to listen for an interrupt or terminate signal from the OS.
	// Use a buffered channel because the signal package requires it.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Make a channel to listen for errors coming from the listener. Use a
	// buffered channel so the goroutine can exit if we don't collect this error.
	serverErrors := make(chan error, 1)

	// =========================================================================
	// Start Public Service

	log.Infow("startup", "status", "initializing V1 public API support")

	// Construct the mux for the public API calls.
	publicMux := handlers.PublicMux(handlers.MuxConfig{
		Shutdown: shutdown,
		Log:      log,
		State:    state,
		NS:       ns,
	})

	// Construct a server to service the requests against the mux.
	public := http.Server{
		Addr:         cfg.Web.PublicHost,
		Handler:      publicMux,
		ReadTimeout:  cfg.Web.ReadTimeout,
		WriteTimeout: cfg.Web.WriteTimeout,
		IdleTimeout:  cfg.Web.IdleTimeout,
		ErrorLog:     zap.NewStdLog(log.Desugar()),
	}

	// Start the service listening for api requests.
	go func() {
		log.Infow("startup", "status", "public api router started", "host", public.Addr)
		serverErrors <- public.ListenAndServe()
	}()

	// =========================================================================
	// Start Private Service

	log.Infow("startup", "status", "initializing V1 private API support")

	// Construct the mux for the private API calls.
	privateMux := handlers.PrivateMux(handlers.MuxConfig{
		Shutdown: shutdown,
		Log:      log,
		State:    state,
	})

	// Construct a server to service the requests against the mux.
	private := http.Server{
		Addr:         cfg.Web.PrivateHost,
		Handler:      privateMux,
		ReadTimeout:  cfg.Web.ReadTimeout,
		WriteTimeout: cfg.Web.WriteTimeout,
		IdleTimeout:  cfg.Web.IdleTimeout,
		ErrorLog:     zap.NewStdLog(log.Desugar()),
	}

	// Start the service listening for api requests.
	go func() {
		log.Infow("startup", "status", "private api router started", "host", private.Addr)
		serverErrors <- private.ListenAndServe()
	}()

	// =========================================================================
	// Shutdown

	// Blocking main and waiting for shutdown.
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		log.Infow("shutdown", "status", "shutdown started", "signal", sig)
		defer log.Infow("shutdown", "status", "shutdown complete", "signal", sig)

		// Give outstanding requests a deadline for completion.
		ctx, cancelPub := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancelPub()

		// Asking listener to shut down and shed load.
		log.Infow("shutdown", "status", "shutdown private API started")
		if err := private.Shutdown(ctx); err != nil {
			private.Close()
			return fmt.Errorf("could not stop private service gracefully: %w", err)
		}

		// Give outstanding requests a deadline for completion.
		ctx, cancelPri := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancelPri()

		// Asking listener to shut down and shed load.
		log.Infow("shutdown", "status", "shutdown public API started")
		if err := public.Shutdown(ctx); err != nil {
			public.Close()
			return fmt.Errorf("could not stop public service gracefully: %w", err)
		}
	}

	return nil
}
