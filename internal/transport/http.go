package transport

import (
	"net/http"
	"transfers-api/internal/handlers"
	"transfers-api/internal/logging"

	"github.com/gin-gonic/gin"
)

//go:generate mockery --name TransfersHandler --structname TransfersHandlerMock --filename transfers_handler_mock.go --output mocks --outpkg mocks

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

// AuthHandler groups the four authentication endpoints.
type AuthHandler interface {
	Register(ctx *gin.Context)
	Login(ctx *gin.Context)
	RefreshToken(ctx *gin.Context)
	Logout(ctx *gin.Context)
}

type HTTPServer struct {
	engine           *gin.Engine
	transfersHandler TransfersHandler
	mqHandler        MqHandler
	authHandler      AuthHandler
	tokenValidator   handlers.TokenValidator
}

func NewHTTPServer(
	transfersHandler TransfersHandler,
	mqHandler MqHandler,
	authHandler AuthHandler,
	tokenValidator handlers.TokenValidator,
) *HTTPServer {
	engine := gin.Default()
	engine.Use(handlers.AllowCORS)
	return &HTTPServer{
		engine:           engine,
		transfersHandler: transfersHandler,
		mqHandler:        mqHandler,
		authHandler:      authHandler,
		tokenValidator:   tokenValidator,
	}
}

func (s *HTTPServer) MapRoutes() {
	jwtMiddleware := handlers.NewJWTMiddleware(s.tokenValidator)

	// ── Auth routes (public) ──────────────────────────────────────────────────
	auth := s.engine.Group("/auth")
	{
		auth.POST("/register", s.authHandler.Register)
		auth.POST("/login", s.authHandler.Login)
		auth.POST("/refresh", s.authHandler.RefreshToken)
		// Logout requires a valid access token.
		auth.POST("/logout", jwtMiddleware, s.authHandler.Logout)
	}

	// ── Transfers routes (protected) ─────────────────────────────────────────
	transfers := s.engine.Group("/transfers", jwtMiddleware)
	{
		transfers.POST("", s.transfersHandler.Create)
		transfers.GET("", s.transfersHandler.GetByUserID)
		transfers.GET("/:id", s.transfersHandler.GetByID)
		transfers.PUT("/:id", s.transfersHandler.Update)
		transfers.DELETE("/:id", s.transfersHandler.Delete)
	}

	// ── MQ routes (protected) ─────────────────────────────────────────────────
	mq := s.engine.Group("/mq", jwtMiddleware)
	{
		mq.GET("/", s.mqHandler.Read)
	}
}

func (s *HTTPServer) Run(port string) {
	if err := s.engine.Run(port); err != nil {
		logging.Logger.Fatalf("failed to run server: %v", err)
	}
}

// ServeHTTP implements http.Handler so HTTPServer can be used in tests via httptest.
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.engine.ServeHTTP(w, r)
}
