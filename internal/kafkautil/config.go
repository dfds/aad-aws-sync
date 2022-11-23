package kafkautil

// ConsumerConfig allows one to configure a Kafka consumer using
// environment variables.
type ConsumerConfig struct {
	Brokers []string `required:"true"`
	GroupID string   `envconfig:"group_id",required:"true"`
	Topic   string   `required:"true"`
}

// ProducerConfig allows one to configure a Kafka producer using
// environment variables.
type ProducerConfig struct {
	Brokers []string `required:"true"`
	Topic   string   `required:"true"`
}

// AuthSASLPlainConfig allows one to configure auth with a plain SASL
// authnetication mechanism to the Kafka brokers.
type AuthSASLPlainConfig struct {
	Username string `required:"true"`
	Password string `required:"true"`
}
