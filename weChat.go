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

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Service struct {
	db *sql.DB
}

type Client struct {
	hub      *Hub
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

type Hub struct {
	entering chan *Client
	leaving  chan *Client
	clients  map[*Client]bool
	private  chan Message
	position chan Message
}

func newHub() *Hub {
	return &Hub{
		entering: make(chan *Client),
		leaving:  make(chan *Client),
		clients:  make(map[*Client]bool),
		private:  make(chan Message),
		position: make(chan Message),
	}
}

func (h *Hub) sendToAllUsers(msg Message) {
	for client := range h.clients {
		client.send <- msg
	}
}

func (h *Hub) sendToOneUser(msg Message) {
	for client := range h.clients {
		if client.id == msg.To {
			client.send <- msg
		}
	}
}

func (c *Client) createMSG(msgType string) Message {
	return Message{
		Type: msgType,
		To:   c.id,
		Position: Position{
			PageX: c.Position.PageX,
			PageY: c.Position.PageY,
		},
	}
}

func (h *Hub) run() {
	for {
		select {
		case newcli := <-h.entering:
			var newUserLocation Message = newcli.createMSG("move")
			for cli := range h.clients {
				var userLocation Message = cli.createMSG("move")
				cli.send <- newUserLocation
				newcli.send <- userLocation
			}
			h.clients[newcli] = true

		case cli := <-h.leaving:
			if _, ok := h.clients[cli]; ok {
				delete(h.clients, cli)
				close(cli.send)
				msg := cli.createMSG("leaved")
				h.sendToAllUsers(msg)
			}
		case msg := <-h.private:
			h.sendToOneUser(msg)
		case msg := <-h.position:
			h.sendToAllUsers(msg)
		}
	}
}

func (c *Client) updatePosition(position Position) {
	c.Position.PageX = position.PageX
	c.Position.PageY = position.PageY
}

func (c *Client) readMessge(db *sql.DB) {
	defer func() {
		c.hub.leaving <- c
		c.conn.Close()
	}()

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
			if err = storeTheMessage(db, &msg); err != nil {
				c.conn.WriteJSON(msg)
			} else {
				c.hub.private <- msg
			}
		} else if msg.Type == "move" {
			c.updatePosition(msg.Position)
			c.hub.position <- msg
		}
	}
}

func storeTheMessage(db *sql.DB, msg *Message) error {
	utc := time.Now().UTC()

	_, err := db.Exec(
		"INSERT INTO messages (address, sender, text, dt) VALUES (?, ?, ?, ?)",
		msg.To,
		msg.From,
		msg.Msg,
		utc,
	)
	if err != nil {
		log.Printf("insert db.Exec error err:%v", err)
		msg.Type = "Error"
		msg.Msg = "failed to send your message"
	}

	return err
}

func (c *Client) writeMessge() {
	defer c.conn.Close()

	for {
		select {
		case message, ok := <-c.send:
			log.Println(message, ok)

			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteJSON(message)
		}
	}
}

func (s *Service) wshandler(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade failed %s ", err)
		return
	}

	db := s.db
	cli := NewClient(conn, hub)
	msg := cli.createMSG("newclient")
	cli.conn.WriteJSON(msg)

	cli.hub.entering <- cli
	go cli.writeMessge()
	go cli.readMessge(db)
}

func NewClient(conn *websocket.Conn, hub *Hub) *Client {
	uuid := uuid.NewString()
	rand.Seed(time.Now().UnixNano())

	pagex := float64(rand.Intn(1000))
	pagey := float64(rand.Intn(1000))

	return &Client{
		hub:  hub,
		id:   uuid,
		conn: conn,
		send: make(chan interface{}),
		Position: Position{
			PageX: pagex,
			PageY: pagey,
		},
	}
}

func getEnv(key string) string {
	return os.Getenv(key)
}

func loadEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func (s *Service) messageHistoryByUser(msg Message) ([]Message, error) {
	rows, err := s.db.Query("select text, sender from messages where address in (?, ?) and sender in (?, ?) order by dt",
		msg.To,
		msg.From,
		msg.From,
		msg.To,
	)

	if err != nil {
		return nil, fmt.Errorf("messageHistoryByUser %q, %q: %v",
			msg.To,
			msg.From,
			err,
		)
	}
	defer rows.Close()

	messages := make([]Message, 0)
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.Msg, &msg.From); err != nil {
			return nil, fmt.Errorf("messageHistoryByUser %q, %q: %v",
				msg.To,
				msg.From,
				err,
			)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("messageHistoryByUser %q, %q: %v",
			msg.To,
			msg.From,
			err,
		)
	}
	return messages, nil
}

func (s *Service) getMessageHistory(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		log.Printf("read body error : %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var msg Message
	if err = json.Unmarshal(body, &msg); err != nil {
		log.Printf("parse error : %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	messages, err := s.messageHistoryByUser(msg)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	json, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		log.Printf("JSON marshaling failed: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Printf("%s\n", json)
	w.WriteHeader(200)
}

func connectDB() *sql.DB {
	dbname := getEnv("DBNAME")

	if getEnv("MODE") == "test" {
		dbname = getEnv("DBNAME_TEST")
	}

	cfg := mysql.Config{
		User:   getEnv("DBUSER"),
		Passwd: getEnv("DBPW"),
		DBName: dbname,
	}

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}

	if pingErr := db.Ping(); pingErr != nil {
		log.Fatal(pingErr)
	}

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	fmt.Println("Connected!")
	return db
}

func main() {
	loadEnv()

	db := connectDB()

	s := &Service{db: db}
	hub := newHub()
	go hub.run()

	http.Handle("/", http.FileServer(http.Dir("root/")))
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		s.wshandler(hub, w, r)
	})

	http.HandleFunc("/messages", s.getMessageHistory)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
