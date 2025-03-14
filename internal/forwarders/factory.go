package forwarders

import (
	"errors"

	"laile/internal/config"
)

func NewForwarder(config *config.Forwarder) (DeliveryAttemptForwarder, error) {
	switch config.Type {
	case "amqp":
		connectionKey := config.Hash
		activeForwarder, ok := globalConnections.GetConnection(connectionKey)
		if ok {
			return activeForwarder, nil
		}
		activeForwarder = NewRMQForwarder(config)
		globalConnections.SetConnection(connectionKey, activeForwarder)
		return activeForwarder, nil
	case "http":
		return NewHTTPForwarder(config), nil
	default:
		return nil, errors.New("invalid forwarder type")
	}
}
