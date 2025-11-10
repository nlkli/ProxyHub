package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
)

var (
	PublicIPAddr string
	PubVars      map[string]string
)

const (
	defaultMode       = 1
	defaultDir        = "."
	defaultHost       = "0.0.0.0"
	defaultPort       = 8090
	defaultPoroto     = "http"
	defaultInfoPort   = 8091
	defaultRootPrefix = ""
)

type Config struct {
	Dir      string
	Host     string
	Port     int
	Prefix   string
	Proto    string
	KeyFile  string
	CertFile string
	InfoHost string
	InfoPort int
	Mode     int
}

func loadEnv() {
	if err := godotenv.Load(); err != nil {
		createDefaultEnv()
		log.Fatal("Error loading .env file")
	}
}

func createDefaultEnv() {
	envFile, err := os.Create(".env")
	if err != nil {
		log.Fatalf("Failed to create env file: %v", err)
	}
	defer envFile.Close()

	envFile.WriteString(`TELEGRAM_BOT_TOKEN=0
TELEGRAM_BOT_OWNER_ID=0
TELEGRAM_BOT_ACCESS_CODE=0`)
}

func getPublicIP() string {
	resp, err := http.Get("https://ifconfig.me/ip")
	if err != nil {
		log.Fatalf("Failed to get public IP: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read IP response: %v", err)
	}

	return strings.TrimSpace(string(data))
}

func parseFlags() Config {
	dir := flag.String("dir", defaultDir, "server directory")
	host := flag.String("host", defaultHost, "server host")
	port := flag.Int("port", defaultPort, "server port")
	prefix := flag.String("prefix", defaultRootPrefix, "server root prefix")
	proto := flag.String("proto", defaultPoroto, "server protocol http/https")
	keyFile := flag.String("skey", "server.key", "server key file")
	certFile := flag.String("scrt", "server.crt", "server cert file")
	infoHost := flag.String("ihost", defaultHost, "info server host")
	infoPort := flag.Int("iport", defaultInfoPort, "info server port")
	mode := flag.Int("mode", defaultMode, "mode")

	flag.Parse()

	return Config{
		Dir:      *dir,
		Host:     *host,
		Port:     *port,
		Prefix:   *prefix,
		Proto:    *proto,
		KeyFile:  *keyFile,
		CertFile: *certFile,
		InfoHost: *infoHost,
		InfoPort: *infoPort,
		Mode:     *mode,
	}
}

func getTelegramConfig() (string, string, string) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	ownerID := os.Getenv("TELEGRAM_BOT_OWNER_ID")
	accessCode := os.Getenv("TELEGRAM_BOT_ACCESS_CODE")

	if token == "" || ownerID == "" || accessCode == "" {
		log.Fatal("Telegram environment variables not set")
	}

	return token, ownerID, accessCode
}

func main() {
	loadEnv()

	PubVars = make(map[string]string)
	envVars := os.Environ()
	for _, env := range envVars {
		if strings.HasPrefix(env, "PUBVAR_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key, _ := strings.CutPrefix(parts[0], "PUBVAR_")
				value := parts[1]
				PubVars[key] = value
			}
		}
	}

	config := parseFlags()
	PublicIPAddr = getPublicIP()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go RunInfoServer(ctx, stop, &InfoServerParams{
		Host: config.InfoHost,
		Port: config.InfoPort,
	})

	if config.Mode > 1 {
		token, ownerID, accessCode := getTelegramConfig()

		go RunServer(ctx, stop, &ServerParams{
			Dir:     config.Dir,
			Host:    config.Host,
			Port:    config.Port,
			Proto:   config.Proto,
			KeyFile: config.KeyFile,
			CrtFile: config.CertFile,
			Prefix:  config.Prefix,
		})

		go RunTelebot(ctx, stop, &TelebotParams{
			Token:         token,
			OwnerID:       ownerID,
			AccessCode:    accessCode,
			WebApp:        "https://core.telegram.org/",
			UsersFilePath: "telebotusers.db",
		})
	}

	<-ctx.Done()
	log.Println("Application exited cleanly.")
}
