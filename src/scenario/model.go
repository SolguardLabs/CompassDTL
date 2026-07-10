package scenario

import (
	"github.com/solguardlabs/compassdtl/src/api"
	"github.com/solguardlabs/compassdtl/src/domain"
)

type Definition struct {
	Name      string        `json:"name"`
	Bootstrap api.Bootstrap `json:"bootstrap"`
	Actions   []Action      `json:"actions"`
}

type Action struct {
	Type        string                           `json:"type"`
	Label       string                           `json:"label,omitempty"`
	Intent      domain.Intent                    `json:"intent,omitempty"`
	Count       uint64                           `json:"count,omitempty"`
	Delta       uint64                           `json:"delta,omitempty"`
	Exposure    domain.ExposureAdjustmentRequest `json:"exposure,omitempty"`
	RouteUpdate domain.RouteUpdateRequest        `json:"routeUpdate,omitempty"`
	ExpectError domain.Code                      `json:"expectError,omitempty"`
}

type Result struct {
	Name     string                `json:"name"`
	Results  []ActionResult        `json:"results"`
	Snapshot domain.SystemSnapshot `json:"snapshot"`
}

type ActionResult struct {
	Type     string                       `json:"type"`
	Label    string                       `json:"label,omitempty"`
	Submit   *domain.SubmitIntentResponse `json:"submit,omitempty"`
	Quotes   []domain.RouteQuote          `json:"quotes,omitempty"`
	Execute  *domain.ExecuteResponse      `json:"execute,omitempty"`
	Snapshot *domain.SystemSnapshot       `json:"snapshot,omitempty"`
	Route    *domain.Route                `json:"route,omitempty"`
	Epoch    *uint64                      `json:"epoch,omitempty"`
	Error    *domain.Error                `json:"error,omitempty"`
}
