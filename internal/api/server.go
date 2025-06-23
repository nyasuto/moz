package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nyasuto/moz/internal/kvstore"
)

type Server struct {
	store  *kvstore.KVStore
	port   string
	router *gin.Engine
	auth   *AuthManager
}

func NewServer(dataPath, port string) *Server {
	store := kvstore.New()
	auth := NewAuthManager()

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	s := &Server{
		store:  store,
		port:   port,
		router: router,
		auth:   auth,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	api := s.router.Group("/api/v1")
	{
		api.GET("/health", s.healthCheck)
		api.POST("/login", s.login)

		// Protected routes
		protected := api.Group("/")
		protected.Use(s.AuthMiddleware())
		{
			protected.GET("/stats", s.getStats)

			kv := protected.Group("/kv")
			{
				kv.PUT("/:key", s.putKey)
				kv.GET("/:key", s.getKey)
				kv.DELETE("/:key", s.deleteKey)
				kv.GET("", s.listKeys)
			}
		}
	}
}

func (s *Server) Start() error {
	fmt.Printf("Starting moz-server on port %s\n", s.port)
	return http.ListenAndServe(":"+s.port, s.router)
}

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "moz-server",
	})
}

func (s *Server) getStats(c *gin.Context) {
	stats, err := s.store.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Status: "error",
			Error: &APIError{
				Code:    "STATS_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Status: "success",
		Data:   stats,
	})
}
