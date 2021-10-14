package store

import (
	"context"
	"embed"
	"fmt"

	"github.com/Boostport/migration"
	"github.com/Boostport/migration/driver/postgres"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Create migration source
//go:embed postgres/migrations
var embedFS embed.FS

var embedSource = &migration.EmbedMigrationSource{
	EmbedFS: embedFS,
	Dir:     "postgres/migrations",
}

func NewPG(dsn string) (Store, error) {
	pool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	s := &pgstore{dsn, pool}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

type pgstore struct {
	dsn  string
	pool *pgxpool.Pool
}

func (pg *pgstore) InsertTransaction(tx Transaction) error {
	_, err := pg.pool.Exec(context.Background(), `
INSERT INTO transactions (
	id_hash, account_id_hash, cusip,
	description, amount, deduction_amount,
	net_amount, price_per_share, total_shares,
	transaction_date, requester_hash
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`,
		tx.IDHash, tx.AccountIDHash, tx.CUSIP,
		tx.Description, tx.Amount, tx.DeductionAmount,
		tx.NetAmount, tx.PricePerShare, tx.TotalShares,
		tx.Date, tx.RequesterHash,
	)
	if err != nil {
		return err
	}
	return nil
}

func (pg *pgstore) GetTransactions(CUSIP string) (transactions []Transaction, err error) {
	rows, err := pg.pool.Query(context.Background(), `
SELECT
	id_hash, account_id_hash, cusip,
	description, amount, deduction_amount,
	net_amount, price_per_share, total_shares, transaction_date
FROM transactions WHERE cusip = $1 ORDER BY transaction_date desc`, CUSIP)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	transactions = make([]Transaction, 0)
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.IDHash, &tx.AccountIDHash, &tx.CUSIP,
			&tx.Description, &tx.Amount, &tx.DeductionAmount,
			&tx.NetAmount, &tx.PricePerShare, &tx.TotalShares, &tx.Date); err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}
	return transactions, nil
}

func (pg *pgstore) SetPosition(position Position) error {
	_, err := pg.pool.Exec(context.Background(), `
INSERT INTO positions as p (
	account_id_hash, cusip, total_shares, transaction_date
)
VALUES ($1, $2, $3, $4) ON CONFLICT (account_id_hash, cusip) DO UPDATE
SET total_shares = $3, transaction_date = $4
WHERE p.transaction_date < $4
	`,
		position.AccountIDHash, position.CUSIP, position.Total, position.Date,
	)
	if err != nil {
		return err
	}
	return nil
}

func (pg *pgstore) GetAllPositions(CUSIP string) (positions []Position, err error) {
	sql := `
	SELECT
		account_id_hash, cusip, total_shares, transaction_date
	FROM positions WHERE cusip = $1`
	rows, err := pg.pool.Query(context.Background(), sql, CUSIP)
	if err != nil {
		return nil, err
	}
	positions = make([]Position, 0)
	defer rows.Close()
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.AccountIDHash, &p.CUSIP, &p.Total, &p.Date); err != nil {
			return nil, err
		}
		positions = append(positions, p)
	}
	return positions, nil
}

func (pg *pgstore) GetTotals() (map[string]Total, error) {
	sql := `SELECT cusip, count(account_id_hash) as accounts, sum(total_shares) FROM positions GROUP BY cusip`
	rows, err := pg.pool.Query(context.Background(), sql)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	defer rows.Close()
	totals := map[string]Total{}
	defer rows.Close()
	for rows.Next() {
		var (
			cusip string
			total Total
		)
		if err := rows.Scan(&cusip, &total.Accounts, &total.Shares); err != nil {
			return nil, err
		}
		totals[cusip] = total
	}
	return totals, nil
}

func (pg *pgstore) migrate() error {
	driver, err := postgres.New(pg.dsn)
	if err != nil {
		return err
	}
	defer driver.Close()
	_, err = migration.Migrate(driver, embedSource, migration.Up, 0)
	if err != nil {
		return err
	}
	return nil
}
