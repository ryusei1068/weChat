package main

import (
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Resolve cross-domain problems
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	id   string
	conn *websocket.Conn
	send chan Message
}

type Message struct {
	Addr string `json:"addr"`
	Msg  string `json:"msg"`
}

var (
	entering = make(chan *Client)
	leaving  = make(chan *Client)
	clients  = make(map[*Client]bool)
)

func usermanage() {
	for {
		select {
		case cli := <-entering:
			clients[cli] = true

		case cli := <-leaving:
			if _, ok := clients[cli]; ok {
				delete(clients, cli)
				close(cli.send)
			}
		}
	}
}

func (c *Client) readMessge() {
	defer func() {
		leaving <- c
		c.conn.Close()
	}()
	for {
		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}
		c.send <- msg
	}
}

func (c Client) broadCast(msg Message) {
	for cli := range clients {
		if err := cli.conn.WriteJSON(msg); err != nil {
			log.Printf("failed to send to all users %s", err)
		}
	}
}

func (c Client) privateMsg(msg Message) {
	for cli := range clients {
		if cli.id == msg.Addr {
			if err := cli.conn.WriteJSON(msg); err != nil {
				log.Printf("failed to send to specific user %s", err)
			}
		}
	}
}

func (c Client) sender() {
	for msg := range c.send {
		if msg.Addr == "public" {
			c.broadCast(msg)
		} else {
			c.privateMsg(msg)
		}
	}
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade failed %s ", err)
		return
	}

	go usermanage()

	cli := NewClient(conn)
	if err := cli.conn.WriteMessage(1, []byte("your id is "+cli.id)); err != nil {
		log.Println(err)
	}

	entering <- cli

	go cli.sender()
	go cli.readMessge()
}

func NewClient(conn *websocket.Conn) *Client {
	uuid := uuid.NewString()
	return &Client{id: uuid, conn: conn, send: make(chan Message)}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "index.html")
}

func main() {
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/chat", wshandler)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
