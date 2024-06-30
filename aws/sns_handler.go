package aws

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"log"
	"net/http"
	"os"
	"strings"
)

type SNSHandler struct {
	snsClient *sns.SNS
}

func NewSNSHandler() *SNSHandler {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Endpoint: aws.String(os.Getenv("LOCALSTACK_ENDPOINT")),
			Region:   aws.String(os.Getenv("AWS_REGION")),
			//LogLevel: aws.LogLevel(aws.LogDebugWithHTTPBody),
		},
		SharedConfigState: session.SharedConfigEnable,
	}))
	return &SNSHandler{
		snsClient: sns.New(sess),
	}
}

func (h *SNSHandler) SubscribeToTopic(topicARN, protocol, endpoint string) (string, error) {
	params := &sns.SubscribeInput{
		Protocol: aws.String(protocol),
		Endpoint: aws.String(endpoint),
		TopicArn: aws.String(topicARN),
	}

	resp, err := h.snsClient.Subscribe(params)
	if err != nil {
		log.Printf("Error subscribing to topic %s: %v \n", topicARN, err)
		return "", err
	}

	return aws.StringValue(resp.SubscriptionArn), nil
}

func (h *SNSHandler) HandleSNSNotification(c *gin.Context) {
	var snsMessage snsMessage
	if err := json.NewDecoder(c.Request.Body).Decode(&snsMessage); err != nil {
		log.Printf("Error decoding SNS message: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bad Request"})
		return
	}

	switch snsMessage.Type {
	case "SubscriptionConfirmation":
		if err := h.handleSubscriptionConfirmation(snsMessage); err != nil {
			log.Printf("Error confirming subscription: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
			return
		}
		log.Println("Subscription confirmed successfully.")
	case "Notification":
		h.processSnsNotification(snsMessage)
	default:
		log.Printf("Unknown message type received: %s\n", snsMessage.Type)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown Message Type"})
		return
	}

	// Respond to SNS with HTTP status 200 to acknowledge receipt
	c.JSON(http.StatusOK, gin.H{"message": "Received SNS message"})
}

func (h *SNSHandler) handleSubscriptionConfirmation(snsMessage snsMessage) error {
	// Confirm subscription using the token
	params := &sns.ConfirmSubscriptionInput{
		Token:    aws.String(snsMessage.Token),
		TopicArn: aws.String(snsMessage.TopicArn),
	}

	_, err := h.snsClient.ConfirmSubscription(params)
	if err != nil {
		return err
	}

	return nil
}

func (h *SNSHandler) processSnsNotification(snsMessage snsMessage) {
	// Handle regular SNS notification
	log.Printf("Received SNS message: Type=%s, Token=%s, TopicArn=%s\n", snsMessage.Type, snsMessage.Token, snsMessage.TopicArn)
	messageBody := string(snsMessage.Body)
	log.Printf("Message Body: %s\n", messageBody)

	if strings.HasPrefix(messageBody, `"{`) && strings.HasSuffix(messageBody, `"}`) {
		messageBody = messageBody[1 : len(messageBody)-1] // Remove surrounding double quotes
	}
	// Parse Data -> into the parseData struct and send to socket servers
	parsedData, err := parseBody(messageBody)
	if err != nil {
		log.Printf("Error parsing message body: %v\n", err)
		return
	}
	log.Printf("Parsed Message: %+v\n", parsedData)
	if err = sendDataToServer(parsedData); err != nil {
		log.Printf("Error sending data: %v\n", err)
		return
	}
	log.Printf("Successfully sent data: %+v\n", parsedData)
}

func parseBody(body string) (map[string]interface{}, error) {
	var parsedData map[string]interface{}
	if err := json.Unmarshal([]byte(body), &parsedData); err != nil {
		// If direct unmarshalling fails, assume body is a JSON-encoded string and try to unmarshal it again
		var bodyStr string
		if err := json.Unmarshal([]byte(body), &bodyStr); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(bodyStr), &parsedData); err != nil {
			return nil, err
		}
	}
	return parsedData, nil
}

func sendDataToServer(data interface{}) error {
	url := os.Getenv("REDIRECT_URL")
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON data: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send data to server: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-200 status code: %v", resp.Status)
	}

	log.Println("Data sent successfully to server.")
	return nil
}

type snsMessage struct {
	Type     string          `json:"Type"`
	Token    string          `json:"Token"`
	TopicArn string          `json:"TopicArn"`
	Body     json.RawMessage `json:"Message"`
}

type ParsedMessage struct {
	Username string `json:"username"`
	Message  string `json:"message"`
	Property string `json:"property"`
	TrxID    string `json:"trx_id"`
}
