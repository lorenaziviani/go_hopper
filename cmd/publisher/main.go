package main

import (
	"log"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	log.Println("Gohopper Publisher starting...")

	// TODO: Implement publisher logic
	// - Connect to RabbitMQ
	// - Configure exchange and queues
	// - Publish messages

	log.Println("Publisher configured successfully!")

	select {}
}
