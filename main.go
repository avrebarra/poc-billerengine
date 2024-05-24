package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

const (
	ConfigDBAddress = "./db.sqlite"
	Port            = 5001
)

func main() {
	db, err := sql.Open("sqlite3", ConfigDBAddress)
	if err != nil {
		err = fmt.Errorf("db conn failed: %w", err)
		log.Fatal(err)
	}
	defer db.Close()

	billerengine, err := NewBillerEngine(BillerEngineConfig{
		Storage:                             db,
		GenerateCurrentDate:                 func() time.Time { return time.Now() },
		DefaultLoanDurationWeeks:            50,
		DefaultInterestRatePercentage:       .1,
		PaymentSkipCountDeliquencyThreshold: 2,
	})
	if err != nil {
		err = fmt.Errorf("engine setup failed: %w", err)
		log.Fatal(err)
	}

	server, err := NewServer(ServerConfig{
		StartTime:    time.Now(),
		BillerEngine: billerengine,
	})
	if err != nil {
		err = fmt.Errorf("server setup failed: %w", err)
		log.Fatal(err)
	}
	router := server.GetRouterEngine()

	addr := fmt.Sprintf(":%d", Port)
	fmt.Printf("server listening on port %s\n", addr)
	log.Fatal(router.Run(addr))
	fmt.Printf("server exited")
}
