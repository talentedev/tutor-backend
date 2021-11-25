package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 16384
)

type EventHandler func(event Event, engine *Engine)
type EngineHandler func(c *Connection)

type Engine struct {
	Hub              *Hub
	handlers         map[string][]EventHandler
	internalHandlers map[string][]EngineHandler
	router           *gin.RouterGroup
	hubMux           sync.Mutex
}

type EventData map[string]interface{}

type Event struct {
	Type   string      `json:"type"`
	Data   EventData   `json:"data,omitempty"`
	Time   int64       `json:"time,omitempty"`
	SeqID  int64       `json:"seqid,omitempty"`
	Source *Connection `json:"-"`
}

// Connection class used to represent a websocket connection
type Connection struct {
	sync.Mutex

	// WebSocket Connection
	ws *websocket.Conn

	// Sending channel
	send chan Event

	user *store.UserMgo `json:"user"`

	closed bool `json:"closed"`
}

// Hub handles connections
type Hub struct {
	// Registered connections.
	connections map[*Connection]bool

	// Inbound messages from the connections.
	broadcast chan Event

	// Register requests from the connections.
	register chan *Connection

	// Unregister requests from connections.
	unregister chan *Connection

	shutdown chan int
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  maxMessageSize,
	WriteBufferSize: maxMessageSize,
	CheckOrigin:     checkOrigin,
}

var hub *Hub
var engine *Engine

func (e Event) String() string {
	return fmt.Sprintf("[Event(%s : %d) %v]", e.Type, e.Time, e.Data)
}

func (e Event) MustHave(keys ...string) {

	for _, key := range keys {

		if !e.Has(key) {

			msg := fmt.Sprintf("Field '%s' is required for event: %s", key, e.Type)

			e.Source.send <- Event{
				Type: "error",
				Data: EventData{
					"seqid":   e.SeqID,
					"event":   e.Type,
					"message": msg,
					"code":    0,
				},
			}

			panic(msg)
		}
	}

}

func (e Event) Has(key string) (exist bool) {
	if e.Data == nil {
		return false
	}

	_, exist = e.Data[key]

	return exist
}

func (e Event) GetString(key string) string {
	if e.Data == nil {
		return ""
	}

	if v, exist := e.Data[key]; exist {
		return v.(string)
	}

	return ""
}

func (e Event) GetBool(key string, def bool) bool {
	if e.Data == nil {
		return def
	}

	if v, exist := e.Data[key]; exist {
		return v.(bool)
	}

	return def
}

func (e Event) GetInt(key string) int {
	if e.Data == nil {
		return 0
	}

	if v, exist := e.Data[key]; exist {
		return int(v.(float64))
	}

	return 0
}

func (e Event) GetFloat(key string) float64 {
	if e.Data == nil {
		return 0
	}

	if v, exist := e.Data[key].(float64); exist {
		return v
	}

	return 0
}

func (a EventData) Equal(b EventData) (equal bool) {
	for _, key := range reflect.ValueOf(a).MapKeys() {
		bk, be := b[key.String()]
		if !be {
			return false
		}

		if fmt.Sprintf("%v", a[key.String()]) != fmt.Sprintf("%v", bk) {
			return false
		}
	}

	return true
}

func (a EventData) Marshal(to interface{}) (err error) {
	marshal, err := json.Marshal(a)
	return json.Unmarshal(marshal, to)
}

func (a EventData) MarshalKey(key string, to interface{}) (err error) {

	mv := reflect.ValueOf(a).MapIndex(reflect.ValueOf(key))

	if !mv.IsValid() {
		return errors.Errorf("Missing key '%s' from event data", key)
	}

	marshal, err := json.Marshal(mv.Interface())

	return json.Unmarshal(marshal, to)
}

func (h *Hub) Count() (count int) {
	return len(h.connections)
}

func (h *Hub) Info() (info interface{}) {
	uniqueOnline := map[bson.ObjectId]*store.UserDto{}
	for c := range h.connections {
		u := c.user.Dto()
		uniqueOnline[u.ID] = u
	}

	online := make([]interface{}, 0)
	for _, v := range uniqueOnline {
		online = append(online, v)
	}

	return bson.M{
		"online": online,
	}
}

func (h *Hub) Register(c *Connection) {
	h.register <- c
}

func (h *Hub) User(id bson.ObjectId) *Connection {
	for c := range h.connections {
		if c.user != nil && c.user.ID.Hex() == id.Hex() {
			return c
		}
	}
	return nil
}

func (h *Hub) NotifyOnlinePresence(c *Connection, presence store.UserPresence) {

	if c == nil || c.user == nil {
		panic("Unexpected nil connection or user")
	}

	c.user.SetPresence(presence)

	for item := range h.connections {
		if item != c {
			item.send <- Event{
				Type: "presence",
				Data: EventData{
					"user":   c.user.ID.Hex(),
					"online": c.user.Online,
				},
			}
		} else {
			users := make([]string, 0)
			for item := range h.connections {
				if item.user == nil {
					panic("IMAError: User expected")
				}
				users = append(users, item.user.ID.Hex())
			}
			item.send <- Event{
				Type: "presence",
				Data: EventData{
					"users": users,
				},
			}
		}
	}
}

// Run starts handling connections
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
			logger.Get().Infof("Socket: User connected: %s", c.GetUser().ID.Hex())
			internalEngineTrigger("onenter", c)
		case c := <-h.unregister:

			internalEngineTrigger("onleave", c)

			if _, ok := h.connections[c]; !ok {
				break
			}

			logger.Get().Infof("Socket: User disconnected: %s", c.GetUser().ID.Hex())

			engine.RouteEvent(Event{Type: "system.user.leave", Source: c})
			delete(h.connections, c)

			c.Lock()
			close(c.send)
			c.closed = true
			c.Unlock()
		case m := <-h.broadcast:
			for c := range h.connections {
				select {
				case c.send <- m:
				default:
					close(c.send)
					c.closed = true
					delete(h.connections, c)
				}
			}
		case <-h.shutdown:
			logger.Get().Infof("Hub gracefully stopped")
			break
		}
	}
}

// Read data from connection
func (c *Connection) readPump() {
	defer func() {
		hub.unregister <- c
		c.ws.Close()
	}()

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		messageType, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				logger.Get().Errorf("Error on c.ws.ReadMessage(): %v\n", err)
			}
			break
		}

		if messageType == websocket.TextMessage {
			event := Event{}

			if err := json.Unmarshal(message, &event); err != nil {
				logger.Get().Errorf("Fail to unmarshal socket packet: %v", err)
				continue
			}

			event.Source = c

			if string(event.Type) == "ping" {
				event.Respond("pong", nil)
				// return
			}

			if !engine.RouteEvent(event) {
				logger.Get().Infof("Event unknown:", event.String())
				//return
			}
		}
	}

}

func (c *Connection) writeEvent(event Event) error {

	event.Time = time.Now().Unix()

	bytes, err := json.Marshal(event)
	if err != nil {
		logger.Get().Errorf("Fail to encode bson bytes: %v", err)
		return err
	}

	if err := c.write(websocket.TextMessage, bytes); err != nil {
		logger.Get().Errorf("Fail to send bson bytes %s: %v", string(bytes), err.Error())
		return err
	}

	return nil
}

func (c *Connection) Close() {
	c.Lock()
	c.ws.Close()
	c.closed = true
	c.Unlock()
}

// write writes a message with the given message type and payload.
func (c *Connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

// Write data to connection
func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			c.writeEvent(data)
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (c *Connection) SetUser(user *store.UserMgo) (err error) {
	if c.user != nil {
		return errors.New("user already set")
	}

	c.user = user

	return
}

func (c *Connection) GetUser() (user *store.UserMgo) {

	if c == nil {
		panic("Unexpected")
	}

	return c.user
}

func (c *Connection) Send(event Event) (err error) {

	if c.closed {
		return errors.New("connection closed")
	}

	event.Time = time.Now().Unix()

	c.send <- event

	return
}

func (e Event) Respond(tip string, data EventData) {
	if e.Source != nil {
		e.Source.Send(Event{
			Type:  tip,
			Data:  data,
			Time:  time.Now().Unix(),
			SeqID: e.SeqID,
		})
	}
}

func (e Event) Error(message string, code int) {
	if e.Source != nil {
		e.Source.Send(Event{
			Type: "error",
			Data: EventData{
				"seqid":   e.SeqID,
				"event":   e.Type,
				"message": message,
				"code":    code,
			},
		})

		//time.AfterFunc(time.Second, func() { e.Source.Close() })
	}
}

// Check if origin is allowed
func checkOrigin(r *http.Request) bool {
	actualOrigin := r.Header.Get("origin")
	origins := config.GetConfig().GetString("websocket.origins")

	if origins == "" {
		return true
	}

	for _, origin := range strings.Split(origins, ",") {
		if origin == actualOrigin {
			return true
		}
	}

	logger.Get().Error("Origin not allowed:", actualOrigin)

	return false
}

// Start listening for connections
func serve(user *store.UserMgo, w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		logger.Get().Error("Method not allowed:", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Get().Error(err.Error())
		return
	}

	c := &Connection{
		ws:   ws,
		send: make(chan Event, 256),
		user: user,
	}

	hub.Register(c)

	go c.writePump()

	c.readPump()
}

func Init(g *gin.RouterGroup) *Engine {
	hub = &Hub{
		broadcast:   make(chan Event),
		register:    make(chan *Connection),
		unregister:  make(chan *Connection),
		connections: make(map[*Connection]bool),
	}

	go hub.Run()

	g.GET("", func(c *gin.Context) {
		auth.Middleware(c)
		user, e := store.GetUser(c)
		if !e {
			return
		}

		if c.Query("access_token") != "" {
			c.Header("Cookie", "token="+c.Query("access_token"))
		}

		serve(user, c.Writer, c.Request)
	})

	engine = &Engine{
		hub,
		make(map[string][]EventHandler, 0),
		make(map[string][]EngineHandler, 0),
		g,
		sync.Mutex{},
	}

	return engine
}

func (engine *Engine) OnLeave(cb EngineHandler) func() {
	return internalEngineListen("onleave", cb)
}

func (engine *Engine) OnEnter(cb EngineHandler) func() {
	return internalEngineListen("onenter", cb)
}

func internalEngineTrigger(event string, c *Connection) {
	for name, handlers := range engine.internalHandlers {
		if name == event {
			for _, handler := range handlers {
				handler(c)
			}
		}
	}
}

func internalEngineListen(event string, handler EngineHandler) (unregister func()) {

	engine.hubMux.Lock()

	if _, e := engine.internalHandlers[event]; !e {
		engine.internalHandlers[event] = make([]EngineHandler, 0)
	}

	index := len(engine.internalHandlers[event]) - 1

	engine.internalHandlers[event] = append(engine.internalHandlers[event], handler)

	unregister = func() {
		engine.hubMux.Lock()

		engine.internalHandlers[event] = append(
			engine.internalHandlers[event][:index],
			engine.internalHandlers[event][index+1:]...,
		)

		engine.hubMux.Unlock()
	}

	engine.hubMux.Unlock()

	return
}

func (engine *Engine) Listen(etype string, handler EventHandler) (unregister func()) {
	engine.hubMux.Lock()

	if _, e := engine.handlers[etype]; !e {
		engine.handlers[etype] = make([]EventHandler, 0)
	}

	index := len(engine.handlers[etype]) - 1

	engine.handlers[etype] = append(engine.handlers[etype], handler)

	unregister = func() {
		engine.hubMux.Lock()

		engine.handlers[etype] = append(
			engine.handlers[etype][:index],
			engine.handlers[etype][index+1:]...,
		)

		engine.hubMux.Unlock()
	}

	engine.hubMux.Unlock()

	return
}

func (engine *Engine) UserLeave(f func(c *Connection)) {
	engine.Listen("system.user.leave", func(event Event, engine *Engine) {
		f(event.Source)
	})
}

func (engine *Engine) RouteEvent(event Event) bool {
	handlers, ok := engine.handlers[event.Type]
	if !ok {
		return false
	}

	for _, handler := range handlers {
		go handler(event, engine)
	}

	return true
}

// Notify sends event to all users if are online
func (e *Engine) Notify(event string, users []string, data EventData) {
	for c := range e.Hub.connections {
		for _, user := range users {
			if c.GetUser().ID.Hex() == user {
				c.send <- Event{
					Type: event,
					Data: data,
					Time: time.Now().Unix(),
				}
			}
		}
	}
}

func GetEngine() *Engine {
	return engine
}

func (engine *Engine) Shutdown() {
	engine.Hub.shutdown <- 1
}
