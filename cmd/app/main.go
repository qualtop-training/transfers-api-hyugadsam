package main

import (
	"context"
	"transfers-api/internal/clients"
	"transfers-api/internal/config"
	"transfers-api/internal/handlers"
	"transfers-api/internal/logging"
	"transfers-api/internal/repositories"
	"transfers-api/internal/services"
	"transfers-api/internal/transport"
	"transfers-api/internal/version"
)

func main() {
	// init logger
	logger := logging.Logger
	logger.Info("logger started")

	// init config
	cfg := config.ParseFromEnv()
	logger.Infof("config loaded: %v", cfg.String())

	// ── Repositories ──────────────────────────────────────────────────────────
	transfersDB := repositories.NewTransfersMongoDBRepository(cfg.MongoDBConfig)
	transfersCache := repositories.NewTransfersMemcachedRepository(cfg.MemcachedConfig)
	transfersLocalCache := repositories.NewTransfersLocalCacheRepository(cfg.LocalCacheConfig)

	// User repository — chosen by AUTH_USER_DB_DRIVER (mongodb | mysql)
	var usersRepo services.UserRepository
	switch cfg.Auth.UserDBDriver {
	case "mysql":
		mysqlRepo := repositories.NewTransfersMySQLRepository(cfg.MySQLConfig)
		usersRepo = repositories.NewUsersMySQLRepository(mysqlRepo.DB())
		logger.Info("using MySQL for user repository")
	default:
		usersRepo = repositories.NewUsersMongoDBRepository(cfg.MongoDBConfig)
		logger.Info("using MongoDB for user repository")
	}
	logger.Info("repositories created")

	// ── Clients ───────────────────────────────────────────────────────────────
	transfersPublisher := clients.NewRabbitMQClient(cfg.RabbitMQConfig)

	// ── Services ──────────────────────────────────────────────────────────────
	transfersService := services.NewTransfersService(cfg.Business, transfersDB, transfersCache, transfersLocalCache, transfersPublisher)
	mqService := services.NewMqService(transfersPublisher)

	cryptoService := services.NewArgon2idHasher()
	jwtService := services.NewJWTService(cfg.Auth)
	authService := services.NewAuthService(usersRepo, cryptoService, jwtService, cfg.Auth.RefreshTokenTTLDays)
	logger.Info("services created")

	// ── Handlers ──────────────────────────────────────────────────────────────
	transfersHandler := handlers.NewTransfersHandler(transfersService)
	mqHandler := handlers.NewMqHandler(mqService)
	authHandler := handlers.NewAuthHandler(authService)
	logger.Info("handlers created")

	// ── Seed default admin ────────────────────────────────────────────────────
	services.SeedDefaultAdmin(
		context.Background(),
		usersRepo,
		cryptoService,
		cfg.Auth.DefaultAdminUsername,
		cfg.Auth.DefaultAdminEmail,
		cfg.Auth.DefaultAdminPassword,
	)
	logger.Info("seed step completed")

	// ── Server ────────────────────────────────────────────────────────────────
	server := transport.NewHTTPServer(transfersHandler, mqHandler, authHandler, jwtService)
	server.MapRoutes()
	logger.Infof("server created, running %s@%s", version.AppName, version.Version)

	server.Run(":8080")
}
