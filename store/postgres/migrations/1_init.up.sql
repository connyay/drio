CREATE TABLE IF NOT EXISTS transactions(
    id_hash TEXT NOT NULL,
    account_id_hash TEXT NOT NULL,
    cusip TEXT NOT NULL,
    description TEXT NOT NULL,
    amount NUMERIC NOT NULL,
    requester_hash TEXT NOT NULL,
    deduction_amount NUMERIC NOT NULL,
    net_amount NUMERIC NOT NULL,
    price_per_share NUMERIC NOT NULL,
    total_shares NUMERIC NOT NULL,
    transaction_date TIMESTAMP NOT NULL,
    PRIMARY KEY(id_hash, account_id_hash)
);

CREATE TABLE IF NOT EXISTS positions(
    account_id_hash TEXT NOT NULL,
    cusip TEXT NOT NULL,
    total_shares NUMERIC NOT NULL,
    transaction_date TIMESTAMP NOT NULL,
    PRIMARY KEY(account_id_hash, cusip)
);