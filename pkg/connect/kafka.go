package connect

import (
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/spf13/viper"
)

func getBrokers() []string {
	brokers := viper.GetStringSlice("kafka.brokers")
	if len(brokers) == 0 {
		return nil
	}
	return brokers
}

func newSASLMechanism() *plain.Mechanism {
	username := viper.GetString("kafka.username")
	password := viper.GetString("kafka.password")
	if username == "" || password == "" {
		return nil
	}
	return &plain.Mechanism{
		Username: username,
		Password: password,
	}
}

func ConnectKafkaProducer() (*kafka.Writer, error) {
	brokers := getBrokers()
	if len(brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers未配置")
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    1,
		BatchTimeout: 0,
		RequiredAcks: kafka.RequireAll,
		MaxAttempts:  5,
	}
	mechanism := newSASLMechanism()
	if mechanism != nil {
		writer.Transport = &kafka.Transport{
			SASL: mechanism,
		}
	}
	return writer, nil
}

func ConnectKafkaConsumerGroup(groupID string, topic string) (*kafka.Reader, error) {
	brokers := getBrokers()
	if len(brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers未配置")
	}
	readerConfig := kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    topic,
		MinBytes: 1,
		MaxBytes: 10e6,
	}
	mechanism := newSASLMechanism()
	if mechanism != nil {
		readerConfig.Dialer = &kafka.Dialer{
			SASLMechanism: mechanism,
		}
	}
	reader := kafka.NewReader(readerConfig)
	return reader, nil
}
