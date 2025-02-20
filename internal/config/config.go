package config

import (
	"fmt"
	"regexp"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
)

type Config struct {
	Settings        Settings                  `toml:"settings"`
	WebhookServices map[string]WebhookService `toml:"webhook_services" validate:"dive"`
}

type Settings struct {
	TickerEnabled  bool `toml:"ticker_enabled"`
	TickerInterval int  `toml:"ticker_interval" validate:"required_if=TickerEnabled true,gte=1"`
	ListenerPort   int  `toml:"listener_port" validate:"required,gte=1,lte=65535"`
	AdminPort      int  `toml:"admin_port" validate:"required,gte=1,lte=65535"`
}

type WebhookService struct {
	Name                 string               // Populated from map key
	Path                 string               `toml:"path" validate:"omitempty,alphanum"`
	AuthenticationType   string               `toml:"authentication_type" validate:"omitempty,oneof=header"`
	AuthenticationHeader string               `toml:"authentication_header"`
	AuthenticationSecret string               `toml:"authentication_secret"`
	Forwarders           map[string]Forwarder `toml:"forwarders" validate:"dive"`
}

type Forwarder struct {
	Name       string            // Populated from map key
	Hash       string            // Populated during instantiation. This will be used to cache any forwarder connections in memory.
	Type       string            `toml:"type" validate:"required,oneof=http amqp"`
	URL        string            `toml:"url" validate:"required_if=Type http,omitempty,url"`
	Headers    map[string]string `toml:"headers"`
	RetryCount int               `toml:"retry_count" validate:"gte=0"`
	RetryDelay string            `toml:"retry_delay" validate:"oneof=exponential fixed"`

	// AMQP specific fields
	ConnectionURL string `toml:"connection_url" validate:"required_if=Type amqp,omitempty,url"`
	Exchange      string `toml:"exchange" validate:"required_if=Type amqp"`
	RoutingKey    string `toml:"routing_key" validate:"required_if=Type amqp"`
	Queue         string `toml:"queue" validate:"required_if=Type amqp"`
	Durable       bool   `toml:"durable"`
	Persistent    bool   `toml:"persistent"`
	ExchangeType  string `toml:"exchange_type" validate:"required_if=Type amqp,omitempty,oneof=direct fanout topic headers"`
	AutoDelete    bool   `toml:"auto_delete"`
	Exclusive     bool   `toml:"exclusive"`
	NoWait        bool   `toml:"no_wait"`
	Internal      bool   `toml:"internal"`  // For exchanges
	Mandatory     bool   `toml:"mandatory"` // For publishing
	Immediate     bool   `toml:"immediate"` // For publishing
}

func loadConfig(path string) (*Config, error) {
	config := &Config{
		Settings: Settings{
			// Default settings that work well for most deployments
			ListenerPort:   8080,
			AdminPort:      8081,
			TickerEnabled:  true,
			TickerInterval: 5, // 5 second interval for retries
		},
		WebhookServices: make(map[string]WebhookService),
	}

	// Read TOML file
	if _, err := toml.DecodeFile(path, config); err != nil {
		return nil, err
	}

	// Create validator
	validate := validator.New()

	// Register custom validation for path
	if err := validate.RegisterValidation("alphanum", validateAlphanumeric); err != nil {
		return nil, err
	}

	// Populate Name fields from map keys and set defaults for each service
	for serviceName, service := range config.WebhookServices {
		service.Name = serviceName

		// Set default forwarder values if not specified
		for forwarderName, forwarder := range service.Forwarders {
			forwarder.Name = forwarderName
			forwarder.Hash = generateUniqueName(serviceName, forwarderName)

			// Set sensible defaults for forwarder
			if forwarder.RetryCount == 0 {
				forwarder.RetryCount = 3 // Default to 3 retries
			}
			if forwarder.RetryDelay == "" {
				forwarder.RetryDelay = "exponential" // Default to exponential backoff
			}

			// AMQP defaults for reliability
			if forwarder.Type == "amqp" {
				if !forwarder.Durable {
					forwarder.Durable = true // Default to durable queues
				}
				if !forwarder.Persistent {
					forwarder.Persistent = true // Default to persistent messages
				}
				if forwarder.ExchangeType == "" {
					forwarder.ExchangeType = "direct" // Default exchange type
				}
			}

			service.Forwarders[forwarderName] = forwarder
		}
		config.WebhookServices[serviceName] = service
	}

	// Validate the config
	if err := validate.Struct(config); err != nil {
		return nil, err
	}

	return config, nil
}

// Custom validator for alphanumeric values
func validateAlphanumeric(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return true // empty is valid
	}
	matched, _ := regexp.MatchString("^[a-zA-Z0-9]+$", value)
	return matched
}

func LoadMainConfig() (*Config, error) {
	config, err := loadConfig("webhook_config.toml")
	if err != nil {
		panic(err)
	}

	// Access config values
	for serviceName, service := range config.WebhookServices {
		// Process each webhook service and its forwarders
		println(serviceName, service.Path)
		for _, forwarder := range service.Forwarders {
			println(forwarder.Type)
		}
	}
	return config, nil
}

func generateUniqueName(serviceName string, forwarderName string) string {
	return fmt.Sprintf("%s-%s", serviceName, forwarderName)
}
