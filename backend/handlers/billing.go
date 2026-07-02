package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ducktivity/identity/backend/api"
	"github.com/ducktivity/identity/backend/auth"
	"github.com/ducktivity/identity/backend/store"

	"github.com/ducktivity/platform-go/authtoken"
)

// BillingWebhook receives Stripe subscription events and resolves them to the single suite-wide entitlement. STUB: signature verification and event parsing are TODOs — see docs/suite-architecture §5. The shape is real so the "one payment unlocks all apps" path is exercised end-to-end via DevGrant.
func BillingWebhook(w http.ResponseWriter, _ *http.Request) {
	// TODO(billing): verify the Stripe-Signature header against stripeWebhookSecret, parse the event (checkout.session.completed, customer.subscription.updated/deleted), map the Stripe customer to a user, and call store.UpsertEntitlement with plan=pro/free, status=<stripe status>, until=current_period_end.
	_ = stripeWebhookSecret
	w.WriteHeader(http.StatusOK)
}

// DevGrant flips a user's suite-wide entitlement without Stripe, so the end-to-end "pro unlocks every app" flow can be tested before billing is wired. Registered only in development.
func DevGrant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in api.DevGrantInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	addr, ok := auth.NormalizeEmail(in.Email)
	if !ok {
		writeErr(w, http.StatusBadRequest, "A valid email is required")
		return
	}
	u, err := store.GetUserByEmail(ctx, addr)
	if err != nil {
		writeErr(w, http.StatusNotFound, "No such account")
		return
	}
	plan := authtoken.PlanFree
	var until *time.Time
	if in.Plan == authtoken.PlanPro {
		plan = authtoken.PlanPro
		if in.Days > 0 {
			t := time.Now().Add(time.Duration(in.Days) * 24 * time.Hour)
			until = &t
		}
	}
	if err := store.UpsertEntitlement(ctx, u.ID, plan, "dev_grant", until); err != nil {
		serverError(w, r, err, "Could not set entitlement")
		return
	}
	writeJSON(w, http.StatusOK, api.GrantResponse{UserID: u.ID, Plan: plan})
}
