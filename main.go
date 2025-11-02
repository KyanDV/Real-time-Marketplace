package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// --- Struct Utama ---

// Stock merepresentasikan data kita
type Stock struct {
	ID    string  `json:"id"`
	Item  string  `json:"item"`
	Price float64 `json:"price"`
}

// WebSocketMessage (tidak berubah)
type WebSocketMessage struct {
	Type    string `json:"type"`
	Payload Stock  `json:"payload"`
}

// Store (tidak berubah)
type Store struct {
	stocks map[string]Stock
	mu     sync.RWMutex
}

func NewStore() *Store {
	return &Store{
		stocks: make(map[string]Stock),
	}
}

// --- Hub WebSocket (tidak berubah) ---

type Hub struct {
	clients   map[*websocket.Conn]bool
	broadcast chan WebSocketMessage
	mu        sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		broadcast: make(chan WebSocketMessage),
		clients:   make(map[*websocket.Conn]bool),
	}
}

func (h *Hub) run() {
	for {
		msg := <-h.broadcast

		h.mu.Lock()
		for client := range h.clients {
			if err := client.WriteJSON(msg); err != nil {
				log.Printf("error writing json: %v", err)
				client.Close()
				delete(h.clients, client)
			}
		}
		h.mu.Unlock()
	}
}

// handleWebSocket (tidak berubah)
func (h *Hub) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("error upgrading: %v", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()
	log.Println("Klien baru terhubung")

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		conn.Close()
		log.Println("Klien terputus")
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// --- Handler API CRUD ---

// handleGetStocks (tidak berubah)
func (s *Store) handleGetStocks(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stockList []Stock
	for _, stock := range s.stocks {
		stockList = append(stockList, stock)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stockList)
}

// handleStock CRUD
func (s *Store) handleStock(hub *Hub, w http.ResponseWriter, r *http.Request) {
	var stock Stock
	if err := json.NewDecoder(r.Body).Decode(&stock); err != nil {
		http.Error(w, "Request body tidak valid", http.StatusBadRequest)
		return
	}

	var msgType string

	switch r.Method {
	case "POST": // CREATE
		stock.ID = uuid.New().String()
		s.mu.Lock()
		s.stocks[stock.ID] = stock
		s.mu.Unlock()
		msgType = "CREATE"
		log.Printf("Stok DIBUAT: %s", stock.Item) // <--- DIGANTI

	case "PUT": // UPDATE
		if stock.ID == "" {
			http.Error(w, "ID diperlukan untuk update", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.stocks[stock.ID] = stock
		s.mu.Unlock()
		msgType = "UPDATE"
		log.Printf("Stok DIPERBARUI: %s", stock.Item) // <--- DIGANTI

	case "DELETE": // DELETE
		if stock.ID == "" {
			http.Error(w, "ID diperlukan untuk delete", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		delete(s.stocks, stock.ID)
		s.mu.Unlock()
		msgType = "DELETE"
		log.Printf("Stok DIHAPUS: %s", stock.Item) // <--- DIGANTI

	default:
		http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
		return
	}

	msg := WebSocketMessage{
		Type:    msgType,
		Payload: stock,
	}
	hub.broadcast <- msg

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stock)
}

// --- Handler Penyaji Halaman (tidak berubah) ---

func serveAdminPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "admin.html")
}

func serveViewerPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "viewer.html")
}

// --- Main (tidak berubah) ---

func main() {
	hub := NewHub()
	go hub.run()

	store := NewStore()

	http.HandleFunc("/", serveViewerPage)
	http.HandleFunc("/admin", serveAdminPage)
	http.HandleFunc("/ws", hub.handleWebSocket)
	http.HandleFunc("/api/stocks", store.handleGetStocks)

	http.HandleFunc("/api/stock", func(w http.ResponseWriter, r *http.Request) {
		store.handleStock(hub, w, r)
	})

	log.Println("Server dimulai di http://localhost:8080")
	log.Println("Halaman Admin: http://localhost:8080/admin")
	log.Println("Halaman Viewer: http://localhost:8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
