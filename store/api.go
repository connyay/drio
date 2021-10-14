package store

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

var (
	// ErrExistingTransaction is returned when a transaction fails to insert due
	// to an existing transaction with the same id + account.
	ErrExistingTransaction = errors.New("existing transaction")
	// ErrNewerPosition is returned when a position fails to insert due to an
	// existing position with a later date was reported.
	ErrNewerPosition = errors.New("have newer position")
)

// Store is the required interface for a storage backend to implement.
type Store interface {
	// InsertTransaction stores the given transaction.
	InsertTransaction(transaction Transaction) error
	// GetTransactions returns all transaction for a given cusip.
	GetTransactions(CUSIP string) (transactions []Transaction, err error)
	// SetPosition stores the given postion.
	SetPosition(position Position) error
	// GetTotals returns a map of cusip to total shares and account.
	GetTotals() (map[string]Total, error)
	// Reset clears all transactions and positions.
	Reset() error
}

// Transaction holds the values of a transaction that are stored.
type Transaction struct {
	IDHash          string          `json:"id_hash"`
	AccountIDHash   string          `json:"account_id_hash"`
	RequesterHash   string          `json:"requester_hash"`
	CUSIP           string          `json:"cusip"`
	Description     string          `json:"description"`
	Amount          decimal.Decimal `json:"amount"`
	DeductionAmount decimal.Decimal `json:"deduction_amount"`
	NetAmount       decimal.Decimal `json:"net_amount"`
	PricePerShare   decimal.Decimal `json:"price_per_share"`
	TotalShares     decimal.Decimal `json:"total_shares"`
	Date            time.Time       `json:"date"`
}

// Position holds the values of a position that are stored.
type Position struct {
	AccountIDHash string          `json:"account_id_hash"`
	CUSIP         string          `json:"cusip"`
	Total         decimal.Decimal `json:"total"`
	Date          time.Time       `json:"date"`
}

// Total holds the number of accounts and shares that will be stored.
type Total struct {
	Accounts int             `json:"accounts"`
	Shares   decimal.Decimal `json:"shares"`
}
