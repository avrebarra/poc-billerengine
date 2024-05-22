package main

import (
	"fmt"
	"net/http"
	"time"

	validator "github.com/avrebarra/minivalidator"
	"github.com/gin-gonic/gin"
)

type ServerConfig struct {
	StartTime    time.Time    `validate:"required"`
	BillerEngine BillerEngine `validate:"required"`
}

type Server struct {
	Config ServerConfig
}

func NewServer(cfg ServerConfig) (*Server, error) {
	if err := validator.Validate(cfg); err != nil {
		err = fmt.Errorf("bad config: %w", err)
		return nil, err
	}
	return &Server{Config: cfg}, nil
}

func (e *Server) GetRouterEngine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()
	r.Use(e.ErrorHandler())

	r.POST("/billables", e.HandleAddBillable())
	r.POST("/billables/:id/make-payment", e.HandleMakePayment())
	r.POST("/billables/:id/check-delinquency", e.HandleCheckDelinquency())
	r.GET("/billables/:id/outstandings/", e.HandleGetOutstanding())

	return r
}

func (e *Server) HandleAddBillable() gin.HandlerFunc {
	return func(ctx *gin.Context) {
	}
}

func (e *Server) HandleMakePayment() gin.HandlerFunc {
	return func(ctx *gin.Context) {
	}
}

func (e *Server) HandleCheckDelinquency() gin.HandlerFunc {
	return func(ctx *gin.Context) {
	}
}

func (e *Server) HandleGetOutstanding() gin.HandlerFunc {
	return func(ctx *gin.Context) {
	}
}

// ***

func (e *Server) ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": c.Errors[0]})
		}
	}
}

func (e *Server) buildJSONResponse(result bool, key string, data interface{}) gin.H {
	return gin.H{"result": result, key: data}
}
