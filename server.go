package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var serverFullExternalURL string
var proxyServersInfo []*ProxyServerInfo

type ServerParams struct {
	Dir     string
	Host    string
	Port    int
	Proto   string
	KeyFile string
	CrtFile string
	Prefix  string
}

type ProxyServerInfo struct {
	Name         string     `json:"name"`
	ID           string     `json:"id"`
	Location     string     `json:"location"`
	ProviderName string     `json:"providerName"`
	ProviderLink string     `json:"providerLink"`
	Plan         string     `json:"plan"`
	SpeedRate    string     `json:"speedRate"`
	Limit        string     `json:"limit"`
	InfoLink     string     `json:"infoLink"`
	ProxyLinks   ProxyLinks `json:"proxyLinks"`
}

type ProxyLinks struct {
	Vless []string `json:"vless"`
	HTTP  []string `json:"http"`
	Socks []string `json:"socks"`
}

func serverInfoHandle(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "url parameter is required", http.StatusBadRequest)
		return
	}

	hasServer := slices.ContainsFunc(proxyServersInfo, func(e *ProxyServerInfo) bool {
		return strings.HasPrefix(url, e.InfoLink)
	})

	if !hasServer {
		http.Error(w, "server not found", http.StatusBadRequest)
		return
	}

	client := &http.Client{
		Timeout: 4 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		if os.IsTimeout(err) {
			http.Error(w, "Request timeout", http.StatusGatewayTimeout)
		} else {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error copying response: %v", err)
	}
}

func RunServer(ctx context.Context, stop context.CancelFunc, params *ServerParams) {
	defer stop()

	if _, err := os.Stat("proxyservers.json"); os.IsNotExist(err) {
		log.Fatal("proxyservers.json not found")
	}

	content, err := os.ReadFile("proxyservers.json")
	if err != nil {
		log.Fatalf("read proxyservers.json error: %v", err)
	}

	err = json.Unmarshal(content, &proxyServersInfo)
	if err != nil {
		log.Fatalf("unmarshal proxyservers.json error: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc(params.Prefix+"/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != params.Prefix+"/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		http.ServeFile(w, r, "index.html")
	})

	mux.HandleFunc(params.Prefix+"/serverinfo/", serverInfoHandle)

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

	var initErr error
	if params.Proto == "https" {
		initErr = server.ListenAndServeTLS(params.CrtFile, params.KeyFile)
	} else {
		initErr = server.ListenAndServe()
	}

	if initErr != nil && initErr != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", initErr)
	}

	log.Println("HTTP server stopped gracefully.")
}
