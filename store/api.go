package store

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrExistingTransaction = errors.New("existing transaction")
	ErrNewerPosition       = errors.New("have newer position")
	ErrPositionNotFound    = errors.New("position not found")
)

type Store interface {
	InsertTransaction(transaction Transaction) error
	GetAllTransactions() (transactions []Transaction, err error)
	SetPosition(position Position) error
	GetTotals() (map[string]Total, error)
}

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

type Position struct {
	AccountIDHash string          `json:"account_id_hash"`
	CUSIP         string          `json:"cusip"`
	Total         decimal.Decimal `json:"total"`
	Date          time.Time       `json:"date"`
}

type Total struct {
	Accounts int             `json:"accounts"`
	Shares   decimal.Decimal `json:"shares"`
}
