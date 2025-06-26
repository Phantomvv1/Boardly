package chat

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Client struct {
	ws   *websocket.Conn
	send chan []byte
}

type Hub struct {
	Broadcast         chan []byte      //message I need to send to all of the clients
	Clients           map[*Client]bool //all of the clients I already have
	RegisteringClient chan *Client     //a new client
	UnregisterClient  chan *Client     //when a client disconnects
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:         make(chan []byte),
		Clients:           make(map[*Client]bool),
		RegisteringClient: make(chan *Client),
		UnregisterClient:  make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.RegisteringClient:
			h.Clients[client] = true
		case client := <-h.UnregisterClient:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.send)
				client = nil
			}
		case message := <-h.Broadcast:
			for client := range h.Clients {
				select {
				case client.send <- message:
				default:
					delete(h.Clients, client)
					close(client.send)
					client = nil
				}
			}
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var RunningHub = NewHub()

func Chat(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error unable to upgrade the endpoint to a websocket"})
		return
	}

	client := &Client{ws: ws, send: make(chan []byte)}
	RunningHub.RegisteringClient <- client

	go client.WriteMessage(ws)
	go client.ReadMessage(ws)
}

func (c *Client) WriteMessage(ws *websocket.Conn) {
	for send := range c.send {
		err := c.ws.WriteMessage(websocket.TextMessage, send)
		if err != nil {
			log.Println(err)
			c.ws.Close()
			return
		}
	}
}

func (c *Client) ReadMessage(ws *websocket.Conn) {
	defer func() {
		RunningHub.UnregisterClient <- c
		c.ws.Close()
	}()

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			return
		}

		RunningHub.Broadcast <- message
	}
}
