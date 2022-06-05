package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

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
	id       string
	conn     *websocket.Conn
	send     chan interface{}
	name     string
	Position Position
}

type Message struct {
	Type     string   `json:"type"`
	Addr     string   `json:"addr,omitempty"`
	Msg      string   `json:"msg,omitempty"`
	PageX    int      `json:"pagex,omitempty"`
	PageY    int      `json:"pagey,omitempty"`
	Position Position `json:"position,omitempty"`
}

type Position struct {
	PageX int `json:"pagex"`
	PageY int `json:"pagey"`
}

var (
	entering  = make(chan *Client)
	leaving   = make(chan *Client)
	clients   = make(map[*Client]bool)
	broadcast = make(chan Message)
	private   = make(chan Message)
	position  = make(chan Message)
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

func hub() {
	for {
		select {
		case newcli := <-entering:
			for cli := range clients {
				cli.send <- Message{Type: "position", Addr: newcli.id, Position: Position{PageX: newcli.Position.PageX, PageY: newcli.Position.PageY}}
			}
			clients[newcli] = true

		case cli := <-leaving:
			if _, ok := clients[cli]; ok {
				delete(clients, cli)
				close(cli.send)
			}
		case msg := <-broadcast:
			for cli := range clients {
				cli.send <- msg
			}
		case msg := <-private:
			for cli := range clients {
				if cli.id == msg.Addr {
					cli.send <- msg
				}
			}
		case msg := <-position:
			for cli := range clients {
				cli.send <- msg
			}
		}
	}
}

func (c *Client) updatePosition(pagex int, pagey int) {
	c.Position.PageX = pagex
	c.Position.PageY = pagey
}

func (c *Client) readMessge() {
	defer func() {
		leaving <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		var msg Message
		_, message, err := c.conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		if err = json.Unmarshal([]byte(string(message)), &msg); err != nil {
			log.Printf("parse error : %s", err)
		}

		if msg.Type == "broadcast" {
			broadcast <- msg
		} else if msg.Type == "private" {
			private <- msg
		} else if msg.Type == "position" {
			c.updatePosition(msg.PageX, msg.PageY)
			position <- msg
		}
	}
}

func (c Client) writeMessge() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			log.Println(message, ok)

			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteJSON(message)
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade failed %s ", err)
		return
	}

	// cookie, err := r.Cookie("id")

	// if err != nil {
	// 	log.Fatal("Cookie: ", err)
	// }
	// userid := cookie.Value
	cli := NewClient(conn)
	// connected new client
	cli.conn.WriteJSON(Message{Type: "position", Addr: cli.id, Position: Position{PageX: cli.Position.PageX, PageY: cli.Position.PageY}})

	// if err := cli.conn.WriteMessage(websocket.TextMessage, []byte("your id is "+cli.id)); err != nil {
	// 	log.Println(err)
	// }

	entering <- cli
	go cli.writeMessge()
	go cli.readMessge()
}

func NewClient(conn *websocket.Conn) *Client {
	uuid := uuid.NewString()
	return &Client{id: uuid, conn: conn, send: make(chan interface{}, 5), Position: Position{PageX: 0, PageY: 0}}
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
	// uuid := uuid.NewString()
	// cookie := &http.Cookie{
	// 	Name:  "id",
	// 	Value: uuid,
	// }
	// http.SetCookie(w, cookie)
	http.ServeFile(w, r, "index.html")
}

func getUserNameFromClient(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, _ := io.ReadAll(r.Body)
	fmt.Println(string(body))
}

func main() {
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/chat", wshandler)
	http.HandleFunc("/username", getUserNameFromClient)

	go hub()

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
