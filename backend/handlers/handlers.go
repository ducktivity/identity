// Package handlers holds the HTTP handlers for the identity service and the shared
// response helpers. Handlers are free functions that call into the auth, token, and
// store packages; main wires them onto the chi router.
package handlers

// stripeWebhookSecret verifies billing webhook signatures. Empty in dev; set by Init.
var stripeWebhookSecret string

// Init wires package-level configuration from main (currently just the Stripe
// webhook secret used by BillingWebhook). Call once at startup before serving.
func Init(stripeSecret string) {
	stripeWebhookSecret = stripeSecret
}
