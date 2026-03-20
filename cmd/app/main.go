package main

import (
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

	// init repositories
	transfersDB := repositories.NewTransfersMongoDBRepository(cfg.MongoDBConfig)
	transfersCache := repositories.NewTransfersMemcachedRepository(cfg.MemcachedConfig)
	transfersLocalCache := repositories.NewTransfersLocalCacheRepository(cfg.LocalCacheConfig)
	logger.Info("repositories created")
	
	// init clients
	tranfersDBPublisher := clients.NewRabbitMQClient(cfg.RabbitMQConfig)
	
	// init services
	transfersService := services.NewTransfersService(cfg.Business, transfersDB, transfersCache, transfersLocalCache, tranfersDBPublisher)
	mqService := services.NewMqService(tranfersDBPublisher)
	logger.Infof("services created")

	// init handlers
	transfersHandler := handlers.NewTransfersHandler(transfersService)
	mqHandler := handlers.NewMqHandler(mqService)
	logger.Infof("Handlers created")

	// init server
	server := transport.NewHTTPServer(transfersHandler, mqHandler)
	server.MapRoutes()
	logger.Infof("server created, running %s@%s", version.AppName, version.Version)

	// run server
	server.Run(":8080")
}
