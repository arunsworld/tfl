package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/arunsworld/tfl/handlers"
	"github.com/arunsworld/tfl/webserver"
	"github.com/gorilla/mux"
)

//go:embed embed/*
var webContent embed.FS

func main() {
	port := flag.Int("port", 4934, "port to run watermill client on")
	flag.Parse()

	if err := start(*port); err != nil {
		log.Fatal(err)
	}
}

func start(port int) error {
	shutdownCtx, shutdown := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer shutdown()

	handler := mux.NewRouter()
	handlers.RegisterHandlers(handler, mustFSSub(webContent, "embed/static"), mustFSSub(webContent, "embed/html"))

	if err := webserver.NewHTTPWebServer(handler).Serve(shutdownCtx, port); err != nil {
		return err
	}

	return nil
}

func mustFSSub(src fs.FS, dir string) fs.FS {
	fsys, err := fs.Sub(src, dir)
	if err != nil {
		panic(err)
	}
	return fsys
}
