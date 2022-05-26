package main

import (
	"fmt"
	"log"
	"net/http"

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

type client chan<- []byte

var (
	entering = make(chan client)
	leaving  = make(chan client)
	messages = make(chan []byte)
)

func broadcaster() {
	clients := make(map[client]bool)

	for {
		select {
		case msg := <-messages:
			for cli := range clients {
				cli <- msg
			}

		case cli := <-entering:
			clients[cli] = true

		case cli := <-leaving:
			delete(clients, cli)
			close(cli)
		}
	}
}

func handleConn(conn *websocket.Conn) {
	ch := make(chan []byte)
	go clientWriter(conn, ch)

	who := conn.RemoteAddr().String()
	messages <- []byte(who + "has arrived")
	entering <- ch
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Println(messageType)
		ch <- message
	}

	// leaving <- ch
	// messages <- []byte(who + " has left")
	// defer conn.Close()
}

func clientWriter(conn *websocket.Conn, ch chan []byte) {
	for msg := range ch {
		if err := conn.WriteMessage(1, msg); err != nil {
			log.Println(err)
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

// func NewClient(conn *websocket.Conn) *Client {
// 	uuid := uuid.New().String()
// 	return &Client{id: uuid, conn: conn, send: make(chan []byte)}
// }

func main() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/chat", wshandler)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
