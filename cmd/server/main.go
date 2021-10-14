package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"

	"github.com/connyay/drclub/api"
	"github.com/connyay/drclub/store"
	"github.com/connyay/drclub/web"
)

var cli struct {
	Serve ServeCmd `cmd:"" default:"1" help:"Start server."`
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
	log := logrus.New()
	r := chi.NewRouter()
	r.Use(logging(log))
	r.Use(middleware.Recoverer)
	var (
		s   store.Store
		err error
	)
	switch {
	case cmd.DSN == "" || cmd.DSN == "memory":
		log.Println("Using memory store")
		s = store.NewMem()
	case strings.HasPrefix(cmd.DSN, "postgres://"):
		log.Println("Using postgres store")
		s, err = store.NewPG(cmd.DSN)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
	default:
		return fmt.Errorf("unknown database url %q", cmd.DSN)
	}

	r.Post("/api/transactions", api.TransactionCreateHandler(s, log))
	r.Get("/api/transactions", api.TransactionListHandler(s, log))
	r.Get("/api/totals", api.TotalsHandler(s, log))
	r.NotFound(web.AssetHandler)
	log.Printf("Listening on %s", cmd.Addr)
	return http.ListenAndServe(cmd.Addr, r)
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
