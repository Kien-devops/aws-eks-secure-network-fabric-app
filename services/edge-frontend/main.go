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
	"strings"
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

func systemStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	targetURL := fmt.Sprintf("%s/system-status", appConfig.CoreBackendURL)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, targetURL, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ResponsePayload{
			Service: "edge-frontend",
			Status:  "ERROR",
			Error:   fmt.Sprintf("Failed to construct backend request: %v", err),
		})
		return
	}
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

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func orderProxyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	subPath := strings.TrimPrefix(r.URL.Path, "/api/v1/orders")
	targetURL := fmt.Sprintf("%s/orders%s", appConfig.CoreBackendURL, subPath)

	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
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

func dashboardHTMLHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/dashboard" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(htmlDashboardTemplate))
}

const htmlDashboardTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>AWS EKS Secure Network Fabric — Operational Dashboard</title>
  <style>
    :root {
      --bg: #0b0f19;
      --card-bg: #151d30;
      --border: #23314f;
      --accent: #38bdf8;
      --accent-hover: #0ea5e9;
      --text: #f1f5f9;
      --text-muted: #94a3b8;
      --success: #10b981;
      --warning: #f59e0b;
      --danger: #ef4444;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
    body { background-color: var(--bg); color: var(--text); padding: 24px; }
    .container { max-width: 1200px; margin: 0 auto; }
    header { display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid var(--border); padding-bottom: 16px; margin-bottom: 24px; }
    h1 { font-size: 1.5rem; color: #fff; display: flex; align-items: center; gap: 8px; }
    .badge { font-size: 0.75rem; padding: 4px 8px; border-radius: 9999px; background: rgba(16, 185, 129, 0.2); color: var(--success); font-weight: 600; border: 1px solid rgba(16, 185, 129, 0.3); }
    .diagram-card { background: var(--card-bg); border: 1px solid var(--border); border-radius: 12px; padding: 20px; margin-bottom: 24px; }
    .flow-steps { display: flex; align-items: center; justify-content: space-between; overflow-x: auto; gap: 12px; padding: 12px 0; }
    .flow-node { background: #1c2742; border: 1px solid var(--border); border-radius: 8px; padding: 10px 14px; text-align: center; min-width: 130px; font-size: 0.85rem; font-weight: 500; }
    .flow-node .label { font-size: 0.7rem; color: var(--text-muted); text-transform: uppercase; margin-bottom: 4px; }
    .flow-arrow { color: var(--accent); font-weight: bold; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 20px; margin-bottom: 24px; }
    .card { background: var(--card-bg); border: 1px solid var(--border); border-radius: 12px; padding: 20px; }
    .card-title { font-size: 1.1rem; font-weight: 600; margin-bottom: 16px; display: flex; align-items: center; justify-content: space-between; }
    .status-item { display: flex; justify-content: space-between; align-items: center; padding: 10px 0; border-bottom: 1px solid rgba(255,255,255,0.05); font-size: 0.9rem; }
    .status-item:last-child { border-bottom: none; }
    .status-pill { padding: 2px 8px; border-radius: 4px; font-size: 0.75rem; font-weight: 600; }
    .pill-green { background: rgba(16, 185, 129, 0.2); color: var(--success); }
    .pill-blue { background: rgba(56, 189, 248, 0.2); color: var(--accent); }
    .form-group { margin-bottom: 14px; }
    label { display: block; font-size: 0.8rem; color: var(--text-muted); margin-bottom: 6px; }
    input, select { width: 100%; background: #0f1524; border: 1px solid var(--border); color: #fff; padding: 10px; border-radius: 6px; font-size: 0.9rem; }
    input:focus, select:focus { outline: none; border-color: var(--accent); }
    button { width: 100%; background: var(--accent); color: #000; border: none; padding: 12px; border-radius: 6px; font-weight: 600; cursor: pointer; transition: 0.2s; font-size: 0.95rem; }
    button:hover { background: var(--accent-hover); }
    button.secondary { background: #23314f; color: #fff; margin-top: 8px; }
    button.secondary:hover { background: #32446d; }
    .result-box { background: #080c14; border: 1px solid var(--border); border-radius: 6px; padding: 12px; font-family: monospace; font-size: 0.85rem; color: #38bdf8; max-height: 220px; overflow-y: auto; margin-top: 14px; white-space: pre-wrap; }
    table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
    th, td { text-align: left; padding: 10px; border-bottom: 1px solid var(--border); }
    th { color: var(--text-muted); font-weight: 600; }
    tr:hover { background: rgba(255,255,255,0.02); }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div>
        <h1>AWS EKS Secure Network Fabric <span class="badge">LIVE • AP-SOUTHEAST-1</span></h1>
        <p style="color: var(--text-muted); font-size: 0.85rem; margin-top: 4px;">Zero-Trust Dual EKS Clusters with East-West GWLB Inspection & Centralized Egress Firewall</p>
      </div>
      <div>
        <button class="secondary" style="width: auto; padding: 8px 16px;" onclick="refreshStatus()">Refresh Status</button>
      </div>
    </header>

    <div class="diagram-card">
      <div style="font-size: 0.85rem; font-weight: 600; color: var(--text-muted); margin-bottom: 8px;">END-TO-END FLOW TRAVERSAL MAP</div>
      <div class="flow-steps">
        <div class="flow-node"><div class="label">Client</div>Internet</div>
        <div class="flow-arrow">→</div>
        <div class="flow-node"><div class="label">Ingress L7</div>AWS WAF & ALB</div>
        <div class="flow-arrow">→</div>
        <div class="flow-node" style="border-color: var(--accent);"><div class="label">Edge VPC</div>frontend (EKS)</div>
        <div class="flow-arrow">→</div>
        <div class="flow-node" style="border-color: #a855f7;"><div class="label">TGW Inspection</div>GWLB + Suricata</div>
        <div class="flow-arrow">→</div>
        <div class="flow-node"><div class="label">Core VPC</div>Internal NLB</div>
        <div class="flow-arrow">→</div>
        <div class="flow-node" style="border-color: var(--success);"><div class="label">Core Backend</div>order-service (EKS)</div>
        <div class="flow-arrow">→</div>
        <div class="flow-node"><div class="label">Data Tier</div>RDS + Redis + SQS</div>
      </div>
    </div>

    <div class="grid">
      <!-- Card 1: System Telemetry -->
      <div class="card">
        <div class="card-title">
          <span>Infrastructure Telemetry</span>
          <span class="status-pill pill-green">ACTIVE</span>
        </div>
        <div class="status-item">
          <span>Edge Cluster:</span>
          <span style="font-family: monospace; color: var(--accent);">fabric-edge-eks</span>
        </div>
        <div class="status-item">
          <span>Core Cluster:</span>
          <span style="font-family: monospace; color: var(--accent);">fabric-core-eks</span>
        </div>
        <div class="status-item">
          <span>Suricata IDS/IPS:</span>
          <span class="status-pill pill-blue">GENEVE In-Line</span>
        </div>
        <div class="status-item">
          <span>Aurora PostgreSQL:</span>
          <span id="db-status" class="status-pill pill-green">Connected</span>
        </div>
        <div class="status-item">
          <span>ElastiCache Redis:</span>
          <span id="redis-status" class="status-pill pill-green">TLS 1.2 Connected</span>
        </div>
        <div class="status-item">
          <span>Amazon SQS Queue:</span>
          <span id="sqs-status" class="status-pill pill-green">fabric-order-events</span>
        </div>
      </div>

      <!-- Card 2: Place Order Form -->
      <div class="card">
        <div class="card-title">
          <span>Create Order (Test Fabric)</span>
        </div>
        <form id="orderForm" onsubmit="submitOrder(event)">
          <div class="form-group">
            <label>Product Item</label>
            <select id="product_id">
              <option value="prod-server-blade-x">AWS EKS Server Blade X1</option>
              <option value="prod-suricata-sensor">Suricata 10Gbps IDS Sensor</option>
              <option value="prod-cloud-hsm">CloudHSM Key Management Token</option>
              <option value="prod-firewall-license">AWS Network Firewall Capacity License</option>
            </select>
          </div>
          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
            <div class="form-group">
              <label>Quantity</label>
              <input type="number" id="quantity" value="1" min="1" max="100" required>
            </div>
            <div class="form-group">
              <label>Amount ($)</label>
              <input type="number" id="amount" value="1299.00" step="0.01" required>
            </div>
          </div>
          <button type="submit" id="submitBtn">Submit Order to Fabric</button>
        </form>
        <div id="orderResult" class="result-box" style="display: none;"></div>
      </div>

      <!-- Card 3: Cache Verification -->
      <div class="card">
        <div class="card-title">
          <span>Cache Hit vs DB Verification</span>
        </div>
        <p style="font-size: 0.85rem; color: var(--text-muted); margin-bottom: 12px;">
          Retrieve an order by ID to demonstrate ElastiCache Redis cache hits versus Aurora PostgreSQL database fallback.
        </p>
        <div class="form-group">
          <label>Order ID</label>
          <input type="text" id="query_order_id" placeholder="e.g. ord-dc72a97e">
        </div>
        <button class="secondary" onclick="lookupOrder()">Check Cache / Fetch Order</button>
        <div id="lookupResult" class="result-box" style="display: none;"></div>
      </div>
    </div>

    <!-- Recent Orders Table -->
    <div class="card">
      <div class="card-title">
        <span>Recent Orders (Persisted in Aurora PostgreSQL & Cached in Redis)</span>
        <button class="secondary" style="width: auto; padding: 4px 12px; font-size: 0.8rem;" onclick="loadRecentOrders()">Reload</button>
      </div>
      <div style="overflow-x: auto;">
        <table>
          <thead>
            <tr>
              <th>Order ID</th>
              <th>Product</th>
              <th>Qty</th>
              <th>Amount</th>
              <th>Status</th>
              <th>Created At</th>
            </tr>
          </thead>
          <tbody id="ordersTableBody">
            <tr><td colspan="6" style="text-align: center; color: var(--text-muted);">Loading recent orders...</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>

  <script>
    async function refreshStatus() {
      try {
        const res = await fetch('/api/v1/status');
        const json = await res.json();
        if (json.data) {
          document.getElementById('db-status').innerText = json.data.aurora_postgresql.status;
          document.getElementById('redis-status').innerText = json.data.elasticache_redis.status;
          document.getElementById('sqs-status').innerText = json.data.amazon_sqs.status + ' (Worker Active)';
        }
      } catch (e) {
        console.error('Failed fetching status:', e);
      }
    }

    async function submitOrder(e) {
      e.preventDefault();
      const btn = document.getElementById('submitBtn');
      const box = document.getElementById('orderResult');
      btn.disabled = true;
      btn.innerText = 'Routing via GWLB & Suricata...';
      box.style.display = 'block';
      box.innerText = 'Transmitting order through AWS Network Fabric...';

      const payload = {
        product_id: document.getElementById('product_id').value,
        quantity: parseInt(document.getElementById('quantity').value, 10),
        amount: parseFloat(document.getElementById('amount').value)
      };

      try {
        const res = await fetch('/api/v1/orders', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        const data = await res.json();
        box.innerText = JSON.stringify(data, null, 2);
        loadRecentOrders();
      } catch (err) {
        box.innerText = 'Error: ' + err.message;
      } finally {
        btn.disabled = false;
        btn.innerText = 'Submit Order to Fabric';
      }
    }

    async function lookupOrder() {
      const id = document.getElementById('query_order_id').value.trim();
      if (!id) return;
      const box = document.getElementById('lookupResult');
      box.style.display = 'block';
      box.innerText = 'Querying Redis Cache & Aurora DB...';

      try {
        const res = await fetch('/api/v1/orders/' + encodeURIComponent(id));
        const data = await res.json();
        box.innerText = JSON.stringify(data, null, 2);
      } catch (err) {
        box.innerText = 'Error: ' + err.message;
      }
    }

    async function loadRecentOrders() {
      const tbody = document.getElementById('ordersTableBody');
      try {
        const res = await fetch('/api/v1/orders');
        const json = await res.json();
        if (json.data && json.data.orders && json.data.orders.length > 0) {
          tbody.innerHTML = json.data.orders.map(o => `
            <tr>
              <td style="font-family: monospace; color: var(--accent); font-weight: 600;">${o.id}</td>
              <td>${o.product_id}</td>
              <td>${o.quantity}</td>
              <td>$${Number(o.amount).toFixed(2)}</td>
              <td><span class="status-pill pill-green">${o.status}</span></td>
              <td style="color: var(--text-muted); font-size: 0.8rem;">${new Date(o.created_at).toLocaleString()}</td>
            </tr>
          `).join('');
        } else {
          tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; color: var(--text-muted);">No orders yet. Submit the form above to create the first order!</td></tr>';
        }
      } catch (e) {
        tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; color: var(--danger);">Failed to load orders: ' + e.message + '</td></tr>';
      }
    }

    // Init
    refreshStatus();
    loadRecentOrders();
  </script>
</body>
</html>
`

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
	mux.HandleFunc("/", dashboardHTMLHandler)
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/readyz", healthHandler)
	mux.HandleFunc("/api/v1/status", systemStatusHandler)
	mux.HandleFunc("/api/v1/orders", orderProxyHandler)
	mux.HandleFunc("/api/v1/orders/", orderProxyHandler)

	addr := fmt.Sprintf(":%s", appConfig.Port)
	log.Printf("Starting edge-frontend server on %s (Forwarding to Core: %s)", addr, appConfig.CoreBackendURL)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server terminated: %v", err)
	}
}
