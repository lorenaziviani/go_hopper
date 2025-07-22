package main

import (
	"log"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	log.Println("Gohopper Consumer starting...")

	// TODO: Implement consumer logic
	// - Connect to RabbitMQ
	// - Configure worker pool
	// - Consume messages
	// - Process with retry and DLQ

	log.Println("Consumer configured successfully!")

	select {}
}
