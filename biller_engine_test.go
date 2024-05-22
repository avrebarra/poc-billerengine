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

			DefaultLoanDurationWeeks:       50,
			DefaultInterestRatePercentage:  .1,
			DeliquencyPaymentSkipThreshold: 2,
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

			DefaultLoanDurationWeeks:       50,
			DefaultInterestRatePercentage:  .1,
			DeliquencyPaymentSkipThreshold: 2,
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

		DefaultLoanDurationWeeks:       50,
		DefaultInterestRatePercentage:  .1,
		DeliquencyPaymentSkipThreshold: 2,
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
		assert.Equal(t, count, 1)
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
		assert.Equal(t, count, 1)
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
		assert.Equal(t, count, 1)
	})
}

func TestBillerEngine_GetOutstanding(t *testing.T) {

}

func TestBillerEngine_MakePayment(t *testing.T) {
	db := setupTestDB()
	defer db.Close()

	b, err := NewBillerEngine(BillerEngineConfig{
		Storage:             db,
		GenerateCurrentDate: func() time.Time { return time.Now() },

		DefaultLoanDurationWeeks:       50,
		DefaultInterestRatePercentage:  .1,
		DeliquencyPaymentSkipThreshold: 2,
	})
	require.NoError(t, err)

	bid := xid.New().String()
	billable, err := b.MakeBillable(InputMakeBillable{
		BID:       bid,
		Principal: 5_000_000,
	})
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		// arrange
		// act
		out, err := b.MakePayment(billable.ID, InputMakePayment{Amount: 5_500_000 / 50, PaidAt: time.Now()})

		// assert
		assert.NotEmpty(t, out)
		assert.NoError(t, err)

		var count int
		query := "SELECT COUNT(*) FROM payments;"
		err = db.QueryRow(query).Scan(&count)

		assert.NoError(t, err)
		assert.Equal(t, count, 1)
	})

	t.Run("bad_input", func(t *testing.T) {
		// arrange
		// act
		out, err := b.MakePayment(billable.ID, InputMakePayment{PaidAt: time.Now()})

		// assert
		assert.Empty(t, out)
		assert.Error(t, err)

		var count int
		query := "SELECT COUNT(*) FROM payments;"
		err = db.QueryRow(query).Scan(&count)

		assert.NoError(t, err)
		assert.Equal(t, count, 1)
	})

	t.Run("bad_amount", func(t *testing.T) {
		// arrange
		// act
		out, err := b.MakePayment(billable.ID, InputMakePayment{Amount: 5_500_000/50 + 1, PaidAt: time.Now()})

		// assert
		assert.Empty(t, out)
		assert.Error(t, err)

		var count int
		query := "SELECT COUNT(*) FROM payments;"
		err = db.QueryRow(query).Scan(&count)

		assert.NoError(t, err)
		assert.Equal(t, count, 1)
	})

	t.Run("bad_billable", func(t *testing.T) {
		// arrange
		// act
		out, err := b.MakePayment(billable.ID+"1", InputMakePayment{Amount: 5_500_000/50 + 1, PaidAt: time.Now()})

		// assert
		assert.Empty(t, out)
		assert.Error(t, err)

		var count int
		query := "SELECT COUNT(*) FROM payments;"
		err = db.QueryRow(query).Scan(&count)

		assert.NoError(t, err)
		assert.Equal(t, count, 1)
	})

}
