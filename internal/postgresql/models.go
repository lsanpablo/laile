// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0

package dbmodels

import (
	"database/sql/driver"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type DeliveryStatus string

const (
	DeliveryStatusFuture     DeliveryStatus = "future"
	DeliveryStatusScheduled  DeliveryStatus = "scheduled"
	DeliveryStatusProcessing DeliveryStatus = "processing"
	DeliveryStatusSuccess    DeliveryStatus = "success"
	DeliveryStatusFailed     DeliveryStatus = "failed"
	DeliveryStatusNotNeeded  DeliveryStatus = "not_needed"
)

func (e *DeliveryStatus) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = DeliveryStatus(s)
	case string:
		*e = DeliveryStatus(s)
	default:
		return fmt.Errorf("unsupported scan type for DeliveryStatus: %T", src)
	}
	return nil
}

type NullDeliveryStatus struct {
	DeliveryStatus DeliveryStatus
	Valid          bool // Valid is true if DeliveryStatus is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullDeliveryStatus) Scan(value interface{}) error {
	if value == nil {
		ns.DeliveryStatus, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.DeliveryStatus.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullDeliveryStatus) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.DeliveryStatus), nil
}

type DeliveryAttempt struct {
	ID              int64
	TargetID        pgtype.Int8
	Status          DeliveryStatus
	ScheduledFor    pgtype.Timestamptz
	ExecutedAt      pgtype.Timestamptz
	ResponseCode    pgtype.Int4
	ResponseBody    pgtype.Text
	ResponseHeaders []byte
	ErrorMessage    pgtype.Text
	CreatedAt       pgtype.Timestamptz
	HashValue       int64
	WorkerName      pgtype.Text
}

type HashRing struct {
	ID        int32
	NodeName  string
	VirtualID int32
	HashKey   int64
}

type TaskLock struct {
	ID         int32
	TaskName   string
	WorkerName string
	AcquiredAt pgtype.Timestamptz
	TouchedAt  pgtype.Timestamptz
}

type Webhook struct {
	ID               int64
	Name             string
	Url              string
	Method           string
	Body             string
	Headers          []byte
	QueryParams      []byte
	WebhookServiceID string
	DeliveryStatus   DeliveryStatus
	CreatedAt        pgtype.Timestamptz
	IdempotencyKey   pgtype.Text
}

type WebhookTarget struct {
	ID          int64
	WebhookID   pgtype.Int8
	ForwarderID string
	CreatedAt   pgtype.Timestamptz
	HashValue   int64
}
