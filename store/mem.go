package store

import (
	"sort"
	"sync"
)

type positionKey struct {
	AccountIDHash string
	CUSIP         string
}

// NewMem returns an initialized Store backed by in memory storage.
func NewMem() Store {
	return &memStore{
		positions:    map[positionKey]Position{},
		transactions: map[string]Transaction{},
	}
}

type memStore struct {
	mu           sync.RWMutex
	transactions map[ /*IDHash*/ string]Transaction
	positions    map[positionKey]Position
}

func (ms *memStore) InsertTransaction(transaction Transaction) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	_, exists := ms.transactions[transaction.IDHash]
	if exists {
		return ErrExistingTransaction
	}
	ms.transactions[transaction.IDHash] = transaction
	return nil
}

func (ms *memStore) GetTransactions(CUSIP string) (transactions []Transaction, err error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	transactions = make([]Transaction, 0)
	for _, transaction := range ms.transactions {
		if transaction.CUSIP == CUSIP {
			transactions = append(transactions, transaction)
		}
	}
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[j].Date.Before(transactions[i].Date)
	})
	return transactions, nil
}

func (ms *memStore) SetPosition(position Position) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	key := positionKey{position.AccountIDHash, position.CUSIP}
	previousPosition := ms.positions[key]
	if previousPosition.Date.After(position.Date) {
		return ErrNewerPosition
	}
	ms.positions[key] = position
	return nil
}

func (ms *memStore) GetTotals() (map[string]Total, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	totals := map[string]Total{}
	for _, position := range ms.positions {
		total := totals[position.CUSIP]
		total.Accounts++
		total.Shares = total.Shares.Add(position.Total)
		totals[position.CUSIP] = total
	}
	return totals, nil
}

func (ms *memStore) Reset() error {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	ms.positions = map[positionKey]Position{}
	ms.transactions = nil
	return nil
}
