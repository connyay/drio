import React, { useCallback, useState } from "react";
import axios from "axios";
import { toast } from "react-toastify";

const uploadURL = `${process.env.REACT_APP_API || ""}/api/transactions`;

function Upload({ refresh } = { refresh: () => {} }) {
  const [renderKey, setRenderKey] = useState(1);
  const [file, setFile] = useState();
  const [loading, setLoading] = useState(false);

  const submitForm = useCallback(
    (event) => {
      event.preventDefault();
      if (!file) {
        return;
      }
      setLoading(true);
      axios
        .post(uploadURL, file, {
          headers: {
            "Content-Type": "application/pdf",
          },
        })
        .then((res) => {
          console.log("successfully imported");
          setRenderKey(Date.now());
          refresh();
          setFile(null);
          toast("Successfully uploaded");
        })
        .catch((err) => {
          console.error("failed import", err);
          toast.error(`Failed upload ${err}`);
        })
        .finally(() => setLoading(false));
    },
    [file, setFile, refresh]
  );

  return (
    <form onSubmit={submitForm}>
      <div className="row">
        <div className="six columns">
          <label>Transaction Document</label>
          <input
            type="file"
            onChange={(e) => setFile(e.target.files[0])}
            accept=".pdf"
            key={renderKey}
          />
        </div>
        <div className="six columns">
          {loading ? (
            <>Processing...</>
          ) : (
            <input
              className="button-primary"
              type="submit"
              value="Upload"
              disabled={!file}
            />
          )}
        </div>
      </div>
    </form>
  );
}

export default Upload;
