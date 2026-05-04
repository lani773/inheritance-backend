// Package realtime — upgraded bi-directional WebSocket hub with channel/room support,
// auto-reconnect heartbeat, presence tracking, and typed client message routing.
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

// ── Wire types ───────────────────────────────────────────────────

// ServerEvent is what the server sends to a client.
type ServerEvent struct {
	Type    string      `json:"type"`
	Channel string      `json:"channel,omitempty"`
	Data    interface{} `json:"data"`
	TS      int64       `json:"ts"`
}

// ClientMessage is what a client sends to the server.
type ClientMessage struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// ── Client ───────────────────────────────────────────────────────

// Client represents one connected WebSocket session.
type Client struct {
	conn      *websocket.Conn
	memberID  string
	voicePart string
	isAdmin   bool
	send      chan ServerEvent
	channels  map[string]bool
	mu        sync.RWMutex
}

func (c *Client) subscribed(ch string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.channels[ch]
}

func (c *Client) subscribe(ch string) {
	c.mu.Lock()
	c.channels[ch] = true
	c.mu.Unlock()
}

func (c *Client) unsubscribeChannel(ch string) {
	c.mu.Lock()
	delete(c.channels, ch)
	c.mu.Unlock()
}

// ── Hub ──────────────────────────────────────────────────────────

// Hub is the central WebSocket broker.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
	log     *zap.Logger
}

var defaultHub *Hub

// NewHub creates and registers the global hub.
func NewHub(log *zap.Logger) *Hub {
	h := &Hub{clients: map[*Client]bool{}, log: log}
	defaultHub = h
	return h
}

// ── Package-level helpers (used by handlers) ──────────────────────

// Publish broadcasts to ALL connected clients (backward-compatible 2-arg wrapper).
func Publish(eventType string, data interface{}) {
	if defaultHub != nil {
		defaultHub.broadcast(eventType, data)
	}
}

// PublishToChannel sends to clients subscribed to a specific channel.
func PublishToChannel(ch, eventType string, data interface{}) {
	if defaultHub != nil {
		defaultHub.PublishToChannel(ch, eventType, data)
	}
}

// PublishTo sends a targeted event to a specific member.
func PublishTo(memberID, eventType string, data interface{}) {
	if defaultHub != nil {
		defaultHub.PublishTo(memberID, eventType, data)
	}
}

// PublishToAdmins sends an event only to admin clients.
func PublishToAdmins(eventType string, data interface{}) {
	if defaultHub != nil {
		defaultHub.PublishToAdmins(eventType, data)
	}
}

// OnlineCount returns the number of connected clients.
func OnlineCount() int {
	if defaultHub != nil {
		return defaultHub.OnlineCount()
	}
	return 0
}

// ── Hub methods ───────────────────────────────────────────────────

// broadcast sends to ALL connected clients (used by package-level Publish).
func (h *Hub) broadcast(eventType string, data interface{}) {
	ev := ServerEvent{Type: eventType, Data: data, TS: time.Now().UnixMilli()}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- ev:
		default:
		}
	}
}

// Publish sends an event to ALL connected clients (with optional channel tag).
func (h *Hub) Publish(eventType, channel string, data interface{}) {
	ev := ServerEvent{Type: eventType, Channel: channel, Data: data, TS: time.Now().UnixMilli()}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- ev:
		default:
		}
	}
}

// PublishToChannel sends to clients subscribed to a specific channel.
func (h *Hub) PublishToChannel(ch, eventType string, data interface{}) {
	ev := ServerEvent{Type: eventType, Channel: ch, Data: data, TS: time.Now().UnixMilli()}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.subscribed(ch) || c.subscribed(ChGeneral) {
			select {
			case c.send <- ev:
			default:
			}
		}
	}
}

// PublishTo sends a targeted event to a specific member.
func (h *Hub) PublishTo(memberID, eventType string, data interface{}) {
	ev := ServerEvent{Type: eventType, Data: data, TS: time.Now().UnixMilli()}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.memberID == memberID {
			select {
			case c.send <- ev:
			default:
			}
		}
	}
}

// PublishToAdmins sends an event only to admin clients.
func (h *Hub) PublishToAdmins(eventType string, data interface{}) {
	ev := ServerEvent{Type: eventType, Channel: ChAdmin, Data: data, TS: time.Now().UnixMilli()}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.isAdmin {
			select {
			case c.send <- ev:
			default:
			}
		}
	}
}

// OnlineCount returns the number of connected clients.
func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// OnlineMembers returns a list of online member IDs.
func (h *Hub) OnlineMembers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	seen := map[string]bool{}
	out := []string{}
	for c := range h.clients {
		if !seen[c.memberID] {
			seen[c.memberID] = true
			out = append(out, c.memberID)
		}
	}
	return out
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

// ── WebSocket upgrade handler ─────────────────────────────────────

func (h *Hub) HandleWebSocket(db *repository.DB, jwtMgr *jwtpkg.Manager) gin.HandlerFunc {
	upgrader := websocket.Upgrader{
		CheckOrigin:     func(*http.Request) bool { return true },
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
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
			h.log.Warn("ws upgrade failed", zap.Error(err))
			return
		}

		// Fetch member info for channel defaults
		ctx := c.Request.Context()
		oid, _ := primitive.ObjectIDFromHex(claims.MemberID)
		var member struct {
			IsAdmin   bool   `bson:"isAdmin"`
			VoicePart string `bson:"voicePart"`
		}
		_ = db.Members().FindOne(ctx, bson.M{"_id": oid}).Decode(&member)

		// Default channel subscriptions
		defaultChannels := map[string]bool{
			ChGeneral: true,
			VoicePartChannel(member.VoicePart): true,
		}
		if member.IsAdmin {
			defaultChannels[ChAdmin] = true
			defaultChannels[ChFinance] = true
		}

		client := &Client{
			conn:      conn,
			memberID:  claims.MemberID,
			voicePart: member.VoicePart,
			isAdmin:   member.IsAdmin,
			send:      make(chan ServerEvent, 64),
			channels:  defaultChannels,
		}

		h.register(client)

		// Mark online
		db.Members().UpdateOne(ctx, bson.M{"_id": oid},
			bson.M{"$set": bson.M{"online": true, "lastSeen": time.Now()}})

		// Announce presence to all
		h.Publish(EvtMemberOnline, ChGeneral, gin.H{
			"memberId": claims.MemberID,
			"online":   true,
		})

		// Send connected ack with presence list
		client.send <- ServerEvent{
			Type: EvtConnected,
			Data: gin.H{
				"memberId":      claims.MemberID,
				"channels":      defaultChannels,
				"onlineMembers": h.OnlineMembers(),
			},
			TS: time.Now().UnixMilli(),
		}

		go h.writePump(client)
		h.readPump(client, db)

		// Cleanup on disconnect
		h.unregister(client)
		db.Members().UpdateOne(ctx, bson.M{"_id": oid},
			bson.M{"$set": bson.M{"online": false, "lastSeen": time.Now()}})
		h.Publish(EvtMemberOffline, ChGeneral, gin.H{
			"memberId": claims.MemberID,
			"online":   false,
		})
	}
}

// ── Read pump — handles inbound client messages ───────────────────

func (h *Hub) readPump(c *Client, db *repository.DB) {
	defer c.conn.Close()
	c.conn.SetReadLimit(8192)
	c.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var msg ClientMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case EvtPing:
			c.send <- ServerEvent{Type: EvtPong, Data: gin.H{"memberId": c.memberID}, TS: time.Now().UnixMilli()}

		case EvtSubscribe:
			// Client wants to join a channel
			var req struct {
				Channel string `json:"channel"`
			}
			if json.Unmarshal(msg.Data, &req) == nil && req.Channel != "" {
				// Admins can join ChAdmin; others only non-admin channels
				if req.Channel == ChAdmin && !c.isAdmin {
					continue
				}
				c.subscribe(req.Channel)
				c.send <- ServerEvent{
					Type:    "subscribed",
					Channel: req.Channel,
					Data:    gin.H{"channel": req.Channel, "ok": true},
					TS:      time.Now().UnixMilli(),
				}
			}

		case EvtChatMessage:
			// Bi-directional chat: relay to the channel immediately
			h.routeChatMessage(c, msg, db)

		case EvtChatTyping:
			// Relay typing indicator to channel members (exclude sender)
			var req struct {
				Channel  string `json:"channel"`
				IsTyping bool   `json:"isTyping"`
			}
			if json.Unmarshal(msg.Data, &req) == nil {
				ch := req.Channel
				if ch == "" {
					ch = ChGeneral
				}
				ev := ServerEvent{
					Type:    EvtChatTyping,
					Channel: ch,
					Data: gin.H{
						"memberId":  c.memberID,
						"isTyping":  req.IsTyping,
						"channelId": ch,
					},
					TS: time.Now().UnixMilli(),
				}
				h.mu.RLock()
				for cl := range h.clients {
					if cl.memberID != c.memberID && cl.subscribed(ch) {
						select {
						case cl.send <- ev:
						default:
						}
					}
				}
				h.mu.RUnlock()
			}
		}
	}
}

// routeChatMessage persists a chat message and broadcasts it to channel subscribers.
func (h *Hub) routeChatMessage(c *Client, msg ClientMessage, db *repository.DB) {
	var req struct {
		ChannelID   string `json:"channelId"`
		Text        string `json:"text"`
		Attachments []string `json:"attachments"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil || req.Text == "" {
		return
	}

	ch := req.ChannelID
	if ch == "" {
		ch = ChGeneral
	}

	chatMsg := map[string]interface{}{
		"channelId":   ch,
		"senderId":    c.memberID,
		"text":        req.Text,
		"attachments": req.Attachments,
		"readBy":      []string{c.memberID},
		"reactions":   map[string][]string{},
		"createdAt":   time.Now(),
	}

	res, err := db.ChatMessages().InsertOne(nil, chatMsg)
	if err != nil {
		return
	}

	chatMsg["id"] = res.InsertedID

	// Broadcast to channel
	ev := ServerEvent{
		Type:    EvtChatMessage,
		Channel: ch,
		Data:    chatMsg,
		TS:      time.Now().UnixMilli(),
	}
	h.mu.RLock()
	for cl := range h.clients {
		if cl.subscribed(ch) {
			select {
			case cl.send <- ev:
			default:
			}
		}
	}
	h.mu.RUnlock()
}

// ── Write pump — sends queued events to client ────────────────────

func (h *Hub) writePump(c *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case ev, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(ev); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
