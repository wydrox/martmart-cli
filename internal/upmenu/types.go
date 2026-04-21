package upmenu

import "net/http"

type Config struct {
	BaseURL       string
	SiteID        string
	RestaurantID  string
	Language      string
	DeliveryType  string
	CartLocation  string
	PaymentMethod string
	UserAgent     string
	HTTPClient    *http.Client
}

type State struct {
	CartID string `json:"cart_id,omitempty"`
}

type RestaurantInfo struct {
	ID                   string   `json:"id,omitempty"`
	Name                 string   `json:"name,omitempty"`
	URL                  string   `json:"url,omitempty"`
	Street               string   `json:"street,omitempty"`
	PostalCode           string   `json:"postal_code,omitempty"`
	City                 string   `json:"city,omitempty"`
	Phone                string   `json:"phone,omitempty"`
	Email                string   `json:"email,omitempty"`
	Currency             string   `json:"currency,omitempty"`
	Delivery             bool     `json:"delivery"`
	Takeaway             bool     `json:"takeaway"`
	OnSite               bool     `json:"onsite"`
	OnlineOrdering       bool     `json:"online_ordering"`
	OpenNow              bool     `json:"open_now"`
	MinimumOrderPrice    *float64 `json:"minimum_order_price,omitempty"`
	MinimumDeliveryCost  *float64 `json:"minimum_delivery_cost,omitempty"`
	MaximumDeliveryCost  *float64 `json:"maximum_delivery_cost,omitempty"`
}

type Menu struct {
	Categories []MenuCategory `json:"categories"`
	Products   []MenuProduct  `json:"products"`
}

type MenuCategory struct {
	ID          string        `json:"id,omitempty"`
	Name        string        `json:"name,omitempty"`
	Description string        `json:"description,omitempty"`
	Products    []MenuProduct `json:"products,omitempty"`
}

type MenuProduct struct {
	ID               string    `json:"id,omitempty"`
	CategoryID       string    `json:"category_id,omitempty"`
	CategoryName     string    `json:"category_name,omitempty"`
	Name             string    `json:"name,omitempty"`
	Description      string    `json:"description,omitempty"`
	ImageURL         string    `json:"image_url,omitempty"`
	ProductPriceID   string    `json:"product_price_id,omitempty"`
	BasePrice        *float64  `json:"base_price,omitempty"`
	RequiresFlow     bool      `json:"requires_flow"`
	AvailableOptions []Variant `json:"available_options,omitempty"`
}

type Variant struct {
	ID    string   `json:"id,omitempty"`
	Name  string   `json:"name,omitempty"`
	Price *float64 `json:"price,omitempty"`
}

type Cart struct {
	ID            string     `json:"id,omitempty"`
	DeliveryType  string     `json:"delivery_type,omitempty"`
	DeliveryStatus string    `json:"delivery_status,omitempty"`
	TotalCost     *float64   `json:"total_cost,omitempty"`
	ProductsCost  *float64   `json:"products_cost,omitempty"`
	DeliveryCost  *float64   `json:"delivery_cost,omitempty"`
	ItemsSize     int        `json:"items_size,omitempty"`
	Items         []CartItem `json:"items,omitempty"`
	Messages      []string   `json:"messages,omitempty"`
	Errors        []string   `json:"errors,omitempty"`
}

type CartItem struct {
	ID             string   `json:"id,omitempty"`
	Name           string   `json:"name,omitempty"`
	ProductID      string   `json:"product_id,omitempty"`
	ProductPriceID string   `json:"product_price_id,omitempty"`
	Quantity       float64  `json:"quantity,omitempty"`
	Price          *float64 `json:"price,omitempty"`
}

type CartRequest struct {
	CartID         string `json:"cartId,omitempty"`
	CustomerID     any    `json:"customerId,omitempty"`
	DeliveryType   string `json:"deliveryType,omitempty"`
	CartLocation   string `json:"cartLocation,omitempty"`
	PaymentMethod  string `json:"paymentMethod,omitempty"`
}

type RequiredResult struct {
	Required bool `json:"required"`
}

type BuyingFlow struct {
	BuyingFlowID   string          `json:"buyingFlowId,omitempty"`
	RestaurantID   string          `json:"restaurantId,omitempty"`
	CartID         string          `json:"cartId,omitempty"`
	ProductPriceID string          `json:"productPriceId,omitempty"`
	ProductName    string          `json:"productName,omitempty"`
	ProductPrice   *float64        `json:"productPrice,omitempty"`
	Quantity       int             `json:"quantity,omitempty"`
	Steps          []BuyingFlowStep `json:"steps,omitempty"`
	Errors         []string        `json:"errors,omitempty"`
	TotalPrice     *float64        `json:"totalPrice,omitempty"`
	Raw            map[string]any  `json:"-"`
}

type BuyingFlowStep struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Done  bool   `json:"done"`
}
