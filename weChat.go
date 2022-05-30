package main

import (
	"fmt"
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
	entering  = make(chan *Client)
	leaving   = make(chan *Client)
	clients   = make(map[*Client]bool)
	broadcast = make(chan Message)
	private   = make(chan Message)
)

func (c *Client) usermanage() {
	for {
		select {
		case cli := <-entering:
			clients[cli] = true

		case cli := <-leaving:
			if _, ok := clients[cli]; ok {
				delete(clients, cli)
				close(cli.send)
			}
		case msg := <-broadcast:
			for client := range clients {
				select {
				case client.send <- msg:
				default:
					close(client.send)
					delete(clients, client)
				}
			}
		case msg := <-private:
			for cli := range clients {
				if cli.id == msg.Addr {
					select {
					case cli.send <- msg:
					default:
						close(cli.send)
						delete(clients, cli)
					}
				}
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
		if msg.Addr == "public" {
			broadcast <- msg
		} else {
			private <- msg
		}
	}
}

func (c Client) writeMessge() {
	for {
		select {
		case message, ok := <-c.send:
			fmt.Println(message, ok)
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteJSON(message)
		}
	}
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade failed %s ", err)
		return
	}
	cli := NewClient(conn)

	go cli.usermanage()

	if err := cli.conn.WriteMessage(1, []byte("your id is "+cli.id)); err != nil {
		log.Println(err)
	}

	entering <- cli
	go cli.writeMessge()
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
