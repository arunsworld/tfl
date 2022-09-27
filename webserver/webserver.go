package webserver

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

func NewHTTPWebServer(handler http.Handler) *httpWebServer {
	return &httpWebServer{
		handler: handler,
	}
}

type httpWebServer struct {
	handler http.Handler
}

func (w *httpWebServer) Serve(ctx context.Context, port int) error {
	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:    addr,
		Handler: w.handler,
	}
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server start error: %v", err)
			errCh <- err
		}
	}()
	log.Printf("Serving on URL: http://localhost:%d/", port)
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("initiating graceful shutdown of server...")
		ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctxShutDown); err != nil {
			log.Printf("error during graceful shutdown: %v", err)
		}
		return nil
	}
}
