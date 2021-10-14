import React, { useEffect, useState } from "react";
import axios from "axios";
import { NavLink } from "react-router-dom";

const totalsURL = `${process.env.REACT_APP_API || ""}/api/totals`;

function Totals() {
  const [totals, setTotals] = useState([]);

  useEffect(() => {
    axios
      .get(totalsURL)
      .then((res) => {
        setTotals(res.data);
      })
      .catch((err) => {
        console.error("failed import", err);
      });
  }, []);

  const totalEntries = Object.entries(totals);
  if (!totalEntries.length) {
    return null;
  }

  return (
    <table class="u-full-width">
      <thead>
        <tr>
          <th>CUSIP</th>
          <th>Accounts</th>
          <th>Shares</th>
        </tr>
      </thead>
      <tbody>
        {totalEntries.map(([cusip, total]) => (
          <tr key={cusip}>
            <td>
              <NavLink
                to={`/transactions?cusip=${cusip}`}
                activeClassName="disabled"
              >
                {cusip}
              </NavLink>
            </td>
            <td>{total.accounts}</td>
            <td>{total.shares}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export default Totals;
