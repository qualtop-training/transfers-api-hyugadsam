package transport

import (
	"transfers-api/internal/handlers"
	"transfers-api/internal/logging"

	"github.com/gin-gonic/gin"
)

//go:generate mockery --name TransfersHandler --structname TransfersHandlerMock --filenametransfers_handler_mock.go --output mocks --outpkg mocks

type TransfersHandler interface {
	Create(ctx *gin.Context)
	GetByID(ctx *gin.Context)
	Update(ctx *gin.Context)
	Delete(ctx *gin.Context)
	GetByUserID(ctx *gin.Context)
}

type MqHandler interface {
	Read(ctx *gin.Context)
}


type HTTPServer struct {
	engine           *gin.Engine
	transfersHandler TransfersHandler
	mqHandler 		 MqHandler
}

func NewHTTPServer(transfersHandler TransfersHandler, mqHandler MqHandler) *HTTPServer {
	engine := gin.Default()
	engine.Use(handlers.AllowCORS)
	return &HTTPServer{
		engine:           engine,
		transfersHandler: transfersHandler,
		mqHandler: mqHandler,
	}
}

func (s *HTTPServer) MapRoutes() {
	s.engine.GET("/transfers/:id", s.transfersHandler.GetByID)
	s.engine.POST("/transfers", s.transfersHandler.Create)
	s.engine.PUT("/transfers/:id", s.transfersHandler.Update)
	s.engine.DELETE("/transfers/:id", s.transfersHandler.Delete)
	s.engine.GET("/mq/", s.mqHandler.Read)
	s.engine.GET("/transfers", s.transfersHandler.GetByUserID)
}

func (s *HTTPServer) Run(port string) {
	if err := s.engine.Run(port); err != nil {
		logging.Logger.Fatalf("failed to run server: %v", err)
	}
}
