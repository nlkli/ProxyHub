package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

var serverFullExternalURL string

type ServerParams struct {
	Dir     string
	Host    string
	Port    int
	Proto   string
	KeyFile string
	CrtFile string
	Prefix  string
}

func RunServer(ctx context.Context, stop context.CancelFunc, params *ServerParams) {
	defer stop()

	mux := http.NewServeMux()

	mux.HandleFunc(params.Prefix+"/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	mux.HandleFunc(params.Prefix+"/proxyservers", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "proxyservers.json")
	})

	assetsPath := filepath.Join(params.Dir, "assets")
	fs := http.FileServer(http.Dir(assetsPath))
	mux.Handle(params.Prefix+"/assets/", http.StripPrefix(params.Prefix+"/assets/", fs))

	addr := fmt.Sprintf("%s:%d", params.Host, params.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("Server running [LOCAL] at http://127.0.0.1:%d%s\n", params.Port, params.Prefix)
	serverFullExternalURL = fmt.Sprintf("http://%s:%d%s", PublicIPAddr, params.Port, params.Prefix)
	log.Printf("Server running [GLOBAL] at %s\n", serverFullExternalURL)

	go func() {
		<-ctx.Done()
		log.Println("Shutting down HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	var err error
	if params.Proto == "https" {
		err = server.ListenAndServeTLS(params.CrtFile, params.KeyFile)
	} else {
		err = server.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}

	log.Println("HTTP server stopped gracefully.")
}
