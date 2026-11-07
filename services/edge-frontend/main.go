package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type Config struct {
	Port           string
	Environment    string
	CoreBackendURL string
	HTTPTimeout    time.Duration
	CACertPath     string
}

type OrderRequest struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Amount    float64 `json:"amount"`
}

type ResponsePayload struct {
	Service   string      `json:"service"`
	Status    string      `json:"status"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
}

var (
	appConfig  Config
	httpClient *http.Client
)

func initClient() {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if appConfig.CACertPath != "" {
		caCert, err := os.ReadFile(appConfig.CACertPath)
		if err == nil {
			caCertPool := x509.NewCertPool()
			if caCertPool.AppendCertsFromPEM(caCert) {
				tlsConfig.RootCAs = caCertPool
				log.Printf("Loaded custom internal CA bundle from %s", appConfig.CACertPath)
			}
		} else {
			log.Printf("Custom CA not loaded (%v), using system trust store", err)
		}
	}

	httpClient = &http.Client{
		Timeout: appConfig.HTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig:     tlsConfig,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ResponsePayload{
		Service:   "edge-frontend",
		Status:    "UP",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]string{
			"cluster":     "fabric-edge-eks",
			"environment": appConfig.Environment,
		},
	})
}

func orderProxyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ResponsePayload{
			Service: "edge-frontend",
			Status:  "ERROR",
			Error:   "Method not allowed, use POST",
		})
		return
	}

	targetURL := fmt.Sprintf("%s/orders", appConfig.CoreBackendURL)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, targetURL, r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ResponsePayload{
			Service: "edge-frontend",
			Status:  "ERROR",
			Error:   fmt.Sprintf("Failed to construct backend request: %v", err),
		})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-By", "edge-frontend")

	resp, err := httpClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(ResponsePayload{
			Service: "edge-frontend",
			Status:  "DOWNSTREAM_ERROR",
			Error:   fmt.Sprintf("Failed calling Core NLB (%s): %v", targetURL, err),
		})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ResponsePayload{
			Service: "edge-frontend",
			Status:  "ERROR",
			Error:   "Failed reading downstream response",
		})
		return
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "production"
	}
	coreURL := os.Getenv("CORE_BACKEND_URL")
	if coreURL == "" {
		coreURL = "https://core.fabric.internal:443"
	}
	timeoutStr := os.Getenv("HTTP_TIMEOUT")
	timeout := 10 * time.Second
	if d, err := time.ParseDuration(timeoutStr); err == nil {
		timeout = d
	}
	caPath := os.Getenv("CA_CERT_PATH")

	appConfig = Config{
		Port:           port,
		Environment:    env,
		CoreBackendURL: coreURL,
		HTTPTimeout:    timeout,
		CACertPath:     caPath,
	}

	initClient()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/readyz", healthHandler)
	mux.HandleFunc("/api/v1/orders", orderProxyHandler)

	addr := fmt.Sprintf(":%s", appConfig.Port)
	log.Printf("Starting edge-frontend server on %s (Forwarding to Core: %s)", addr, appConfig.CoreBackendURL)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server terminated: %v", err)
	}
}