package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/oleg3190/Web-studio-img/backend/internal/config"
	"github.com/oleg3190/Web-studio-img/backend/internal/composition"
	"github.com/oleg3190/Web-studio-img/backend/internal/library"
	"github.com/oleg3190/Web-studio-img/backend/internal/assets"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/generation"
	"github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
	"github.com/oleg3190/Web-studio-img/backend/internal/httpapi"
	"github.com/oleg3190/Web-studio-img/backend/internal/iterations"
	"github.com/oleg3190/Web-studio-img/backend/internal/projects"
	"github.com/oleg3190/Web-studio-img/backend/internal/prompts"
	"github.com/oleg3190/Web-studio-img/backend/internal/providers/yandexart"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
	"github.com/oleg3190/Web-studio-img/backend/internal/queue"
	"github.com/oleg3190/Web-studio-img/backend/internal/references"
	"github.com/oleg3190/Web-studio-img/backend/internal/similarity"
	"github.com/oleg3190/Web-studio-img/backend/internal/exports"
	"github.com/oleg3190/Web-studio-img/backend/internal/storage"
	"github.com/oleg3190/Web-studio-img/backend/internal/security"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil { logger.Error("configuration error", "error", err); os.Exit(1) }

	limiter := security.NewRateLimiter(cfg.RateLimit, cfg.RateWindow)
	var projectHandler *projects.Handler
	var iterationHandler *iterations.Handler
	var generationHandler *generation.Handler
	var eventHandler *events.Handler
	var promptHandler *prompts.Handler
	var referenceHandler *references.Handler
	var actionHandler *humanactions.Handler
	var provenanceHandler *provenance.Handler
	var assetHandler *assets.UploadHandler
	var similarityHandler *similarity.Handler
	var exportHandler *exports.Handler
	var compositionHandler *composition.Handler
	var libraryHandler *library.Handler
	var projectDB *sql.DB

	if cfg.DatabaseURL != "" {
		projectDB, err = sql.Open("pgx", cfg.DatabaseURL)
		if err != nil { logger.Error("database open failed", "error", err); os.Exit(1) }
		defer projectDB.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = projectDB.PingContext(ctx)
		cancel()
		if err != nil { logger.Error("database ping failed", "error", err); os.Exit(1) }

		projectStore, err := projects.NewSQLStore(projectDB)
		if err != nil { logger.Error("project store initialization failed", "error", err); os.Exit(1) }
		iterationStore, err := iterations.NewSQLStore(projectDB)
		if err != nil { logger.Error("iteration store initialization failed", "error", err); os.Exit(1) }
		eventStore, err := events.NewStore(projectDB)
		if err != nil { logger.Error("event store initialization failed", "error", err); os.Exit(1) }
		provenanceStore, err := provenance.NewStore(projectDB)
		if err != nil { logger.Error("provenance store initialization failed", "error", err); os.Exit(1) }
		actionStore, err := humanactions.NewStore(projectDB)
		if err != nil { logger.Error("human actions store initialization failed", "error", err); os.Exit(1) }
		promptStore, err := prompts.NewStore(projectDB)
		if err != nil { logger.Error("prompt store initialization failed", "error", err); os.Exit(1) }
		referenceStore, err := references.NewStore(projectDB)
		if err != nil { logger.Error("reference store initialization failed", "error", err); os.Exit(1) }

		projectHandler, err = projects.NewHandlerWithEventsAndProvenance(projectStore, eventStore, provenanceStore)
		if err != nil { logger.Error("project handler initialization failed", "error", err); os.Exit(1) }
		iterationHandler, err = iterations.NewHandlerWithEventsAndProvenance(iterationStore, eventStore, provenanceStore)
		if err != nil { logger.Error("iteration handler initialization failed", "error", err); os.Exit(1) }
		eventHandler, err = events.NewHandler(eventStore)
		if err != nil { logger.Error("event handler initialization failed", "error", err); os.Exit(1) }
		promptHandler, err = prompts.NewHandler(promptStore, eventStore, provenanceStore, actionStore)
		if err != nil { logger.Error("prompt handler initialization failed", "error", err); os.Exit(1) }
		referenceHandler, err = references.NewHandler(referenceStore, eventStore, provenanceStore, actionStore)
		if err != nil { logger.Error("reference handler initialization failed", "error", err); os.Exit(1) }
		actionHandler, err = humanactions.NewHandler(actionStore, eventStore, provenanceStore)
		if err != nil { logger.Error("human action handler initialization failed", "error", err); os.Exit(1) }
		provenanceHandler, err = provenance.NewHandler(provenanceStore)
		if err != nil {
			logger.Error("provenance handler initialization failed", "error", err)
			os.Exit(1)
		}
		assetStore, assetErr := assets.NewStore(projectDB)
		if assetErr != nil {
			logger.Error("asset store initialization failed", "error", assetErr)
			os.Exit(1)
		}
		var objectStorage storage.StorageProvider
		if cfg.S3Bucket != "" && cfg.S3AccessKey != "" && cfg.S3SecretKey != "" {
			objectStorage, assetErr = storage.NewS3Storage(context.Background(), storage.S3Config{
				Endpoint: cfg.S3Endpoint,
				Region: cfg.S3Region,
				Bucket: cfg.S3Bucket,
				AccessKey: cfg.S3AccessKey,
				SecretKey: cfg.S3SecretKey,
				UsePathStyle: cfg.S3UsePathStyle,
			})
			if assetErr != nil {
				logger.Error("object storage initialization failed", "error", assetErr)
				os.Exit(1)
			}
		}
		scanner := assets.SecurityScanner(assets.ImageSecurityScanner{})
		if cfg.ClamAVAddress != "" {
			scanner = assets.CompositeScanner{
				assets.ImageSecurityScanner{},
				assets.ClamAVScanner{Address: cfg.ClamAVAddress, Timeout: 10 * time.Second},
			}
		}
		assetProcessor := &assets.Processor{Storage: objectStorage, Store: assetStore, Scanner: scanner}
		assetHandler, err = assets.NewUploadHandler(assetStore, assetProcessor, objectStorage, eventStore, provenanceStore)
		if err != nil {
			logger.Error("asset handler initialization failed", "error", err)
			os.Exit(1)
		}
		similarityHandler, err = similarity.NewHandlerWithDB(projectDB, referenceStore, assetStore, objectStorage, eventStore, provenanceStore)
		if err != nil {
			logger.Error("similarity handler initialization failed", "error", err)
			os.Exit(1)
		}
		exportHandler, err = exports.NewHandlerWithDB(projectDB, assetStore, objectStorage, eventStore, provenanceStore, actionStore)
		if err != nil {
			logger.Error("export handler initialization failed", "error", err)
			os.Exit(1)
		}

		compositionHandler, err = composition.NewHandler(projectDB, eventStore, provenanceStore, actionStore)
		if err != nil {
			logger.Error("composition handler initialization failed", "error", err)
			os.Exit(1)
		}
		libraryHandler, err = library.NewHandler(projectDB)
		if err != nil {
			logger.Error("library handler initialization failed", "error", err)
			os.Exit(1)
		}

		if cfg.RedisURL != "" && cfg.YandexARTAPIKey != "" && cfg.YandexARTFolderID != "" {
			redisCfg, redisErr := queue.ParseRedisURL(cfg.RedisURL)
			if redisErr != nil { logger.Error("redis configuration failed", "error", redisErr); os.Exit(1) }
			queueClient, queueErr := queue.NewClient(redisCfg)
			if queueErr != nil { logger.Error("queue initialization failed", "error", queueErr); os.Exit(1) }
			defer queueClient.Close()
			provider, providerErr := yandexart.New(yandexart.Config{
				Endpoint: cfg.YandexARTEndpoint, OperationEndpoint: cfg.YandexARTOperationEndpoint,
				APIKey: cfg.YandexARTAPIKey, FolderID: cfg.YandexARTFolderID, Model: cfg.YandexARTModel,
			})
			if providerErr != nil { logger.Error("YandexART initialization failed", "error", providerErr); os.Exit(1) }
			generationStore, generationErr := generation.NewSQLStore(projectDB)
			if generationErr != nil { logger.Error("generation store initialization failed", "error", generationErr); os.Exit(1) }
			generationHandler, err = generation.NewHandlerWithDependencies(generationStore, queueClient, provider, provenanceStore, eventStore)
			if err != nil { logger.Error("generation handler initialization failed", "error", err); os.Exit(1) }
		}
	}

	api := httpapi.NewServerWithStudio(
		logger, cfg.CORSOrigins, limiter,
		projectHandler, iterationHandler, generationHandler,
		eventHandler, promptHandler, referenceHandler, actionHandler, provenanceHandler, assetHandler, similarityHandler, exportHandler, compositionHandler, libraryHandler,
	)
	srv := api.HTTPServer(":"+cfg.Port, cfg.ReadTimeout, cfg.WriteTimeout, cfg.IdleTimeout)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api server starting", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) { errCh <- err }
	}()

	sig, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-sig.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		logger.Error("api server failed", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil { logger.Error("graceful shutdown failed", "error", err); os.Exit(1) }
	logger.Info("api server stopped")
}
