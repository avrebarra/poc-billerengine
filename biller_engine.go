package main

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	validator "github.com/avrebarra/minivalidator"
	"github.com/mattn/go-sqlite3"
	"github.com/rs/xid"
)

type BillerEngineConfig struct {
	Storage             *sql.DB          `validate:"required"`
	GenerateCurrentDate func() time.Time `validate:"required"`

	DefaultLoanDurationWeeks            int     `validate:"required"`
	DefaultInterestRatePercentage       float64 `validate:"required"` // percentage in float
	PaymentSkipCountDeliquencyThreshold int     `validate:"required"` // how many payments to skip until marked as delinquent
}

type BillerEngine struct {
	Conf BillerEngineConfig
}

func NewBillerEngine(conf BillerEngineConfig) (out *BillerEngine, err error) {
	if err = validator.Validate(conf); err != nil {
		err = fmt.Errorf("bad config: %w", err)
		return
	}
	out = &BillerEngine{Conf: conf}
	return
}

func (b *BillerEngine) MakeBillable(in InputMakeBillable) (out Billable, err error) {
	// validate inputs
	if err = validator.Validate(in); err != nil {
		err = fmt.Errorf("bad input: %w", err)
		return
	}

	curDate := b.Conf.GenerateCurrentDate()
	dueDate := curDate.AddDate(0, b.Conf.DefaultLoanDurationWeeks, 0)
	amount := int(math.Ceil(float64(in.Principal) * (b.Conf.DefaultInterestRatePercentage + 1)))

	// create and store the billable
	billable := Billable{
		ID:        in.BID,
		Principal: in.Principal,
		Amount:    amount,
		DurWeek:   b.Conf.DefaultLoanDurationWeeks,
		CreatedAt: curDate,
		DueAt:     dueDate,
	}

	_, err = b.Conf.Storage.Exec(
		"INSERT INTO billables (id, amount, principal, dur_week, created_at, due_at) VALUES (?, ?, ?, ?, ?, ?);",
		billable.ID, billable.Amount, billable.Principal, billable.DurWeek, billable.CreatedAt, billable.DueAt,
	)
	var dbErr sqlite3.Error
	if errors.As(err, &dbErr) && dbErr.Code == sqlite3.ErrConstraint {
		err = fmt.Errorf("unique id violation: %w", err)
		return
	}
	if err != nil {
		err = fmt.Errorf("insert failed: %w", err)
		return
	}

	out = billable
	return
}

func (b *BillerEngine) GetOutstanding(bID string) (out OutstandingDetails, err error) {
	// validate required inputs
	if bID == "" {
		err = fmt.Errorf("bad input: billable id not defined")
		return
	}

	// find the billable
	var billable Billable
	err = b.Conf.Storage.QueryRow("SELECT amount, principal FROM billables WHERE id = ?", bID).Scan(&billable.Amount, &billable.Principal)
	if err != nil {
		err = fmt.Errorf("billable not found: id %s", bID)
		return
	}

	// find last payment for the billable
	var amountPaid int
	var paidAt time.Time
	err = b.Conf.Storage.QueryRow("SELECT amount_accumulated, paid_at FROM payments WHERE billable_id = ? ORDER BY created_at DESC, paid_at DESC LIMIT 1", bID).Scan(&amountPaid, &paidAt)
	if errors.Is(err, sql.ErrNoRows) {
		amountPaid = 0
		err = nil
	}
	if err != nil {
		err = fmt.Errorf("error fetching latest payment: %w", err)
		return
	}

	outstanding := billable.Amount - amountPaid
	out = OutstandingDetails{
		Principal:   billable.Principal,
		Bill:        billable.Amount,
		Paid:        amountPaid,
		Outstanding: outstanding,
	}

	return out, nil
}

func (b *BillerEngine) IsDelinquent(bID string) (out DelinquencyDetails, err error) {
	//  validate required inputs
	if bID == "" {
		err = fmt.Errorf("bad input: billable id not defined")
		return
	}

	// retrieve billable
	var billable Billable
	err = b.Conf.Storage.QueryRow("SELECT amount, dur_week, created_at FROM billables WHERE id = ?", bID).
		Scan(&billable.Amount, &billable.DurWeek, &billable.CreatedAt)
	if err != nil {
		return out, err
	}
	weeklyBillAmount := int(float64(billable.Amount) / float64(billable.DurWeek))

	// retrieve latest payments
	getWeeksSinceDate := func(startDate time.Time) int {
		millisecondsInWeek := 7 * 24 * 60 * 60 * 1000
		currentDate := b.Conf.GenerateCurrentDate()
		diffInMilliseconds := int(currentDate.Sub(startDate).Milliseconds())
		return diffInMilliseconds / millisecondsInWeek
	}

	var amountPaid int
	err = b.Conf.Storage.QueryRow("SELECT amount_accumulated FROM payments WHERE billable_id = ? ORDER BY created_at DESC, paid_at DESC LIMIT 1", bID).
		Scan(&amountPaid)
	if errors.Is(err, sql.ErrNoRows) {
		amountPaid = 0
		err = nil
	}
	if err != nil {
		err = fmt.Errorf("error fetching latest payment: %w", err)
		return
	}

	billableAgeInWeek := getWeeksSinceDate(billable.CreatedAt)
	expectedAggregatedPaidAmount := weeklyBillAmount * billableAgeInWeek
	delinquencyThreshold := b.Conf.PaymentSkipCountDeliquencyThreshold

	// build output
	out.Delinquency = expectedAggregatedPaidAmount-amountPaid >= delinquencyThreshold*int(weeklyBillAmount)
	return
}

func (b *BillerEngine) MakePayment(bID string, in InputMakePayment) (out Payment, err error) {
	// validate inputs
	if err = validator.Validate(in); err != nil {
		err = fmt.Errorf("bad input: %w", err)
		return
	}

	amount := in.Amount
	paidAt := in.PaidAt
	timestamp := b.Conf.GenerateCurrentDate()

	// retrieve billable
	var billable Billable
	err = b.Conf.Storage.QueryRow(
		"SELECT amount, principal, dur_week FROM billables WHERE id = ?", bID).
		Scan(&billable.Amount, &billable.Principal, &billable.DurWeek)
	if err != nil {
		err = fmt.Errorf("billable not found: id %s", bID)
		return
	}

	// validate amount
	if amount != (billable.Amount / billable.DurWeek) {
		err = fmt.Errorf("wrong payment amount increment: expected %d", billable.Amount/billable.DurWeek)
		return
	}

	// retrieve last payment aggregated amount
	var amountPaid int
	err = b.Conf.Storage.QueryRow("SELECT amount_accumulated FROM payments WHERE billable_id = ? ORDER BY id DESC, paid_at DESC", bID).Scan(&amountPaid)
	if errors.Is(err, sql.ErrNoRows) {
		amountPaid = 0
		err = nil
	}
	if err != nil {
		err = fmt.Errorf("failed getting latest payment: %w", err)
		return
	}

	payment := Payment{
		ID:                xid.New().String(),
		BillableID:        bID,
		Amount:            amount,
		PaidAt:            paidAt,
		CreatedAt:         timestamp,
		AmountAccumulated: amountPaid + amount,
	}

	// save the new payment to the database
	_, err = b.Conf.Storage.Exec(
		"INSERT INTO payments (id, billable_id, amount, amount_accumulated, paid_at, created_at) VALUES (?, ?, ?, ?, ?, ?);",
		payment.ID, payment.BillableID, payment.Amount, payment.AmountAccumulated, payment.PaidAt, payment.CreatedAt,
	)
	if err != nil {
		err = fmt.Errorf("failed to save payment: %w", err)
		return
	}

	out = payment
	return
}

// ***

type InputMakeBillable struct {
	BID       string `validate:"required"`
	Principal int    `validate:"required"`
}

type InputMakePayment struct {
	Amount int       `validate:"required"`
	PaidAt time.Time `validate:"required"`
}

type OutstandingDetails struct {
	Principal   int
	Bill        int
	Paid        int
	Outstanding int
}

type DelinquencyDetails struct {
	Delinquency bool
}
