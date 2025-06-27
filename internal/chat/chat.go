package chat

import (
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"slices"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Client struct {
	ID        uint64 `json:"id"`
	Points    uint64 `json:"points"`
	ws        *websocket.Conn
	send      chan []byte
	sendBoard chan []byte
}

type Hub struct {
	Broadcast         chan []byte      //message I need to send to all of the clients
	Clients           map[*Client]bool //all of the clients I already have
	RegisteringClient chan *Client     //a new client
	UnregisterClient  chan *Client     //when a client disconnects
	BoradcastBoard    chan []byte
}

type Board struct {
	Board []*Client
}

func NewBoard() *Board {
	return &Board{
		Board: make([]*Client, 0),
	}
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:         make(chan []byte),
		Clients:           make(map[*Client]bool),
		RegisteringClient: make(chan *Client),
		UnregisterClient:  make(chan *Client),
		BoradcastBoard:    make(chan []byte),
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
					close(client.sendBoard)
					client = nil
				}
			}
		case board := <-h.BoradcastBoard:
			for client := range h.Clients {
				select {
				case client.sendBoard <- board:
				default:
					delete(h.Clients, client)
					close(client.send)
					close(client.sendBoard)
					client = nil
				}
			}
		}
	}
}

func (b *Board) TransformToBytes() []byte {
	boardData := []byte{}
	for _, client := range RunningBoard.Board {
		buf := make([]byte, 16)
		binary.BigEndian.PutUint64(buf[:8], client.ID)
		binary.BigEndian.PutUint64(buf[8:], client.Points)

		boardData = append(boardData, buf...)
	}

	return boardData
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var mu = &sync.Mutex{}
var RunningHub = NewHub()
var ClientsNumber uint64 = 0
var RunningBoard = NewBoard()

func Chat(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error unable to upgrade the endpoint to a websocket"})
		return
	}

	mu.Lock()
	ClientsNumber++
	mu.Unlock()

	client := &Client{ID: ClientsNumber, ws: ws, send: make(chan []byte), Points: 0, sendBoard: make(chan []byte)}
	RunningBoard.Board = append(RunningBoard.Board, client)
	RunningHub.RegisteringClient <- client

	go client.WriteMessage(ws)
	go client.ReadMessage(ws)
	go client.WriteBoard(ws)
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

		c.UpdatePoints(uint64(len(message)))
		UpdateBoard(c)

		message = append([]byte(fmt.Sprintf("%d: ", c.ID)), message...)

		RunningHub.Broadcast <- message
		RunningHub.BoradcastBoard <- RunningBoard.TransformToBytes()
	}
}

func (c *Client) WriteBoard(ws *websocket.Conn) {
	for boardData := range c.sendBoard {
		err := ws.WriteMessage(websocket.BinaryMessage, boardData)
		if err != nil {
			log.Println(err)
			c.ws.Close()
			return
		}
	}
}

func (c *Client) UpdatePoints(points uint64) {
	c.Points += points
}

func GetClientID(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"id": ClientsNumber})
}

func UpdateBoard(client *Client) {
	clientIndex := slices.Index(RunningBoard.Board, client)

	change := 0
	for i := clientIndex; i >= 1; i-- {
		if client.Points >= RunningBoard.Board[i-1].Points {
			change++
		}
	}

	if change != 0 {
		RemoveFromBoard(clientIndex)
		InsertIntoBoard(client, clientIndex-change)
	}
}

func RemoveFromBoard(index int) {
	RunningBoard.Board = append(RunningBoard.Board[:index], RunningBoard.Board[index+1:]...)
}

func InsertIntoBoard(client *Client, index int) {
	if index == 0 {
		RunningBoard.Board = append([]*Client{client}, RunningBoard.Board...)
		return
	} else if index == len(RunningBoard.Board) {
		RunningBoard.Board = append(RunningBoard.Board, client)
		return
	}

	RunningBoard.Board = append(RunningBoard.Board[:index], append([]*Client{client}, RunningBoard.Board[index:]...)...)
}
