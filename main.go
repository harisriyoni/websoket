package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type Message struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

type Client struct {
	Username string
	Conn     *websocket.Conn
}

type ChatRoom struct {
	clients    []*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan Message
}

func NewChatRoom() *ChatRoom {
	return &ChatRoom{
		clients:    make([]*Client, 0),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Message),
	}
}

func (cr *ChatRoom) Run() {
	for {
		select {
		case client := <-cr.register:
			cr.clients = append(cr.clients, client)
			go cr.broadcastMessage(Message{
				Username: "Server",
				Content:  fmt.Sprintf("User %s joined the chat", client.Username),
			})
		case client := <-cr.unregister:
			for i, c := range cr.clients {
				if c == client {
					cr.clients = append(cr.clients[:i], cr.clients[i+1:]...)
					go cr.broadcastMessage(Message{
						Username: "Server",
						Content:  fmt.Sprintf("User %s left the chat", client.Username),
					})
					break
				}
			}
		case message := <-cr.broadcast:
			for _, client := range cr.clients {
				go func(c *Client) {
					if err := c.Conn.WriteJSON(message); err != nil {
						log.Println("Error broadcasting message:", err)
					}
				}(client)
			}
		}
	}
}

func (cr *ChatRoom) broadcastMessage(message Message) {
	cr.broadcast <- message
}

func main() {
	chatRoom := NewChatRoom()
	go chatRoom.Run()

	app := fiber.New()

	app.Static("/", "./home.html")

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		username := c.Query("username")
		client := &Client{
			Username: username,
			Conn:     c,
		}
		chatRoom.register <- client

		defer func() {
			chatRoom.unregister <- client
			c.Close()
		}()

		for {
			var message Message
			err := c.ReadJSON(&message)
			if err != nil {
				log.Println("Error reading message:", err)
				break
			}

			chatRoom.broadcastMessage(message)
		}
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("./home.html")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Fatal(app.Listen(":" + port))
}
