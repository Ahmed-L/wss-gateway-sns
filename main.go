package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"net/http"
)

var (
	localstackEndpoint string
	region             string
	queueName          string
	waitTimeSeconds    int
	visibilityTimeout  int
)

type MessageFormat struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	localstackEndpoint = os.Getenv("LOCALSTACK_ENDPOINT")
	region = os.Getenv("AWS_REGION")
	queueName = os.Getenv("SQS_QUEUE_NAME")
	waitTimeSeconds, err = strconv.Atoi(os.Getenv("SQS_LONG_POLL_WAIT_TIME"))
	if err != nil {
		log.Fatalf("Error parsing SQS_LONG_POLL_WAIT_TIME: %v", err)
	}

	visibilityTimeout, err = strconv.Atoi(os.Getenv("SQS_VISIBILITY_TIMEOUT"))
	if err != nil {
		log.Fatalf("Error parsing SQS_VISIBILITY_TIMEOUT: %v", err)
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			Endpoint: aws.String(localstackEndpoint),
			Region:   aws.String(region),
		},
	}))

	// Create an SQS service client
	svc := sqs.New(sess)
	// Start listening to SQS messages
	listenSQSMessages(svc)

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down...")

	log.Println("Server stopped gracefully")
}

// listenSQSMessages starts listening to messages from SQS queue
func listenSQSMessages(svc sqsiface.SQSAPI) {
	queueURLResult, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		log.Fatalf("Failed to get SQS queue URL: %v", err)
	}
	queueURL := queueURLResult.QueueUrl

	for {
		// Receive messages from SQS queue with long polling
		receiveParams := &sqs.ReceiveMessageInput{
			QueueUrl:            queueURL,
			MaxNumberOfMessages: aws.Int64(10),
			WaitTimeSeconds:     aws.Int64(int64(waitTimeSeconds)),
			VisibilityTimeout:   aws.Int64(int64(visibilityTimeout)),
		}

		resp, err := svc.ReceiveMessage(receiveParams)
		if err != nil {
			log.Printf("Failed to receive messages from SQS: %v", err)
			continue
		}

		// Process received messages
		for _, msg := range resp.Messages {
			formattedMessage := formatSQSMessages(msg.Body)

			// Send message to another server
			err := sendLogEntry(formattedMessage)
			if err != nil {
				log.Printf("Failed to send message to server: %v", err)
				continue
			}

			// Delete the message from SQS after processing
			_, err = svc.DeleteMessage(&sqs.DeleteMessageInput{
				QueueUrl:      queueURL,
				ReceiptHandle: msg.ReceiptHandle,
			})
			if err != nil {
				log.Printf("Failed to delete message %s: %v\n", *msg.MessageId, err)
				continue
			}
			log.Printf("Deleted message: %s\n", *msg.MessageId)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func sendLogEntry(msg MessageFormat) error {
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	serverURL := os.Getenv("REDIRECT_URL")
	resp, err := http.Post(serverURL, "application/json", bytes.NewBuffer(msgJSON))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-200 status code: %v", resp.StatusCode)
	}

	return nil
}

func formatSQSMessages(msgBody *string) MessageFormat {
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(*msgBody), &body); err != nil {
		log.Printf("Failed to parse message body: %v\n", err)
		return MessageFormat{}
	}

	message, ok := body["Message"].(string)
	if !ok {
		log.Printf("Message field not found or not a string\n")
		return MessageFormat{}
	}

	timestampStr, ok := body["Timestamp"].(string)
	if !ok {
		log.Printf("Timestamp field not found or not a string\n")
		return MessageFormat{}
	}

	timestamp, err := time.Parse(time.RFC3339Nano, timestampStr)
	if err != nil {
		log.Printf("Failed to parse timestamp: %v\n", err)
		return MessageFormat{}
	}
	msg := MessageFormat{
		Message:   message,
		Timestamp: timestamp,
	}
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		log.Fatalf("Failed to marshal LogEntry to JSON: %v", err)
	}
	log.Printf("%s\n", msgJSON)
	return msg
}
