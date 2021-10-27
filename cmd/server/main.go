package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"

	"github.com/connyay/drio/api"
	"github.com/connyay/drio/store"
	"github.com/connyay/drio/web"
)

var cli struct {
	Serve ServeCmd `cmd:"" default:"1" help:"Start server."`
	Reset ResetCmd `cmd:"" help:"Reset database."`
}

func main() {
	ctx := kong.Parse(&cli)
	ctx.FatalIfErrorf(ctx.Run())
}

type ServeCmd struct {
	Addr string `help:"Address to listen on." default:"0.0.0.0:8080" env:"LISTEN_ADDR"`
	DSN  string `help:"DSN to use for backing store." env:"DATABASE_URL"`
}

func (cmd *ServeCmd) Run() error {
	store, err := getStore(cmd.DSN)
	if err != nil {
		return err
	}
	log := logrus.New()
	r := chi.NewRouter()
	r.Use(logging(log))
	r.Use(middleware.Recoverer)
	r.Post("/api/transactions", api.TransactionCreateHandler(store, log))
	r.Get("/api/transactions", api.TransactionListHandler(store, log))
	r.Get("/api/totals", api.TotalsHandler(store, log))
	r.NotFound(func(rw http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet &&
			(r.RequestURI == "/" ||
				r.RequestURI == "favicon.ico" ||
				strings.HasPrefix(r.RequestURI, "/static/") ||
				strings.HasPrefix(r.RequestURI, "/transactions")) {
			web.AssetHandler(rw, r)
			return
		}
		rw.WriteHeader(http.StatusNotFound)
	})
	log.Printf("Listening on %s", cmd.Addr)
	return http.ListenAndServe(cmd.Addr, r)
}

type ResetCmd struct {
	DSN string `help:"DSN to use for backing store." env:"DATABASE_URL"`
}

func (cmd *ResetCmd) Run() error {
	store, err := getStore(cmd.DSN)
	if err != nil {
		return err
	}
	err = store.Reset()
	if err != nil {
		return err
	}
	return nil
}

func getStore(dsn string) (store.Store, error) {
	switch {
	case dsn == "" || dsn == "memory":
		log.Println("Using memory store")
		return store.NewMem(), nil
	case strings.HasPrefix(dsn, "postgres://"):
		log.Println("Using postgres store")
		s, err := store.NewPG(dsn)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unknown database url %q", dsn)
	}
}

func logging(logger logrus.FieldLogger) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t1 := time.Now()
			defer func() {
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				logger.WithFields(logrus.Fields{
					"status_code":      ww.Status(),
					"bytes":            ww.BytesWritten(),
					"duration":         int64(time.Since(t1)),
					"duration_display": time.Since(t1).String(),
					"proto":            r.Proto,
					"method":           r.Method,
				}).Infof("%s://%s%s", scheme, r.Host, r.RequestURI)
			}()

			h.ServeHTTP(ww, r)
		}

		return http.HandlerFunc(fn)
	}
}
