package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"learning-runtime/auth"
	"learning-runtime/db"
	"learning-runtime/engine"
	"learning-runtime/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(os.Getenv("LOG_LEVEL")),
	}))

	port := os.Getenv("PORT")
	if port == "" { port = "3000" }
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" { dbPath = "./data/runtime.db" }
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" { baseURL = fmt.Sprintf("http://localhost:%s", port) }

	// Init JWT
	if err := auth.LoadJWTSecret(); err != nil {
		logger.Error("failed to load JWT secret", "err", err)
		os.Exit(1)
	}

	// Open DB
	os.MkdirAll("data", 0755)
	database, err := db.OpenDB(dbPath)
	if err != nil {
		logger.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		logger.Error("failed to migrate database", "err", err)
		os.Exit(1)
	}
	logger.Info("database ready", "path", dbPath)

	store := db.NewStore(database)

	// Create MCP server
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "learning-runtime",
		Version: "1.0.0",
	}, nil)

	// Register tools
	deps := &tools.Deps{Store: store}
	tools.RegisterTools(mcpServer, deps)

	// Create MCP handler
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	// OAuth server
	oauthServer := auth.NewOAuthServer(store, baseURL, logger)

	// HTTP mux
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// OAuth routes
	oauthServer.RegisterRoutes(mux)

	// MCP route (auth protected)
	mux.Handle("/mcp", auth.BearerMiddleware(baseURL, mcpHandler))

	// Start scheduler
	scheduler := engine.NewScheduler(store, logger)
	if err := scheduler.Start(); err != nil {
		logger.Error("failed to start scheduler", "err", err)
		os.Exit(1)
	}
	defer scheduler.Stop()

	logger.Info("learning runtime starting", "port", port, "base_url", baseURL)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		logger.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug": return slog.LevelDebug
	case "warn": return slog.LevelWarn
	case "error": return slog.LevelError
	default: return slog.LevelInfo
	}
}
