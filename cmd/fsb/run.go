package main

import (
	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/bot"
	"EverythingSuckz/fsb/internal/cache"
	"EverythingSuckz/fsb/internal/routes"
	"EverythingSuckz/fsb/internal/types"
	"EverythingSuckz/fsb/internal/utils"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var runCmd = &cobra.Command{
	Use:                "run",
	Short:              "Run the bot with the given configuration.",
	DisableSuggestions: false,
	Run:                runApp,
}

var startTime time.Time = time.Now()

func runApp(cmd *cobra.Command, args []string) {
	// initialize logger early so config loading is logged to file
	utils.InitLogger(false)
	config.Load(utils.Logger, cmd)
	// reinitialize with correct dev mode
	utils.InitLogger(config.ValueOf.Dev)
	log := utils.Logger
	mainLogger := log.Named("Main")
	mainLogger.Info("Starting server")
	router := getRouter(log)

	mainBot, err := bot.StartClient(log)
	if err != nil {
		log.Sugar().Fatalf("Failed to start main bot: %v", err)
	}
	cache.InitCache(log)
	workers, err := bot.StartWorkers(log)
	if err != nil {
		log.Sugar().Fatalf("Failed to start workers: %v", err)
	}
	workers.AddDefaultClient(mainBot, mainBot.Self)
	bot.StartUserBot(log)
	mainLogger.Info("Server started", zap.Int("port", config.ValueOf.Port))
	mainLogger.Info("File Stream Bot", zap.String("version", versionString))
	mainLogger.Sugar().Infof("Server is running at %s", config.ValueOf.Host)

	addr := fmt.Sprintf(":%d", config.ValueOf.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start HTTP server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			mainLogger.Sugar().Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	mainLogger.Sugar().Infof("Received signal %v, shutting down...", sig)

	// Give in-flight requests up to 10 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		mainLogger.Error("Server forced to shutdown", zap.Error(err))
	}

	mainLogger.Info("Server exited")
}

func getRouter(log *zap.Logger) *gin.Engine {
	if config.ValueOf.Dev {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()
	router.Use(gin.ErrorLogger())
	router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, types.RootResponse{
			Message: "Server is running.",
			Ok:      true,
			Uptime:  utils.TimeFormat(uint64(time.Since(startTime).Seconds())),
			Version: versionString,
		})
	})
	routes.Load(log, router)
	return router
}
