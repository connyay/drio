import React, { useEffect, useState, useMemo } from "react";
import axios from "axios";

const transactionsURL = `${process.env.REACT_APP_API || ""}/api/transactions`;

function Transactions({ location }) {
  const [txs, setTxs] = useState([]);

  const params = useMemo(
    () => new URLSearchParams(location.search),
    [location]
  );
  const cusip = params.get("cusip");
  useEffect(() => {
    if (!cusip) {
      return;
    }
    axios
      .get(transactionsURL, { params })
      .then((res) => {
        setTxs(res.data.transactions);
      })
      .catch((err) => {
        console.error("failed import", err);
      });
  }, [params, cusip]);

  if (!cusip) {
    return null;
  }
  if (!txs.length) {
    return null;
  }

  return (
    <>
      <br />
      <strong>{cusip} Transactions</strong>
      <table className="u-full-width">
        <thead>
          <tr>
            <th>Date</th>
            <th>Account</th>
            <th>Description</th>
            <th>PPS</th>
            <th>Shares</th>
          </tr>
        </thead>
        <tbody>
          {txs.map((tx) => (
            <tr key={tx.id_hash}>
              <td>{tx.date.slice(0, 10)}</td>
              <td title={tx.account_id_hash}>{tx.account_id_hash.slice(0, 7)}...</td>
              <td>{tx.description}</td>
              <td>{tx.price_per_share}</td>
              <td>{tx.total_shares}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}

export default Transactions;
