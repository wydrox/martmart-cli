package upmenu

const (
	DefaultBaseURL     = "https://dobrabula.orderwebsite.com"
	DefaultSiteID      = "a30e364b-df5c-11e7-93f9-525400841de1"
	DefaultRestaurantID = "b8a354b8-c958-11eb-a1e9-525400080521"
	DefaultLanguage    = "pl"
	DefaultDeliveryType = DeliveryTypeDelivery
	DefaultCartLocation = CartLocationMenu
	DefaultPaymentMethod = PaymentMethodOnline
)

const (
	HeaderAccept       = "Accept"
	HeaderContentType  = "Content-Type"
	HeaderUserAgent    = "User-Agent"
	HeaderXRequestedWith = "X-Requested-With"
)

const (
	ContentTypeJSON = "application/json"
	ContentTypeHTML = "text/html"
)

const (
	DeliveryTypeDelivery = "DELIVERY"
	DeliveryTypeTakeaway = "TAKEAWAY"
	DeliveryTypeOnSite   = "ONSITE"
)

const (
	CartLocationMenu      = "MENU"
	CartLocationOrderForm = "ORDER_FORM"
)

const (
	PaymentMethodOnline = "ONLINE"
	PaymentMethodCash   = "CASH"
	PaymentMethodCard   = "CART"
)
