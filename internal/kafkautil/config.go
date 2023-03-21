package kafkautil

// ConsumerConfig allows one to configure a Kafka consumer using
// environment variables.
type ConsumerConfig struct {
	GroupID string `envconfig:"group_id",required:"true"`
	Topic   string `required:"true"`
}

// ProducerConfig allows one to configure a Kafka producer using
// environment variables.
type ProducerConfig struct {
	Topic string `required:"true"`
}

// AuthConfig allows one to configure auth with a plain SASL
// authnetication mechanism to the Kafka brokers.
type AuthConfig struct {
	Brokers          []string          `required:"true"`
	Mechanism        string            `required:"true"`
	MechanismOptions map[string]string `envconfig:"MECHANISM_OPTIONS"`
	Tls              bool              `required:"true"`
}
