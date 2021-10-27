import React, { useCallback, useState } from "react";
import { BrowserRouter as Router, Route } from "react-router-dom";
import { ToastContainer } from "react-toastify";
import "react-toastify/dist/ReactToastify.css";

import Upload from "./UploadTransaction";
import Totals from "./Totals";
import Transactions from "./Transactions";

function App() {
  const [refreshKey, setRefresh] = useState(0);
  const refresh = useCallback(() => setRefresh(Date.now()), [setRefresh]);
  return (
    <Router>
      <div className="container">
        <div className="row" style={{ marginTop: "10%" }}>
          <div className="two-thirds column">
            <h4>Direct Registration</h4>
            <p>
              Self-reported transactions and positions. This is for
              informational purposes only and is not financial advice. Conduct
              your own due diligence, or consult a licensed financial advisor or
              broker before making any and all investment decisions.
            </p>
            <p>
              <a
                href="https://github.com/connyay/drio#drio"
                title="connyay/drio on github"
              >
                Read more about this project and review the source on github.
              </a>
            </p>
          </div>
        </div>
        <div className="row" style={{ marginBottom: "15px" }}>
          <Upload refresh={refresh} />
        </div>
        <Totals key={`${refreshKey}totals`} />
        <Route
          path="/transactions"
          component={Transactions}
          key={`${refreshKey}transactions`}
        />
      </div>
      <ToastContainer />
    </Router>
  );
}

export default App;
