// Package domain defines pure business entities, request/response models,
// and Clean Architecture ports for the Abeta SDUI Domain BFF.
// It must never import Gin, net/http, or any infrastructure/handler package.
package domain

import (
	"errors"
	"time"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
)

// Sentinel domain errors mapped to structured API responses by the handler layer.
var (
	ErrScreenNotFound = errors.New("screen not found")
	ErrInvalidInput   = errors.New("invalid input")
	ErrNotFound       = errors.New("resource not found")
)

// AppDescriptor describes a single vertical application exposed in the 6-in-1 Super-App catalog.
type AppDescriptor struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Tagline     string   `json:"tagline"`
	Category    string   `json:"category"`
	TenantID    string   `json:"tenant_id"`
	EntryScreen string   `json:"entry_screen"`
	ThemePreset string   `json:"theme_preset"`
	Screens     []string `json:"screens"`
}

// ScreenSummary describes a discoverable screen route in GET /api/v1/screens.
type ScreenSummary struct {
	ScreenID    string `json:"screen_id"`
	Title       string `json:"title"`
	Vertical    string `json:"vertical"`
	Description string `json:"description"`
}

// ScreenRequest captures targeting headers and query parameters for rendering an SDUI screen.
type ScreenRequest struct {
	ScreenID    string
	Query       string
	Filter      string
	City        string
	Platform    string
	UserSegment string
	WindowClass string
	TenantID    string
	Cursor      string
	Extra       map[string]string
}

// ActionExecuteRequest mirrors the KMP client payload sent to POST /api/v1/actions/execute.
type ActionExecuteRequest struct {
	ActionID   string         `json:"action_id"`
	ActionType string         `json:"action_type"`
	Payload    map[string]any `json:"payload,omitempty"`
}

// ActionResult is returned by POST /api/v1/actions/execute and domain mutation endpoints,
// carrying SDUI follow_up_actions that RemoteApiCallExecutor dispatches automatically.
type ActionResult struct {
	Status          string                  `json:"status"`
	Message         string                  `json:"message,omitempty"`
	FollowUpActions []actions.ActionTrigger `json:"follow_up_actions,omitempty"`
	Data            map[string]any          `json:"data,omitempty"`
}

// ExpressOrderRequest represents an instant checkout submission across commerce verticals.
type ExpressOrderRequest struct {
	Vertical  string         `json:"vertical,omitempty"`
	Items     map[string]int `json:"items,omitempty"`
	Address   string         `json:"address,omitempty"`
	Coupon    string         `json:"coupon,omitempty"`
	Payment   string         `json:"payment,omitempty"`
	Notes     string         `json:"notes,omitempty"`
	ItemCount int            `json:"item_count,omitempty"`
	TotalUSD  float64        `json:"total_usd,omitempty"`
}

// BookingRequest represents an event/movie/dining ticket reservation for Abeta Pulse.
type BookingRequest struct {
	EventID   string `json:"event_id"`
	EventName string `json:"event_name,omitempty"`
	SeatTier  string `json:"seat_tier"`
	Tier      string `json:"tier,omitempty"`
	Quantity  int    `json:"quantity"`
	City      string `json:"city,omitempty"`
}

// SwipeRequest represents a profile swipe decision for Abeta Spark.
type SwipeRequest struct {
	ProfileID string `json:"profile_id"`
	Direction string `json:"direction"`        // like | nope | super_like | undo | reset
	Action    string `json:"action,omitempty"` // alias for direction
}

// SparkMatch represents a persisted mutual match in Abeta Spark.
type SparkMatch struct {
	MatchID    string    `json:"match_id"`
	ProfileID  string    `json:"profile_id"`
	Name       string    `json:"name"`
	Role       string    `json:"role"`
	City       string    `json:"city"`
	SuperLiked bool      `json:"super_liked"`
	MatchedAt  time.Time `json:"matched_at"`
}

// RFQRequest represents a B2B Request For Quotation submitted via Abeta Trade.
type RFQRequest struct {
	SupplierID   string `json:"supplier_id"`
	ProductName  string `json:"product_name"`
	Quantity     int    `json:"quantity"`
	CompanyName  string `json:"company_name"`
	Company      string `json:"company,omitempty"` // alias for company_name
	ContactEmail string `json:"contact_email"`
	Email        string `json:"email,omitempty"` // alias for contact_email
	DeliveryCity string `json:"delivery_city,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

// RFQRecord represents a persisted B2B quotation tracking entry in Abeta Trade.
type RFQRecord struct {
	QuoteID      string    `json:"quote_id"`
	SupplierID   string    `json:"supplier_id"`
	ProductName  string    `json:"product_name"`
	Quantity     int       `json:"quantity"`
	CompanyName  string    `json:"company_name"`
	ContactEmail string    `json:"contact_email"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
