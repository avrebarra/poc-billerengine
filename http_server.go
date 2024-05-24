package main

import (
	"fmt"
	"net/http"
	"time"

	validator "github.com/avrebarra/minivalidator"
	"github.com/gin-gonic/gin"
)

type ServerConfig struct {
	StartTime    time.Time     `validate:"required"`
	BillerEngine *BillerEngine `validate:"required"`
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

	r.GET("/", e.Ping())
	r.POST("/billables", e.HandleMakeBillable())
	r.POST("/billables/:billable_id/make-payment", e.HandleMakePayment())
	r.POST("/billables/:billable_id/check-delinquency", e.HandleCheckDelinquency())
	r.GET("/billables/:billable_id/outstandings/", e.HandleGetOutstanding())

	return r
}

func (e *Server) Ping() gin.HandlerFunc {
	type Response struct {
		Status    string    `json:"ok"`
		StartedAt time.Time `json:"started_at"`
		Uptime    string    `json:"uptime"`
	}

	fmtDurCompact := func(d time.Duration) string {
		days := int(d.Hours() / 24)
		hours := int(d.Hours()) % 24
		minutes := int(d.Minutes()) % 60
		seconds := int(d.Seconds()) % 60

		result := ""
		if days > 0 {
			result += fmt.Sprintf("%dd", days)
		}
		if hours > 0 {
			result += fmt.Sprintf("%dh", hours)
		}
		if minutes > 0 {
			result += fmt.Sprintf("%dm", minutes)
		}
		result += fmt.Sprintf("%ds", seconds)

		return result
	}

	return func(c *gin.Context) {
		c.JSON(http.StatusOK, e.buildJSONResponse(Response{
			Status:    "up",
			StartedAt: e.Config.StartTime,
			Uptime:    fmtDurCompact(time.Since(e.Config.StartTime)),
		}))
	}
}

func (e *Server) HandleMakeBillable() gin.HandlerFunc {
	type Request struct {
		BillableID      string `json:"billable_id"`
		PrincipalAmount int    `json:"amount_principal"`
	}
	type Response struct {
		ID        string    `json:"id"`
		Amount    int       `json:"amount"`
		Principal int       `json:"principal"`
		DurWeek   int       `json:"dur_week"`
		CreatedAt time.Time `json:"created_at"`
		DueAt     time.Time `json:"due_at"`
	}
	return func(ctx *gin.Context) {
		var req Request
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		billable, err := e.Config.BillerEngine.MakeBillable(InputMakeBillable{
			BID:       req.BillableID,
			Principal: req.PrincipalAmount,
		})
		if err != nil {
			err = fmt.Errorf("billable creation failed: %w", err)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, e.buildJSONResponse(Response(billable)))
	}
}

func (e *Server) HandleMakePayment() gin.HandlerFunc {
	type Request struct {
		BillableID string    `uri:"billable_id"`
		Amount     int       `json:"amount"`
		PaidAt     time.Time `json:"paid_at"`
	}
	type Response struct {
		ID                string    `json:"id"`
		BillableID        string    `json:"billable_id"`
		Amount            int       `json:"amount"`
		AmountAccumulated int       `json:"amount_accumulated"`
		PaidAt            time.Time `json:"paid_at"`
		CreatedAt         time.Time `json:"created_at"`
	}
	return func(ctx *gin.Context) {
		var req Request
		if err := ctx.ShouldBind(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := ctx.ShouldBindUri(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.PaidAt.IsZero() {
			req.PaidAt = time.Now()
		}
		payment, err := e.Config.BillerEngine.MakePayment(req.BillableID, InputMakePayment{
			Amount: req.Amount,
			PaidAt: req.PaidAt,
		})
		if err != nil {
			err = fmt.Errorf("payment failed: %w", err)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, e.buildJSONResponse(Response(payment)))
	}
}

func (e *Server) HandleCheckDelinquency() gin.HandlerFunc {
	type Request struct {
		BillableID string `uri:"billable_id"`
	}
	type Response struct {
		Delinquency bool `json:"delinquency"`
	}
	return func(ctx *gin.Context) {
		var req Request
		if err := ctx.ShouldBindUri(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		status, err := e.Config.BillerEngine.IsDelinquent(req.BillableID)
		if err != nil {
			err = fmt.Errorf("getting delinquency status failed: %w", err)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, e.buildJSONResponse(Response(status)))
	}
}

func (e *Server) HandleGetOutstanding() gin.HandlerFunc {
	type Request struct {
		BillableID string `uri:"billable_id"`
	}
	type Response struct {
		Principal   int `json:"principal"`
		Bill        int `json:"bill"`
		Paid        int `json:"paid"`
		Outstanding int `json:"outstanding"`
	}
	return func(ctx *gin.Context) {
		var req Request
		if err := ctx.ShouldBindUri(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		status, err := e.Config.BillerEngine.GetOutstanding(req.BillableID)
		if err != nil {
			err = fmt.Errorf("getting outstanding status failed: %w", err)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, e.buildJSONResponse(Response(status)))
	}
}

// ***

func (e *Server) ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			statusCode := http.StatusBadRequest
			responseData := gin.H{"error": err.Err.Error()}

			c.JSON(statusCode, responseData)
		}
	}
}

func (e *Server) buildJSONResponse(data interface{}) gin.H {
	return gin.H{"data": data}
}
