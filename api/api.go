package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/connyay/drio/cs"
	"github.com/connyay/drio/store"
	"github.com/sirupsen/logrus"
)

const (
	_maxDocumentSizeBytes = 300 * 1024
)

func TransactionCreateHandler(db store.Store, log *logrus.Logger) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if ct := req.Header.Get("Content-Type"); ct != "application/pdf" {
			http.Error(rw, "invalid content-type", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		docBytes, err := io.ReadAll(io.LimitReader(req.Body, _maxDocumentSizeBytes))
		if err != nil {
			log.WithFields(logrus.Fields{
				"err":        err,
				"request_ip": req.RemoteAddr,
			}).Error("failed reading body")
			http.Error(rw, "", http.StatusInternalServerError)
			return
		}

		tx, err := cs.Parse(docBytes)
		if err != nil {
			log.WithFields(logrus.Fields{
				"err":        err,
				"request_ip": req.RemoteAddr,
			}).Error("failed parse")
			http.Error(rw, "failed parse", http.StatusBadRequest)
			return
		}

		requester := fmt.Sprintf("%x", sha256.Sum256([]byte(req.RemoteAddr)))
		storedTx := tx.ToStore(requester)

		err = db.InsertTransaction(storedTx)
		if err != nil {
			log.WithFields(logrus.Fields{
				"err":        err,
				"request_ip": req.RemoteAddr,
			}).Error("failed inserting transaction")
			http.Error(rw, "failed inserting transaction", http.StatusBadRequest)
			return
		}

		err = db.SetPosition(store.Position{
			AccountIDHash: storedTx.AccountIDHash,
			CUSIP:         tx.CUSIP,
			Date:          tx.Date,
			Total:         tx.ClosePosition,
		})
		if err != nil {
			log.WithFields(logrus.Fields{
				"err": err,
			}).Error("failed setting position")

			// note that this is not fatal
			// FIXME(cjh) this is a bit smelly. Should return a special err to
			// eat when the position is a dupe.
		}
		err = json.NewEncoder(rw).Encode(map[string]interface{}{
			"transaction": storedTx,
		})
		if err != nil {
			log.WithFields(logrus.Fields{
				"err": err,
			}).Error("encoding json body")
		}
	}
}

func TotalsHandler(db store.Store, log *logrus.Logger) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		totals, err := db.GetTotals()
		if err != nil {
			log.WithFields(logrus.Fields{
				"err": err,
			}).Error("get totals")
			http.Error(rw, "", http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(rw).Encode(totals)
		if err != nil {
			log.WithFields(logrus.Fields{
				"err": err,
			}).Error("encoding json body")
		}
	}
}

func TransactionListHandler(db store.Store, log *logrus.Logger) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		cusip := req.URL.Query().Get("cusip")
		if cusip == "" {
			http.Error(rw, "missing cusip", http.StatusBadRequest)
			return
		}
		transactions, err := db.GetTransactions(cusip)
		if err != nil {
			log.WithFields(logrus.Fields{
				"err": err,
			}).Error("get transactions")
			http.Error(rw, "", http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(rw).Encode(map[string][]store.Transaction{
			"transactions": transactions,
		})
		if err != nil {
			log.WithFields(logrus.Fields{
				"err": err,
			}).Error("encoding json body")
		}
	}
}
