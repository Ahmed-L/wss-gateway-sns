#!/bin/bash

# LocalStack endpoint and AWS region
ENDPOINT_URL="http://localhost:4566"
AWS_REGION="us-east-1"  # LocalStack uses a default region

# SNS Topic ARN (replace with your topic ARN)
SNS_TOPIC_ARN="arn:aws:sns:us-east-1:000000000000:sample-topic"

# JSON message data
JSON_DATA='{
  "message": "Hello from Bash script!",
  "username": "john_doe",
  "trx_id": "abc123",
  "property": "additional_field_value"
}'


# Publish message to SNS topic using awslocal CLI
awslocal sns publish \
    --topic-arn $SNS_TOPIC_ARN \
    --message "$JSON_DATA" \
    --endpoint-url $ENDPOINT_URL \
    --region $AWS_REGION
