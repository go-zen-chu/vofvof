package main

import (
	"flag"
	"log/slog"
	"net/http"

	"github.com/go-zen-chu/vofvof/internal/member"
	"github.com/go-zen-chu/vofvof/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	flag.Parse()

	store := member.NewStore()
	srv := server.New(store)

	slog.Info("vofvof server starting", "addr", *addr)
	if err := http.ListenAndServe(*addr, srv); err != nil {
		slog.Error("server error", "err", err)
	}
}
