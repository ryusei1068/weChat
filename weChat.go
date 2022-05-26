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
	send chan []byte
}

var (
	entering = make(chan Client)
	leaving  = make(chan Client)
	messages = make(chan []byte)
)

var clients = make(map[Client]bool)

func broadcaster() {

	for {
		select {
		case msg := <-messages:
			for cli := range clients {
				cli.send <- msg
			}

		case cli := <-entering:
			clients[cli] = true

		case cli := <-leaving:
			delete(clients, cli)
			close(cli.send)
		}
	}
}

func handleConn(conn *websocket.Conn) {
	cli := *NewClient(conn)
	who := conn.RemoteAddr().String()
	messages <- []byte(who + "has arrived")
	entering <- *NewClient(conn)

	go clientWriter(conn, cli.send)

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Println(messageType)
		cli.send <- message
	}
}

func clientWriter(conn *websocket.Conn, ch chan []byte) {
	for msg := range ch {
		for cli := range clients {
			if err := cli.conn.WriteMessage(1, msg); err != nil {
				log.Println(err)
			}
		}
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World")
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	go broadcaster()
	go handleConn(conn)

}

func NewClient(conn *websocket.Conn) *Client {
	uuid := uuid.New().String()
	return &Client{id: uuid, conn: conn, send: make(chan []byte)}
}

func main() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/chat", wshandler)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
