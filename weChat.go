package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
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
	To       string   `json:"to,omitempty"`
	From     string   `json:"from,omitempty"`
	Msg      string   `json:"msg,omitempty"`
	Position Position `json:"position,omitempty"`
}

type Position struct {
	PageX float64 `json:"pagex"`
	PageY float64 `json:"pagey"`
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
			var newUserLocation Message = Message{Type: "move", To: newcli.id, Position: Position{PageX: newcli.Position.PageX, PageY: newcli.Position.PageY}}
			for cli := range clients {
				var userLocation Message = Message{Type: "move", To: cli.id, Position: Position{PageX: cli.Position.PageX, PageY: cli.Position.PageY}}
				cli.send <- newUserLocation
				newcli.send <- userLocation
			}

			clients[newcli] = true

		case cli := <-leaving:
			if _, ok := clients[cli]; ok {
				delete(clients, cli)
				close(cli.send)
				var msg Message = Message{Type: "leaved", To: cli.id}
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
				if cli.id == msg.To {
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

func (c *Client) updatePosition(position Position) {
	c.Position.PageX = position.PageX
	c.Position.PageY = position.PageY
}

func (c *Client) readMessge(db *sql.DB) {
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

		if err = json.Unmarshal(message, &msg); err != nil {
			log.Printf("parse error : %s", err)
		}

		if msg.Type == "private" {
			insert(db, msg)
			private <- msg
		} else if msg.Type == "move" {
			c.updatePosition(msg.Position)
			position <- msg
		}
	}
}

func insert(db *sql.DB, msg Message) {
	res, err := db.Exec(
		"INSERT INTO messages (address, sender, text) VALUES (?, ?, ?)",
		msg.To,
		msg.From,
		msg.Msg,
	)
	if err != nil {
		log.Printf("insert db.Exec error err:%v", err)
	}
	fmt.Println(res)
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

func (s *Service) wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade failed %s ", err)
		return
	}

	db := s.db
	cli := NewClient(conn)
	// connected new client
	cli.conn.WriteJSON(Message{Type: "newclient", To: cli.id, Position: Position{PageX: cli.Position.PageX, PageY: cli.Position.PageY}})

	entering <- cli
	go cli.writeMessge()
	go cli.readMessge(db)
}

func NewClient(conn *websocket.Conn) *Client {
	uuid := uuid.NewString()
	rand.Seed(time.Now().UnixNano())
	pagex := float64(rand.Intn(1000))
	pagey := float64(rand.Intn(1000))
	return &Client{id: uuid, conn: conn, send: make(chan interface{}), Position: Position{PageX: pagex, PageY: pagey}}
}

func getEnvVariable(key string) string {
	return os.Getenv(key)
}

func loadEnvFile() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

type Service struct {
	db *sql.DB
}

func (s *Service) selectQuery(w http.ResponseWriter, r *http.Request) {
	var msgHistory Message
	body, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		log.Printf("read body error : %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err = json.Unmarshal(body, &msgHistory); err != nil {
		log.Printf("parse error : %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Println("Struct is:", msgHistory)
	w.WriteHeader(200)
}

func main() {
	loadEnvFile()

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/%s", getEnvVariable("DBUSER"), getEnvVariable("DBPW"), getEnvVariable("DBNAME")))
	if err != nil {
		panic(err)
	}

	// See "Important settings" section.
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	s := &Service{db: db}

	http.Handle("/", http.FileServer(http.Dir("root/")))
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	http.HandleFunc("/chat", s.wshandler)
	http.HandleFunc("/messages", s.selectQuery)
	go hub()

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
