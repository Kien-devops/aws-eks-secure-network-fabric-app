package queue

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type SQSHandler struct {
	Client   *sqs.Client
	QueueURL string
}

func NewSQSHandler(ctx context.Context, region, queueURL string) (*SQSHandler, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config (Pod Identity): %w", err)
	}

	client := sqs.NewFromConfig(cfg)
	log.Printf("Initialized Amazon SQS client for queue: %s (Region: %s)", queueURL, region)
	return &SQSHandler{
		Client:   client,
		QueueURL: queueURL,
	}, nil
}

func (s *SQSHandler) PublishOrderEvent(ctx context.Context, orderID, payloadJSON string) error {
	input := &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.QueueURL),
		MessageBody: aws.String(payloadJSON),
	}

	_, err := s.Client.SendMessage(ctx, input)
	if err != nil {
		return fmt.Errorf("failed sending SQS message: %w", err)
	}
	log.Printf("Published OrderCreated event for order [%s] to SQS", orderID)
	return nil
}

func (s *SQSHandler) StartConsumer(ctx context.Context, workerID string) {
	log.Printf("Starting SQS background worker [%s] on queue %s", workerID, s.QueueURL)
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Printf("SQS worker [%s] shutting down", workerID)
				return
			default:
				result, err := s.Client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
					QueueUrl:            aws.String(s.QueueURL),
					MaxNumberOfMessages: 5,
					WaitTimeSeconds:     20, // Long polling
					VisibilityTimeout:   120,
				})
				if err != nil {
					log.Printf("Worker [%s] receive error: %v. Retrying in 5s...", workerID, err)
					time.Sleep(5 * time.Second)
					continue
				}

				for _, msg := range result.Messages {
					log.Printf("Worker [%s] processing message ID: %s", workerID, *msg.MessageId)
					// Giả lập xử lý thanh toán / inventory async idempotency
					time.Sleep(200 * time.Millisecond)

					// Xóa message sau khi xử lý thành công
					_, delErr := s.Client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
						QueueUrl:      aws.String(s.QueueURL),
						ReceiptHandle: msg.ReceiptHandle,
					})
					if delErr != nil {
						log.Printf("Worker [%s] failed to delete message: %v", workerID, delErr)
					} else {
						log.Printf("Worker [%s] acknowledged & deleted message: %s", workerID, *msg.MessageId)
					}
				}
			}
		}
	}()
}

