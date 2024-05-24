package main

import (
	"database/sql"
	"testing"
	"time"

	"github.com/rs/xid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const CreateTableQuery = `
CREATE TABLE IF NOT EXISTS billables (
    id VARCHAR(255) PRIMARY KEY,
    amount INTEGER,
    principal INTEGER,
    dur_week INTEGER,
    created_at DATETIME,
    due_at DATETIME
);

CREATE TABLE IF NOT EXISTS payments (
    id VARCHAR(255) PRIMARY KEY,
    billable_id VARCHAR(255),
    amount INTEGER,
    amount_accumulated INTEGER,
    paid_at DATETIME,
    created_at DATETIME,
    FOREIGN KEY (billable_id) REFERENCES billables(id)
);
`

func setupTestDB() *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(CreateTableQuery)
	if err != nil {
		panic(err)
	}

	return db
}

func TestNewBillerEngine(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		// arrange
		db := setupTestDB()
		defer db.Close()

		// act
		b, err := NewBillerEngine(BillerEngineConfig{
			Storage:             db,
			GenerateCurrentDate: func() time.Time { return time.Now() },

			DefaultLoanDurationWeeks:            50,
			DefaultInterestRatePercentage:       .1,
			PaymentSkipCountDeliquencyThreshold: 2,
		})

		// assert
		assert.NotEmpty(t, b)
		assert.NoError(t, err)
	})

	t.Run("invalid_config", func(t *testing.T) {
		// arrange
		db := setupTestDB()
		defer db.Close()

		// act
		b, err := NewBillerEngine(BillerEngineConfig{
			// Storage:                        db,
			GenerateCurrentDate: func() time.Time { return time.Now() },

			DefaultLoanDurationWeeks:            50,
			DefaultInterestRatePercentage:       .1,
			PaymentSkipCountDeliquencyThreshold: 2,
		})

		// assert
		assert.Empty(t, b)
		assert.Error(t, err)
	})
}

func TestBillerEngine_MakeBillable(t *testing.T) {
	db := setupTestDB()
	defer db.Close()

	b, err := NewBillerEngine(BillerEngineConfig{
		Storage:             db,
		GenerateCurrentDate: func() time.Time { return time.Now() },

		DefaultLoanDurationWeeks:            50,
		DefaultInterestRatePercentage:       .1,
		PaymentSkipCountDeliquencyThreshold: 2,
	})
	require.NoError(t, err)

	bid := xid.New().String()

	t.Run("ok", func(t *testing.T) {
		// arrange
		// act
		out, err := b.MakeBillable(InputMakeBillable{
			BID:       bid,
			Principal: 5_000_000,
		})

		// assert
		assert.NotEmpty(t, out)
		assert.NoError(t, err)

		var count int
		query := "SELECT COUNT(*) FROM billables;"
		err = db.QueryRow(query).Scan(&count)

		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("invalid_input", func(t *testing.T) {
		// arrange
		// act
		out, err := b.MakeBillable(InputMakeBillable{
			// BID:       bid,
			Principal: 5_000_000,
		})

		// assert
		assert.Empty(t, out)
		assert.Error(t, err)

		var count int
		query := "SELECT COUNT(*) FROM billables;"
		err = db.QueryRow(query).Scan(&count)

		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("duplicate_id", func(t *testing.T) {
		// arrange
		// act
		out, err := b.MakeBillable(InputMakeBillable{
			BID:       bid,
			Principal: 5_000_000,
		})

		// assert
		assert.Empty(t, out)
		assert.Error(t, err)

		var count int
		query := "SELECT COUNT(*) FROM billables;"
		err = db.QueryRow(query).Scan(&count)

		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

func TestBillerEngine_Flows(t *testing.T) {
	db := setupTestDB()
	defer db.Close()

	curdate := time.Now()
	getDate := func() time.Time { return curdate }
	bid := xid.New().String()
	eng, err := NewBillerEngine(BillerEngineConfig{
		Storage:             db,
		GenerateCurrentDate: func() time.Time { return getDate() },

		DefaultLoanDurationWeeks:            50,
		DefaultInterestRatePercentage:       .1,
		PaymentSkipCountDeliquencyThreshold: 2,
	})
	require.NoError(t, err)

	t.Run("initial_billable_state_okay", func(t *testing.T) {
		_, err := eng.MakeBillable(InputMakeBillable{BID: bid, Principal: 5_000_000})
		assert.NoError(t, err)

		outstanding, err := eng.GetOutstanding(bid)
		assert.NoError(t, err)
		assert.Equal(t, 5_000_000, outstanding.Principal)
		assert.Equal(t, 5_500_000, outstanding.Bill)

		delinquency, err := eng.IsDelinquent(bid)
		assert.NoError(t, err)
		assert.Equal(t, false, delinquency.Delinquency)
	})

	t.Run("sequential_late_payment", func(t *testing.T) {
		orig := getDate
		defer func() { getDate = orig }()

		getDate = func() time.Time { return curdate.AddDate(0, 0, 14) }
		delinquency, err := eng.IsDelinquent(bid)
		assert.NoError(t, err)
		assert.Equal(t, true, delinquency.Delinquency)
	})

	t.Run("non_sequential_late_payment", func(t *testing.T) {
		orig := getDate
		defer func() { getDate = orig }()

		getDate = func() time.Time { return curdate.AddDate(0, 0, 14) }
		payment, err := eng.MakePayment(bid, InputMakePayment{Amount: 110_000, PaidAt: getDate()})
		assert.NoError(t, err)
		assert.Equal(t, 110_000, payment.Amount)
		assert.Equal(t, 110_000, payment.AmountAccumulated)

		getDate = func() time.Time { return curdate.AddDate(0, 0, 14) }
		delinquency, err := eng.IsDelinquent(bid)
		assert.NoError(t, err)
		assert.Equal(t, false, delinquency.Delinquency)

		getDate = func() time.Time { return curdate.AddDate(0, 0, 28) }
		delinquency, err = eng.IsDelinquent(bid)
		assert.NoError(t, err)
		assert.Equal(t, true, delinquency.Delinquency)

		getDate = func() time.Time { return curdate.AddDate(0, 0, 35) }
		delinquency, err = eng.IsDelinquent(bid)
		assert.NoError(t, err)
		assert.Equal(t, true, delinquency.Delinquency)

		getDate = func() time.Time { return curdate.AddDate(0, 0, 35) }
		payment, err = eng.MakePayment(bid, InputMakePayment{Amount: 110_000, PaidAt: getDate()})
		assert.NoError(t, err)
		assert.Equal(t, 110_000, payment.Amount)
		assert.Equal(t, 220_000, payment.AmountAccumulated)

		getDate = func() time.Time { return curdate.AddDate(0, 0, 35) }
		payment, err = eng.MakePayment(bid, InputMakePayment{Amount: 110_000, PaidAt: getDate()})
		assert.NoError(t, err)
		assert.Equal(t, 110_000, payment.Amount)
		assert.Equal(t, 330_000, payment.AmountAccumulated)

		getDate = func() time.Time { return curdate.AddDate(0, 0, 35) }
		payment, err = eng.MakePayment(bid, InputMakePayment{Amount: 110_000, PaidAt: getDate()})
		assert.NoError(t, err)
		assert.Equal(t, 110_000, payment.Amount)
		assert.Equal(t, 440_000, payment.AmountAccumulated)

		getDate = func() time.Time { return curdate.AddDate(0, 0, 35) }
		delinquency, err = eng.IsDelinquent(bid)
		assert.NoError(t, err)
		assert.Equal(t, false, delinquency.Delinquency)
	})
}
