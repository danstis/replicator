package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/danstis/replicator/cmd/replicator/internal/dbot"
	"github.com/danstis/replicator/internal/version"
	"github.com/danstis/replicator/pkg/logging"
	"github.com/danstis/replicator/pkg/tracing"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

type app struct {
	bot            *dbot.Bot
	discordToken   string
	logger         *otelzap.SugaredLogger
	replicateToken string
	shutdown       func(ctx context.Context) error
}

// Main entry point for the app.
func main() {
	log.Printf("Replicator bot - v%q", version.Version)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// create the logger and initialise the tracing.
	a := &app{}
	if err := a.startup(); err != nil {
		log.Fatal(err)
	}
	defer a.shutdown(ctx) //nolint:errcheck
	defer a.logger.Sync() //nolint:errcheck

	ctx, span := otel.Tracer("").Start(ctx, "main")
	defer span.End()

	// var err error
	a.bot = &dbot.Bot{}
	a.bot.Connect(ctx, a.discordToken)
	defer a.bot.State.Close()
	a.logger.Ctx(ctx).Infow("replicator bot started", "version", version.Version)
	// a.asanaClient = initAsanaClient(ctx, asanaPAT)
	// if err != nil {
	// 	a.logger.Ctx(ctx).Fatalw("failed to connect to Azure DevOps",
	// 		"error", err,
	// 	)
	// }
	span.End()

	WaitForInterrupt()
}

// WaitForInterrupt blocks until a SIGINT, SIGTERM or another OS interrupt is received.
// "Pause until Ctrl+C", basically.
func WaitForInterrupt() {
	// Thanks to various Discord Gophers for this very simple stuff.
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, os.Interrupt)
	<-signalCh
}

func (a *app) startup() error {
	// Create tracer.
	var err error
	a.shutdown, err = tracing.InitTracer(version.Version)
	if err != nil {
		return err
	}

	// Init the logger, including sending logs to the tracer.
	a.logger, err = logging.InitLogger(zap.InfoLevel)
	if err != nil {
		return err
	}

	a.discordToken = os.Getenv("DISCORD_TOKEN")
	a.replicateToken = os.Getenv("REPLICATE_TOKEN")

	return nil
}
