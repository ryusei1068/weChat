package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
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
	Position Position `json:"position,omitempty"`
}

type Position struct {
	PageX  float64 `json:"pagex"`
	PageY  float64 `json:"pagey"`
	Height float64 `json:"height"`
	Width  float64 `json:"width"`
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
			var newUserLocation Message = Message{Type: "move", Addr: newcli.id, Position: Position{PageX: newcli.Position.PageX, PageY: newcli.Position.PageY}}
			for cli := range clients {
				var userLocation Message = Message{Type: "move", Addr: cli.id, Position: Position{PageX: cli.Position.PageX, PageY: cli.Position.PageY}}
				cli.send <- newUserLocation
				newcli.send <- userLocation
			}

			clients[newcli] = true

		case cli := <-leaving:
			if _, ok := clients[cli]; ok {
				delete(clients, cli)
				close(cli.send)
				var msg Message = Message{Type: "leaved", Addr: cli.id}
				for cli := range clients {
					cli.send <- msg
				}
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

func (c *Client) updatePosition(pagex, pagey float64) {
	c.Position.PageX = pagex
	c.Position.PageY = pagey
}

func (c *Client) readMessge() {
	defer func() {
		leaving <- c
		c.conn.Close()
	}()

	//	c.conn.SetReadLimit(maxMessageSize)
	//	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	//	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		var msg Message
		_, message, err := c.conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		if err = json.Unmarshal([]byte(string(message)), &msg); err != nil {
			log.Printf("parse error : %s", err)
		}

		if msg.Type == "private" {
			private <- msg
		} else if msg.Type == "move" {
			c.updatePosition(msg.Position.PageX, msg.Position.PageY)
			position <- msg
		}
	}
}

func (c *Client) writeMessge() {
	//ticker := time.NewTicker(pingPeriod)
	defer func() {
		//ticker.Stop()
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
			//	case <-ticker.C:
			//			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			//		if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			//		return
			//	}
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
	cli.conn.WriteJSON(Message{Type: "newclient", Addr: cli.id, Position: Position{PageX: cli.Position.PageX, PageY: cli.Position.PageY}})

	entering <- cli
	go cli.writeMessge()
	go cli.readMessge()
}

func NewClient(conn *websocket.Conn) *Client {
	uuid := uuid.NewString()
	pagex := rand.Float64() + 50
	pagey := rand.Float64() + 50
	return &Client{id: uuid, conn: conn, send: make(chan interface{}, 5), Position: Position{PageX: pagex, PageY: pagey}}
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

	//	http.HandleFunc("/", serveHome)
	http.Handle("/", http.FileServer(http.Dir("root/")))

	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	http.HandleFunc("/chat", wshandler)
	// http.HandleFunc("/username", getUserNameFromClient)

	go hub()

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
