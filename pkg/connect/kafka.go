package connect

import (
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/spf13/viper"
)

func getBrokers(v *viper.Viper) []string {
	brokers := v.GetStringSlice("kafka.brokers")
	if len(brokers) == 0 {
		return nil
	}
	return brokers
}

func newSASLMechanism(v *viper.Viper) *plain.Mechanism {
	username := v.GetString("kafka.username")
	password := v.GetString("kafka.password")
	if username == "" || password == "" {
		return nil
	}
	return &plain.Mechanism{
		Username: username,
		Password: password,
	}
}

func ConnectKafkaProducer(v *viper.Viper) (*kafka.Writer, error) {
	brokers := getBrokers(v)
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
	mechanism := newSASLMechanism(v)
	if mechanism != nil {
		writer.Transport = &kafka.Transport{
			SASL: mechanism,
		}
	}
	return writer, nil
}

func ConnectKafkaConsumerGroup(v *viper.Viper, groupID string, topic string) (*kafka.Reader, error) {
	brokers := getBrokers(v)
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
	mechanism := newSASLMechanism(v)
	if mechanism != nil {
		readerConfig.Dialer = &kafka.Dialer{
			SASLMechanism: mechanism,
		}
	}
	reader := kafka.NewReader(readerConfig)
	return reader, nil
}
