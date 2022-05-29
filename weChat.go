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
	entering = make(chan Client)
	leaving  = make(chan Client)
	clients  = make(map[Client]bool)
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

func handleConn(conn *websocket.Conn) {
	cli := *NewClient(conn)
	who := cli.id
	for cli := range clients {
		if err := cli.conn.WriteMessage(1, []byte(who+" has arrvied")); err != nil {
			log.Printf("failed sending %s", err)
		}
	}
	entering <- cli

	go cli.broadCast()
	go cli.readMessge()
}

func (c *Client) readMessge() {
	defer func() {
		leaving <- *c
		c.conn.Close()
	}()
	for {
		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			return
		}
		c.send <- msg
	}
}

func (c Client) broadCast() {
	for msg := range c.send {
		for cli := range clients {
			if err := cli.conn.WriteJSON(msg); err != nil {
				log.Printf("failed sending message to client %s", err)
			}
		}
	}
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed upgrade %s ", err)
		return
	}

	go usermanage()
	go handleConn(conn)
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
