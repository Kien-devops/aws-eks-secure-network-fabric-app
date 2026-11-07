package main

import (
	"context"
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
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/google/uuid"
)

type App struct {
	DB     *db.DBClient
	Cache  *cache.CacheClient
	SQS    *queue.SQSHandler
	Config Config
}

type Config struct {
	Port               string
	DBHost             string
	DBPort             string
	DBName             string
	DBUser             string
	DBPassword         string
	DBSecretID         string
	DBSSLMode          string
	RedisHost          string
	RedisPort          string
	RedisAuth          string
	RedisSecretID      string
	RedisTLS           bool
	RedisTLSServerName string
	SQSQueueURL        string
	AWSRegion          string
	TLSCertPath        string
	TLSKeyPath         string
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

func (a *App) ordersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
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

		dbStatus := "SAVED"
		// 1. Lưu vào Aurora PostgreSQL
		if a.DB != nil {
			if err := a.DB.CreateOrder(ctx, orderRecord); err != nil {
				log.Printf("DB error: %v", err)
				dbStatus = fmt.Sprintf("ERROR: %v", err)
			}
		} else {
			dbStatus = "NOT_CONFIGURED"
		}

		cacheStatus := "CACHED"
		// 2. Lưu Cache vào ElastiCache Redis
		if a.Cache != nil {
			payloadBytes, _ := json.Marshal(orderRecord)
			if err := a.Cache.SetOrderCache(ctx, orderID, payloadBytes, 10*time.Minute); err != nil {
				log.Printf("Redis cache error: %v", err)
				cacheStatus = fmt.Sprintf("ERROR: %v", err)
			}
		} else {
			cacheStatus = "NOT_CONFIGURED"
		}

		sqsStatus := "PUBLISHED"
		// 3. Đẩy OrderCreated event vào Amazon SQS
		if a.SQS != nil {
			msgBytes, _ := json.Marshal(orderRecord)
			if err := a.SQS.PublishOrderEvent(ctx, orderID, string(msgBytes)); err != nil {
				log.Printf("SQS error: %v", err)
				sqsStatus = fmt.Sprintf("ERROR: %v", err)
			}
		} else {
			sqsStatus = "NOT_CONFIGURED"
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ResponsePayload{
			Service:   "core-service",
			Status:    "ORDER_CREATED",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Data: map[string]interface{}{
				"order":        orderRecord,
				"database":     dbStatus,
				"redis_cache":  cacheStatus,
				"sqs_event":    sqsStatus,
				"cluster":      "fabric-core-eks",
			},
		})

	case http.MethodGet:
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if a.DB == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(ResponsePayload{
				Service: "core-service",
				Status:  "ERROR",
				Error:   "Database not connected",
			})
			return
		}

		orders, err := a.DB.ListOrders(ctx, 20)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ResponsePayload{
				Service: "core-service",
				Status:  "ERROR",
				Error:   err.Error(),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ResponsePayload{
			Service:   "core-service",
			Status:    "SUCCESS",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Data: map[string]interface{}{
				"count":  len(orders),
				"orders": orders,
			},
		})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ResponsePayload{
			Service: "core-service",
			Status:  "ERROR",
			Error:   "Method not allowed (Use GET or POST)",
		})
	}
}

func (a *App) getOrderByIDHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	orderID := r.URL.Path[len("/orders/"):]
	if orderID == "" {
		a.ordersHandler(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Step 1: Check Redis cache first (Fast path - Cache Hit)
	if a.Cache != nil {
		cachedStr, err := a.Cache.GetOrderCache(ctx, orderID)
		if err == nil && cachedStr != "" {
			var cachedRecord db.OrderRecord
			if err := json.Unmarshal([]byte(cachedStr), &cachedRecord); err == nil {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(ResponsePayload{
					Service:   "core-service",
					Status:    "ORDER_RETRIEVED",
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Data: map[string]interface{}{
						"order":        cachedRecord,
						"cache_source": "REDIS_CACHE_HIT",
						"cluster":      "fabric-core-eks",
					},
				})
				return
			}
		}
	}

	// Step 2: Fallback to Aurora PostgreSQL (DB Hit)
	if a.DB != nil {
		order, err := a.DB.GetOrder(ctx, orderID)
		if err == nil && order != nil {
			// Populate cache for subsequent requests
			if a.Cache != nil {
				payloadBytes, _ := json.Marshal(order)
				_ = a.Cache.SetOrderCache(ctx, orderID, payloadBytes, 10*time.Minute)
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ResponsePayload{
				Service:   "core-service",
				Status:    "ORDER_RETRIEVED",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Data: map[string]interface{}{
					"order":        order,
					"cache_source": "AURORA_DB_HIT (Cache populated)",
					"cluster":      "fabric-core-eks",
				},
			})
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(ResponsePayload{
		Service: "core-service",
		Status:  "NOT_FOUND",
		Error:   fmt.Sprintf("Order [%s] not found in Cache or Database", orderID),
	})
}

func (a *App) systemStatusHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbPing := "OK"
	if a.DB != nil {
		if err := a.DB.Pool.Ping(ctx); err != nil {
			dbPing = fmt.Sprintf("FAILED: %v", err)
		}
	} else {
		dbPing = "NOT_CONNECTED"
	}

	redisPing := "OK"
	if a.Cache != nil {
		if err := a.Cache.Client.Ping(ctx).Err(); err != nil {
			redisPing = fmt.Sprintf("FAILED: %v", err)
		}
	} else {
		redisPing = "NOT_CONNECTED"
	}

	hostname, _ := os.Hostname()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ResponsePayload{
		Service:   "core-service",
		Status:    "UP",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]interface{}{
			"cluster":           "fabric-core-eks",
			"pod_hostname":      hostname,
			"aurora_postgresql": map[string]string{
				"host":   a.Config.DBHost,
				"status": dbPing,
			},
			"elasticache_redis": map[string]interface{}{
				"host":     a.Config.RedisHost,
				"status":   redisPing,
				"tls":      a.Config.RedisTLS,
			},
			"amazon_sqs": map[string]string{
				"queue_url": a.Config.SQSQueueURL,
				"status":    "ACTIVE",
				"worker":    "payment-worker-1",
			},
		},
	})
}

func main() {
	cfg := Config{
		Port:               getEnv("PORT", "8082"),
		DBHost:             getEnv("DB_HOST", "db.fabric.internal"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBName:             getEnv("DB_NAME", "postgres"),
		DBUser:             getEnv("DB_USER", "postgresadmin"),
		DBPassword:         os.Getenv("DB_PASSWORD"),
		DBSecretID:         getEnv("DB_SECRET_ID", "rds!cluster-62aa262a-3209-417c-95ed-d72af68465ac"),
		DBSSLMode:          getEnv("DB_SSL_MODE", "prefer"),
		RedisHost:          getEnv("REDIS_HOST", "redis.fabric.internal"),
		RedisPort:          getEnv("REDIS_PORT", "6379"),
		RedisAuth:          os.Getenv("REDIS_AUTH"),
		RedisSecretID:      getEnv("REDIS_SECRET_ID", "fabric-core-redis-auth"),
		RedisTLS:           getEnv("REDIS_TLS", "true") == "true",
		RedisTLSServerName: getEnv("REDIS_TLS_SERVER_NAME", "master.fabric-core-redis.qspmd1.apse1.cache.amazonaws.com"),
		SQSQueueURL:        getEnv("SQS_QUEUE_URL", "https://ap-southeast-1.queue.amazonaws.com/620306387033/fabric-order-events"),
		AWSRegion:          getEnv("AWS_REGION", "ap-southeast-1"),
		TLSCertPath:        getEnv("TLS_CERT_PATH", "/etc/tls/tls.crt"),
		TLSKeyPath:         getEnv("TLS_KEY_PATH", "/etc/tls/tls.key"),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Tự động lấy DB password từ AWS Secrets Manager qua Pod Identity nếu chưa có trong env
	if cfg.DBPassword == "" && cfg.DBSecretID != "" {
		raw, err := fetchSecretFromAWS(ctx, cfg.AWSRegion, cfg.DBSecretID)
		if err == nil {
			var parsed struct {
				Password string `json:"password"`
			}
			if err := json.Unmarshal([]byte(raw), &parsed); err == nil && parsed.Password != "" {
				cfg.DBPassword = parsed.Password
				log.Printf("Successfully retrieved DB password from AWS Secrets Manager (%s) via Pod Identity", cfg.DBSecretID)
			}
		} else {
			log.Printf("Warn: Failed fetching DB secret (%s) via Pod Identity: %v", cfg.DBSecretID, err)
		}
	}

	// Tự động lấy Redis auth token từ AWS Secrets Manager qua Pod Identity nếu chưa có trong env
	if cfg.RedisAuth == "" && cfg.RedisSecretID != "" {
		raw, err := fetchSecretFromAWS(ctx, cfg.AWSRegion, cfg.RedisSecretID)
		if err == nil {
			var parsed struct {
				AuthToken string `json:"auth_token"`
			}
			if err := json.Unmarshal([]byte(raw), &parsed); err == nil && parsed.AuthToken != "" {
				cfg.RedisAuth = parsed.AuthToken
				log.Printf("Successfully retrieved Redis auth token from AWS Secrets Manager (%s) via Pod Identity", cfg.RedisSecretID)
			} else {
				cfg.RedisAuth = raw
			}
		} else {
			log.Printf("Warn: Failed fetching Redis secret (%s) via Pod Identity: %v", cfg.RedisSecretID, err)
		}
	}

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
	cacheClient, err := cache.ConnectRedis(ctx, cfg.RedisHost, cfg.RedisPort, cfg.RedisAuth, cfg.RedisTLS, cfg.RedisTLSServerName)
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
	mux.HandleFunc("/orders", app.ordersHandler)
	mux.HandleFunc("/orders/", app.getOrderByIDHandler)
	mux.HandleFunc("/system-status", app.systemStatusHandler)

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

func fetchSecretFromAWS(ctx context.Context, region, secretID string) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config for secrets: %w", err)
	}
	client := secretsmanager.NewFromConfig(cfg)
	out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", secretID, err)
	}
	if out.SecretString == nil {
		return "", fmt.Errorf("secret %s has empty SecretString", secretID)
	}
	return *out.SecretString, nil
}