package realtime

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"github.com/inheritance-choir/backend/internal/repository"
	jwtpkg "github.com/inheritance-choir/backend/pkg/jwt"
)

type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
	TS   int64       `json:"ts"`
}

type Client struct {
	conn     *websocket.Conn
	memberID string
	send     chan Event
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
	log     *zap.Logger
}

var defaultHub *Hub

func NewHub(log *zap.Logger) *Hub {
	h := &Hub{clients: map[*Client]bool{}, log: log}
	defaultHub = h
	return h
}

func Publish(eventType string, data interface{}) {
	if defaultHub != nil {
		defaultHub.Publish(eventType, data)
	}
}

func PublishTo(memberID, eventType string, data interface{}) {
	if defaultHub != nil {
		defaultHub.PublishTo(memberID, eventType, data)
	}
}

func (h *Hub) Publish(eventType string, data interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ev := Event{Type: eventType, Data: data, TS: time.Now().UnixMilli()}
	for c := range h.clients {
		select {
		case c.send <- ev:
		default:
		}
	}
}

func (h *Hub) PublishTo(memberID, eventType string, data interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ev := Event{Type: eventType, Data: data, TS: time.Now().UnixMilli()}
	for c := range h.clients {
		if c.memberID == memberID {
			select {
			case c.send <- ev:
			default:
			}
		}
	}
}

func (h *Hub) HandleWebSocket(db *repository.DB, jwtMgr *jwtpkg.Manager) gin.HandlerFunc {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
	}
	return func(c *gin.Context) {
		token := c.Query("token")
		claims, err := jwtMgr.VerifyAccessToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid token"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		client := &Client{conn: conn, memberID: claims.MemberID, send: make(chan Event, 32)}
		h.register(client)
		db.Members().UpdateOne(c.Request.Context(), bson.M{"_id": mustObjectID(claims.MemberID)}, bson.M{"$set": bson.M{"online": true, "lastSeen": time.Now()}})
		h.Publish("member:online", gin.H{"memberId": claims.MemberID})

		go h.writePump(client)
		h.readPump(client)

		h.unregister(client)
		db.Members().UpdateOne(c.Request.Context(), bson.M{"_id": mustObjectID(claims.MemberID)}, bson.M{"$set": bson.M{"online": false, "lastSeen": time.Now()}})
		h.Publish("member:offline", gin.H{"memberId": claims.MemberID})
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	if h.clients[c] {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
	c.conn.Close()
}

func (h *Hub) readPump(c *Client) {
	defer c.conn.Close()
	c.conn.SetReadLimit(4096)
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg map[string]interface{}
		if json.Unmarshal(raw, &msg) == nil && msg["type"] == "ping" {
			_ = c.conn.WriteJSON(Event{Type: "pong", Data: gin.H{"memberId": c.memberID}, TS: time.Now().UnixMilli()})
		}
	}
}

func (h *Hub) writePump(c *Client) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case ev, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(ev); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteJSON(Event{Type: "system", Data: gin.H{"message": "heartbeat"}, TS: time.Now().UnixMilli()}); err != nil {
				return
			}
		}
	}
}

func mustObjectID(id string) interface{} {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return id
	}
	return oid
}
