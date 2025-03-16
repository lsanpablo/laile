package config

import (
	"fmt"
	"net"
	"regexp"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
)

type Config struct {
	Settings        Settings                  `toml:"settings"`
	Database        DatabaseConfig            `toml:"database"         validate:"required"`
	WebhookServices map[string]WebhookService `toml:"webhook_services" validate:"dive"`
}

type Settings struct {
	TickerEnabled                         bool `toml:"ticker_enabled"`
	TickerInterval                        int  `toml:"ticker_interval" validate:"required_if=TickerEnabled true,gte=1"`
	ListenerPort                          int  `toml:"listener_port"   validate:"required,gte=1,lte=65535"`
	AdminPort                             int  `toml:"admin_port"      validate:"required,gte=1,lte=65535"`
	RunBackgroundWorkerWithListenerServer bool `toml:"run_background_worker_with_listener_server"`
}

type DatabaseConfig struct {
	Host              string        `toml:"host"                validate:"required"`
	Port              string        `toml:"port"                validate:"required"`
	Username          string        `toml:"username"            validate:"required"`
	Password          string        `toml:"password"`
	Database          string        `toml:"database"            validate:"required"`
	MaxConns          int32         `toml:"max_conns"           validate:"omitempty,gte=1"`
	MinConns          int32         `toml:"min_conns"           validate:"omitempty,gte=0"`
	MaxConnLifetime   string        `toml:"max_conn_lifetime"   validate:"omitempty"`
	MaxConnIdleTime   string        `toml:"max_conn_idle_time"  validate:"omitempty"`
	HealthCheckPeriod string        `toml:"health_check_period" validate:"omitempty"`
	SSLMode           string        `toml:"ssl_mode"            validate:"omitempty,oneof=disable require verify-ca verify-full"`
	ConnectTimeout    time.Duration `toml:"connect_timeout"     validate:"omitempty"`
}

type WebhookService struct {
	Name                 string               // Populated from map key
	Path                 string               `toml:"path"                  validate:"omitempty,alphanum"`
	AuthenticationType   string               `toml:"authentication_type"   validate:"omitempty,oneof=header"`
	AuthenticationHeader string               `toml:"authentication_header"`
	AuthenticationSecret string               `toml:"authentication_secret"`
	Forwarders           map[string]Forwarder `toml:"forwarders"            validate:"dive"`
}

type Forwarder struct {
	Name string // Populated from map key
	// Hash is populated during instantiation. This will be used to cache any forwarder connections in memory.
	Hash       string
	Type       string            `toml:"type"        validate:"required,oneof=http amqp"`
	URL        string            `toml:"url"         validate:"required_if=Type http,omitempty,url"`
	Headers    map[string]string `toml:"headers"`
	RetryCount int               `toml:"retry_count" validate:"gte=0"`
	RetryDelay string            `toml:"retry_delay" validate:"oneof=exponential fixed"`

	// AMQP specific fields
	ConnectionURL string `toml:"connection_url" validate:"required_if=Type amqp,omitempty,url"`
	Exchange      string `toml:"exchange"       validate:"required_if=Type amqp"`
	RoutingKey    string `toml:"routing_key"    validate:"required_if=Type amqp"`
	Queue         string `toml:"queue"          validate:"required_if=Type amqp"`
	Durable       bool   `toml:"durable"`
	Persistent    bool   `toml:"persistent"`
	ExchangeType  string `toml:"exchange_type"  validate:"required_if=Type amqp,omitempty,oneof=direct fanout topic headers"`
	AutoDelete    bool   `toml:"auto_delete"`
	Exclusive     bool   `toml:"exclusive"`
	NoWait        bool   `toml:"no_wait"`
	Internal      bool   `toml:"internal"`  // For exchanges
	Mandatory     bool   `toml:"mandatory"` // For publishing
	Immediate     bool   `toml:"immediate"` // For publishing
}

const (
	DefaultAdminPort    = 8081
	DefaultListenerPort = 8080

	// DefaultTickerInterval is the default interval for the event ticker.
	DefaultTickerInterval = 5

	// Database defaults.
	DefaultDBMaxConns          = 20
	DefaultDBMinConns          = 5
	DefaultDBMaxConnLifetime   = "30m"
	DefaultDBMaxConnIdleTime   = "5m"
	DefaultDBHealthCheckPeriod = "1m"
	DefaultDBSSLMode           = "disable"
	DefaultDBConnectTimeout    = 10 * time.Second
)

// initializeDefaultConfig creates a new Config with default values.
func initializeDefaultConfig() *Config {
	return &Config{
		Settings: Settings{
			// Default settings that work well for most deployments.
			ListenerPort:   DefaultListenerPort,
			AdminPort:      DefaultAdminPort,
			TickerEnabled:  true,
			TickerInterval: DefaultTickerInterval,
		},
		Database: DatabaseConfig{ //nolint: exhaustruct // This initializes defaults. We don't care about it being complete.
			// These will be overridden by the TOML file
			SSLMode:        DefaultDBSSLMode,
			ConnectTimeout: DefaultDBConnectTimeout,
		},
		WebhookServices: make(map[string]WebhookService),
	}
}

// applyDatabaseDefaults sets default values for database configuration if not specified.
func applyDatabaseDefaults(db *DatabaseConfig) {
	if db.MaxConns == 0 {
		db.MaxConns = DefaultDBMaxConns
	}
	if db.MinConns == 0 {
		db.MinConns = DefaultDBMinConns
	}
	if db.MaxConnLifetime == "" {
		db.MaxConnLifetime = DefaultDBMaxConnLifetime
	}
	if db.MaxConnIdleTime == "" {
		db.MaxConnIdleTime = DefaultDBMaxConnIdleTime
	}
	if db.HealthCheckPeriod == "" {
		db.HealthCheckPeriod = DefaultDBHealthCheckPeriod
	}
	if db.SSLMode == "" {
		db.SSLMode = DefaultDBSSLMode
	}
	if db.ConnectTimeout == 0 {
		db.ConnectTimeout = DefaultDBConnectTimeout
	}
}

// applyForwarderDefaults sets default values for a forwarder.
func applyForwarderDefaults(forwarder *Forwarder) {
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
}

// populateServiceAndForwarderNames sets the Name fields from map keys and applies defaults.
func populateServiceAndForwarderNames(services map[string]WebhookService) map[string]WebhookService {
	result := make(map[string]WebhookService)

	// Populate Name fields from map keys and set defaults for each service
	for serviceName, service := range services {
		service.Name = serviceName

		// Set default forwarder values if not specified
		for forwarderName, forwarder := range service.Forwarders {
			forwarder.Name = forwarderName
			forwarder.Hash = generateUniqueName(serviceName, forwarderName)

			applyForwarderDefaults(&forwarder)

			service.Forwarders[forwarderName] = forwarder
		}
		result[serviceName] = service
	}

	return result
}

// setupValidator creates and configures a validator with custom validations.
func setupValidator() (*validator.Validate, error) {
	validate := validator.New()

	// Register custom validation for path
	if err := validate.RegisterValidation("alphanum", validateAlphanumeric); err != nil {
		return nil, fmt.Errorf("failed to register alphanum validation: %w", err)
	}

	return validate, nil
}

func loadConfig(path string) (*Config, error) {
	// Initialize config with default values
	config := initializeDefaultConfig()

	// Read TOML file
	if _, err := toml.DecodeFile(path, config); err != nil { //nolint: musttag,lll // everything that is configurable is tagged, but there are generated fields that are not.
		return nil, fmt.Errorf("failed to decode application config toml: %w", err)
	}

	// Apply defaults to database configuration
	applyDatabaseDefaults(&config.Database)

	// Process webhook services and forwarders
	config.WebhookServices = populateServiceAndForwarderNames(config.WebhookServices)

	// Setup and run validation
	validate, err := setupValidator()
	if err != nil {
		return nil, err
	}

	// Validate the config
	if err = validate.Struct(config); err != nil {
		return nil, err
	}

	// Validate that paths are unique across all webhook services
	if err = validateUniquePaths(config); err != nil {
		return nil, err
	}

	return config, nil
}

// validateAlphanumeric is the custom validator for alphanumeric values.
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
		return nil, err
	}
	return config, nil
}

func generateUniqueName(serviceName string, forwarderName string) string {
	return fmt.Sprintf("%s-%s", serviceName, forwarderName)
}

// validateUniquePaths ensures that no two webhook services have the same path.
func validateUniquePaths(config *Config) error {
	pathMap := make(map[string]string) // map[path]serviceName

	// First check service names as paths
	for serviceName := range config.WebhookServices {
		pathMap[serviceName] = serviceName
	}

	// Then check explicit paths
	for serviceName, service := range config.WebhookServices {
		if service.Path != "" {
			if existingService, exists := pathMap[service.Path]; exists {
				return fmt.Errorf("duplicate webhook path '%s' found in services '%s' and '%s'",
					service.Path, existingService, serviceName)
			}
			pathMap[service.Path] = serviceName
		}
	}

	return nil
}

// GetDSN returns the database connection string.
func (c *DatabaseConfig) GetDSN() string {
	hostPort := net.JoinHostPort(c.Host, c.Port)
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
		c.Username, c.Password, hostPort, c.Database, c.SSLMode)
}
