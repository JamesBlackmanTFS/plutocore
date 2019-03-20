package shared

import (
	"github.com/gorilla/websocket"
	"github.com/matryer/goblueprints/chapter1/trace"
)

type Room struct {

	// Forward is a channel that holds incoming messages
	// that should be Forwarded to the other clients.
	Forward chan []byte

	// join is a channel for clients wishing to join the room.
	join chan *client

	// leave is a channel for clients wishing to leave the room.
	leave chan *client

	// clients holds all current clients in this room.
	clients map[*client]bool

	// tracer will receive trace information of activity
	// in the room.
	tracer trace.Tracer
}

// newRoom makes a new room that is ready to
// go.
func NewRoom() *Room {
	return &Room{
		Forward: make(chan []byte),
		join:    make(chan *client),
		leave:   make(chan *client),
		clients: make(map[*client]bool),
		tracer:  trace.Off(),
	}
}

func (r *Room) Run() {
	for {
		select {
		case client := <-r.join:
			// joining
			r.clients[client] = true
			r.tracer.Trace("New client joined")
		case client := <-r.leave:
			// leaving
			delete(r.clients, client)
			close(client.send)
			r.tracer.Trace("Client left")
		case msg := <-r.Forward:
			r.tracer.Trace("Message received: ", string(msg))
			// Forward message to all clients
			for client := range r.clients {
				client.send <- msg
				r.tracer.Trace(" -- sent to client")
			}
		}
	}
}

const (
	socketBufferSize  = 1024
	messageBufferSize = 256
)

var upgrader = &websocket.Upgrader{ReadBufferSize: socketBufferSize, WriteBufferSize: socketBufferSize}

// func (r *Room) ServeHTTP(w http.ResponseWriter, req *http.Request) {
// 	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
// 	socket, err := upgrader.Upgrade(w, req, nil)
// 	if err != nil {
// 		log.Fatal("ServeHTTP:", err)
// 		return
// 	}
// 	client := &client{
// 		socket: socket,
// 		send:   make(chan []byte, messageBufferSize),
// 		room:   r,
// 	}
// 	r.join <- client
// 	defer func() { r.leave <- client }()
// 	go client.write()
// 	client.read()
// }
