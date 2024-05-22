package main

import "time"

type Billable struct {
	ID        string
	Amount    int
	Principal int
	DurWeek   int
	CreatedAt time.Time
	DueAt     time.Time
}

type Payment struct {
	ID                string
	BillableID        string
	Amount            int
	AmountAccumulated int
	PaidAt            time.Time
	CreatedAt         time.Time
}
