package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"websocket-gateway/aws"

	"github.com/gin-gonic/gin"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	router := gin.Default()
	snsHandler := aws.NewSNSHandler()
	setupRoutes(router, snsHandler)
	//setupSnsSubscription(snsHandler, os.Getenv("SNS_TOPIC_ARN"), os.Getenv("SNS_NOTIFICATION_ENDPOINT")) // for local testing only
	port := os.Getenv("PORT")
	fmt.Printf("Starting WebSocket Gateway Server on port %s\n", port)

	go handleSignals()

	if err := router.Run(port); err != nil {
		log.Fatalf("Failed to start WebSocket Gateway Server: %v", err)
	}
}

func setupRoutes(router *gin.Engine, snsHandler *aws.SNSHandler) {
	router.GET("/health", healthCheck)
	router.POST("/sns-webhook", snsHandler.HandleSNSNotification)
}

func setupSnsSubscription(snsHandler *aws.SNSHandler, topicARN string, endpoint string) {
	subscriptionArn, err := snsHandler.SubscribeToTopic(topicARN, "https", endpoint)
	if err != nil {
		log.Fatalf("Failed to subscribe to SNS topic: %v", err)
	}
	log.Printf("Subscribed to SNS topic successfully. Subscription ARN: %s", subscriptionArn)

}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, "OK")
}

func handleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	fmt.Println("\nShutting down WebSocket Gateway Server...")

	fmt.Println("WebSocket Gateway Server gracefully stopped.")
	os.Exit(0)
}
