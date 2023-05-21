package main

import (
	"flag"
	"log"

	"github.com/aiteung/musik"
	"github.com/gofiber/fiber"
	"github.com/gofiber/websocket"
)

type client struct {
	Username string
} // Add more data to this type if needed

var clients = make(map[*websocket.Conn]*client)

// Note: although large maps with pointer-like types (e.g. strings) as keys are slow, using pointers themselves as keys is acceptable and fast
var register = make(chan *websocket.Conn)
var broadcast = make(chan string)
var unregister = make(chan *websocket.Conn)

func runHub() {
	for {
		select {
		case connection := <-register:
			// Read the username from the client
			_, usernameBytes, err := connection.ReadMessage()
			if err != nil {
				log.Println("read error:", err)
				return
			}

			// Create a new client with the username
			username := string(usernameBytes)
			newClient := &client{
				Username: username,
			}
			clients[connection] = newClient
			log.Println("connection registered with username:", username)

		case message := <-broadcast:
			log.Println("message received:", message)

			// Send the message to all clients
			for connection, client := range clients {
				usernameMessage := "[" + client.Username + "]: " + message
				if err := connection.WriteMessage(websocket.TextMessage, []byte(usernameMessage)); err != nil {
					log.Println("write error:", err)

					unregister <- connection
					connection.WriteMessage(websocket.CloseMessage, []byte{})
					connection.Close()
				}
			}

		case connection := <-unregister:
			// Remove the client from the hub
			delete(clients, connection)

			log.Println("connection unregistered")
		}
	}
}

func main() {
	app := fiber.New()

	app.Static("/", "./home.html")

	app.Use(func(c *fiber.Ctx) {
		if websocket.IsWebSocketUpgrade(c) { // Returns true if the client requested upgrade to the WebSocket protocol
			c.Next()
		}
	})

	go runHub()

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		// When the function returns, unregister the client and close the connection
		defer func() {
			unregister <- c
			c.Close()
		}()

		// Register the client
		register <- c

		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Println("read error:", err)
				}

				return // Calls the deferred function, i.e. closes the connection on error
			}

			if messageType == websocket.TextMessage {
				// Broadcast the received message
				broadcast <- string(message)
			} else {
				log.Println("websocket message received of type", messageType)
			}
		}
	}))

	addr := flag.String("addr", musik.Dangdut(), "http service address")
	flag.Parse()
	app.Listen(*addr)
}
