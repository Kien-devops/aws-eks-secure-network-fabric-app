package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Kien-devops/aws-eks-secure-network-fabric-app/services/core-service/cache"
	"github.com/Kien-devops/aws-eks-secure-network-fabric-app/services/core-service/db"
	"github.com/Kien-devops/aws-eks-secure-network-fabric-app/services/core-service/queue"
	"github.com/google/uuid"
)

type App struct {
	DB     *db.DBClient
	Cache  *cache.CacheClient
	SQS    *queue.SQSHandler
	Config Config
}

type Config struct {
	Port        string
	DBHost      string
	DBPort      string
	DBName      string
	DBUser      string
	DBPassword  string
	DBSSLMode   string
	RedisHost   string
	RedisPort   string
	RedisAuth   string
	RedisTLS    bool
	SQSQueueURL string
	AWSRegion   string
	TLSCertPath string
	TLSKeyPath  string
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

func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbStatus := "UP"
	if a.DB != nil {
		if err := a.DB.Pool.Ping(ctx); err != nil {
			dbStatus = fmt.Sprintf("DOWN: %v", err)
		}
	} else {
		dbStatus = "NOT_CONFIGURED"
	}

	redisStatus := "UP"
	if a.Cache != nil {
		if err := a.Cache.Client.Ping(ctx).Err(); err != nil {
			redisStatus = fmt.Sprintf("DOWN: %v", err)
		}
	} else {
		redisStatus = "NOT_CONFIGURED"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ResponsePayload{
		Service:   "core-service",
		Status:    "UP",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]string{
			"cluster":    "fabric-core-eks",
			"database":   dbStatus,
			"redis":      redisStatus,
			"sqs_target": a.Config.SQSQueueURL,
		},
	})
}

func (a *App) createOrderHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ResponsePayload{
			Service: "core-service",
			Status:  "ERROR",
			Error:   "Use POST method",
		})
		return
	}

	var req OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ResponsePayload{
			Service: "core-service",
			Status:  "INVALID_PAYLOAD",
			Error:   err.Error(),
		})
		return
	}

	orderID := "ord-" + uuid.New().String()[:8]
	orderRecord := db.OrderRecord{
		ID:        orderID,
		ProductID: req.ProductID,
		Quantity:  req.Quantity,
		Amount:    req.Amount,
		Status:    "PENDING_PAYMENT",
		CreatedAt: time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// 1. Lưu vào Aurora PostgreSQL
	if a.DB != nil {
		if err := a.DB.CreateOrder(ctx, orderRecord); err != nil {
			log.Printf("DB error: %v", err)
		}
	}

	// 2. Lưu Cache vào ElastiCache Redis
	if a.Cache != nil {
		payloadBytes, _ := json.Marshal(orderRecord)
		if err := a.Cache.SetOrderCache(ctx, orderID, payloadBytes, 10*time.Minute); err != nil {
			log.Printf("Redis cache error: %v", err)
		}
	}

	// 3. Đẩy OrderCreated event vào Amazon SQS
	if a.SQS != nil {
		msgBytes, _ := json.Marshal(orderRecord)
		if err := a.SQS.PublishOrderEvent(ctx, orderID, string(msgBytes)); err != nil {
			log.Printf("SQS error: %v", err)
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ResponsePayload{
		Service:   "core-service",
		Status:    "ORDER_CREATED",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      orderRecord,
	})
}

func main() {
	cfg := Config{
		Port:        getEnv("PORT", "8082"),
		DBHost:      getEnv("DB_HOST", "db.fabric.internal"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBName:      getEnv("DB_NAME", "fabric_db"),
		DBUser:      getEnv("DB_USER", "postgresadmin"),
		DBPassword:  os.Getenv("DB_PASSWORD"),
		DBSSLMode:   getEnv("DB_SSL_MODE", "prefer"),
		RedisHost:   getEnv("REDIS_HOST", "redis.fabric.internal"),
		RedisPort:   getEnv("REDIS_PORT", "6379"),
		RedisAuth:   os.Getenv("REDIS_AUTH"),
		RedisTLS:    getEnv("REDIS_TLS", "true") == "true",
		SQSQueueURL: getEnv("SQS_QUEUE_URL", "https://ap-southeast-1.queue.amazonaws.com/123456789012/fabric-order-events"),
		AWSRegion:   getEnv("AWS_REGION", "ap-southeast-1"),
		TLSCertPath: getEnv("TLS_CERT_PATH", "/etc/tls/tls.crt"),
		TLSKeyPath:  getEnv("TLS_KEY_PATH", "/etc/tls/tls.key"),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := &App{Config: cfg}

	// Kết nối Database nếu có password
	if cfg.DBPassword != "" {
		dbClient, err := db.ConnectPostgres(ctx, cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)
		if err == nil {
			app.DB = dbClient
			_ = dbClient.RunMigrations(ctx)
		} else {
			log.Printf("Warn: Failed connecting to Aurora DB: %v", err)
		}
	}

	// Kết nối Redis
	cacheClient, err := cache.ConnectRedis(ctx, cfg.RedisHost, cfg.RedisPort, cfg.RedisAuth, cfg.RedisTLS)
	if err == nil {
		app.Cache = cacheClient
	} else {
		log.Printf("Warn: Failed connecting to ElastiCache Redis: %v", err)
	}

	// Kết nối Amazon SQS (Pod Identity)
	sqsHandler, err := queue.NewSQSHandler(ctx, cfg.AWSRegion, cfg.SQSQueueURL)
	if err == nil {
		app.SQS = sqsHandler
		app.SQS.StartConsumer(ctx, "payment-worker-1")
	} else {
		log.Printf("Warn: Failed to initialize Amazon SQS: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", app.healthHandler)
	mux.HandleFunc("/readyz", app.healthHandler)
	mux.HandleFunc("/orders", app.createOrderHandler)

	addr := fmt.Sprintf(":%s", cfg.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server (HTTPS nếu có cert, hoặc HTTP cho test)
	go func() {
		if fileExists(cfg.TLSCertPath) && fileExists(cfg.TLSKeyPath) {
			log.Printf("Starting HTTPS server on %s with TLS certs (%s)", addr, cfg.TLSCertPath)
			if err := server.ListenAndServeTLS(cfg.TLSCertPath, cfg.TLSKeyPath); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTPS server failed: %v", err)
			}
		} else {
			log.Printf("Starting standard HTTP server on %s (TLS certificates not found)", addr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTP server failed: %v", err)
			}
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down core-service gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

