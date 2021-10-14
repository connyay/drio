package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/connyay/drclub/cs"
	"github.com/connyay/drclub/store"
	"github.com/sirupsen/logrus"
)

func TransactionCreateHandler(db store.Store, log *logrus.Logger) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if ct := req.Header.Get("Content-Type"); ct != "application/pdf" {
			http.Error(rw, "invalid content-type", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		docBytes, err := io.ReadAll(io.LimitReader(req.Body, 200*1024))
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
			http.Error(rw, "failed insert", http.StatusBadRequest)
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
			log.Printf("failed get all %v", err)
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
		transactions, err := db.GetAllTransactions()
		if err != nil {
			log.Printf("failed get all %v", err)
			http.Error(rw, "", http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(rw).Encode(map[string][]store.Transaction{
			"transactions": transactions,
		})
		if err != nil {
			log.Printf("err %v", err)
		}
	}
}