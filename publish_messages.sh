#!/bin/bash

# Replace with your SNS topic ARN in LocalStack
sns_topic_arn="arn:aws:sns:us-east-1:000000000000:my-topic"

# Loop to publish messages every 0.5 seconds for 5 seconds
for ((i=0; i<10; i++)); do
    message="Message $i from Bash script"

    # Publish message to SNS using AWS CLI
    aws sns publish --topic-arn $sns_topic_arn --message "$message"

    echo "Published: $message"

    # Sleep for 0.5 seconds
    sleep 0.5
done
