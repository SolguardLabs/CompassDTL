package domain

import (
	"fmt"
	"regexp"
	"strings"
)

type AssetID string
type AccountID string
type RouteID string
type CorridorID string
type IntentID string
type TicketID string
type ReceiptID string
type EventID string

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{1,63}$`)

func CleanID(value string) string {
	return strings.TrimSpace(value)
}

func ValidateID(kind string, value string) error {
	value = CleanID(value)
	if value == "" {
		return fmt.Errorf("%s is required", kind)
	}
	if !idPattern.MatchString(value) {
		return fmt.Errorf("%s %q is not a valid identifier", kind, value)
	}
	return nil
}

func (id AssetID) String() string {
	return string(id)
}

func (id AccountID) String() string {
	return string(id)
}

func (id RouteID) String() string {
	return string(id)
}

func (id CorridorID) String() string {
	return string(id)
}

func (id IntentID) String() string {
	return string(id)
}

func (id TicketID) String() string {
	return string(id)
}

func (id ReceiptID) String() string {
	return string(id)
}

func (id EventID) String() string {
	return string(id)
}

func (id AssetID) Validate() error {
	return ValidateID("asset", string(id))
}

func (id AccountID) Validate() error {
	return ValidateID("account", string(id))
}

func (id RouteID) Validate() error {
	return ValidateID("route", string(id))
}

func (id CorridorID) Validate() error {
	return ValidateID("corridor", string(id))
}

func (id IntentID) Validate() error {
	return ValidateID("intent", string(id))
}

func (id TicketID) Validate() error {
	return ValidateID("ticket", string(id))
}

func (id ReceiptID) Validate() error {
	return ValidateID("receipt", string(id))
}

func NewTicketID(intent IntentID, seq uint64) TicketID {
	return TicketID(fmt.Sprintf("ticket:%s:%06d", intent, seq))
}

func NewReceiptID(ticket TicketID, seq uint64) ReceiptID {
	return ReceiptID(fmt.Sprintf("receipt:%s:%06d", ticket, seq))
}

func NewEventID(epoch uint64, seq uint64) EventID {
	return EventID(fmt.Sprintf("evt:%06d:%06d", epoch, seq))
}
