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
	send chan []byte
}

type Message struct {
	Addr string `json:"addr"`
	Msg  string `json:"msg"`
}

var (
	entering = make(chan Client)
	leaving  = make(chan Client)
)

var clients = make(map[Client]bool)

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
			log.Println(err)
		}
	}
	entering <- *NewClient(conn)

	ch := make(chan Message)
	go broadCast(ch)
	go readMessge(conn, ch)
}

func readMessge(conn *websocket.Conn, ch chan Message) {
	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println(err)
			return
		}
		ch <- msg
	}
}

func broadCast(ch chan Message) {
	for msg := range ch {
		for cli := range clients {
			if err := cli.conn.WriteJSON(msg); err != nil {
				log.Println(err)
			}
		}
	}
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	go usermanage()
	go handleConn(conn)

}

func NewClient(conn *websocket.Conn) *Client {
	uuid := uuid.NewString()
	return &Client{id: uuid, conn: conn, send: make(chan []byte)}
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
