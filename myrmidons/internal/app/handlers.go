package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// TemplateContext holds data passed to templates
type TemplateContext struct {
	Locale string
	Theme  string
	User   *User // nil if not authenticated
}

// User represents the authenticated user
type User struct {
	ID       string
	Username string
	IsVendor bool
	IsJuror  bool
	IsAdmin  bool
}

// HandleHome renders the home page
func (app *Application) HandleHome(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "home", tc, nil)
}

// HandleBrowse renders the product browsing page
func (app *Application) HandleBrowse(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "browse", tc, nil)
}

// HandleProduct renders a product detail page
func (app *Application) HandleProduct(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	tc := app.newTemplateContext(r)
	data := map[string]interface{}{"Slug": slug}
	app.renderTemplate(w, r, "product", tc, data)
}

// HandleCategory renders products in a category
func (app *Application) HandleCategory(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	tc := app.newTemplateContext(r)
	data := map[string]interface{}{"Category": slug}
	app.renderTemplate(w, r, "category", tc, data)
}

// HandleLoginPage renders the login page
func (app *Application) HandleLoginPage(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "login", tc, nil)
}

// HandleLogin processes login form submission
func (app *Application) HandleLogin(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	if username == "" || password == "" {
		http.Error(w, "Username and password required", http.StatusBadRequest)
		return
	}

	// Placeholder - implement authentication logic
	// In production, use bcrypt and check against database
	_ = username
	_ = password

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleRegisterPage renders the registration page
func (app *Application) HandleRegisterPage(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "register", tc, nil)
}

// HandleRegister processes registration form
func (app *Application) HandleRegister(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")
	passwordConfirm := r.PostForm.Get("password_confirm")

	if username == "" || password == "" {
		http.Error(w, "All fields required", http.StatusBadRequest)
		return
	}

	if password != passwordConfirm {
		http.Error(w, "Passwords do not match", http.StatusBadRequest)
		return
	}

	// Placeholder - implement registration logic
	_ = username
	_ = password

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// HandleLogout processes logout
func (app *Application) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Placeholder - implement logout logic
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleWallet renders the wallet page
func (app *Application) HandleWallet(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "wallet", tc, nil)
}

// HandleDeposit processes deposit
func (app *Application) HandleDeposit(w http.ResponseWriter, r *http.Request) {
	// Placeholder - implement deposit logic
	http.Redirect(w, r, "/wallet", http.StatusSeeOther)
}

// HandleWithdraw processes withdrawal
func (app *Application) HandleWithdraw(w http.ResponseWriter, r *http.Request) {
	// Placeholder - implement withdrawal logic
	http.Redirect(w, r, "/wallet", http.StatusSeeOther)
}

// HandleOrders renders the orders list
func (app *Application) HandleOrders(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "orders", tc, nil)
}

// HandleOrder renders order detail
func (app *Application) HandleOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tc := app.newTemplateContext(r)
	data := map[string]interface{}{"OrderID": id}
	app.renderTemplate(w, r, "order", tc, data)
}

// HandleCreateOrder creates a new order
func (app *Application) HandleCreateOrder(w http.ResponseWriter, r *http.Request) {
	// Placeholder - implement order creation
	http.Redirect(w, r, "/orders", http.StatusSeeOther)
}

// HandleFinalize finalizes an order
func (app *Application) HandleFinalize(w http.ResponseWriter, r *http.Request) {
	// Placeholder - implement finalization
	http.Redirect(w, r, "/orders", http.StatusSeeOther)
}

// HandleDispute opens a dispute
func (app *Application) HandleDispute(w http.ResponseWriter, r *http.Request) {
	// Placeholder - implement dispute opening
	http.Redirect(w, r, "/orders", http.StatusSeeOther)
}

// HandleOrderMessage sends a message in an order
func (app *Application) HandleOrderMessage(w http.ResponseWriter, r *http.Request) {
	// Placeholder - implement messaging
	http.Redirect(w, r, "/orders", http.StatusSeeOther)
}

// HandleVendorDashboard renders vendor dashboard
func (app *Application) HandleVendorDashboard(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "vendor_dashboard", tc, nil)
}

// HandleVendorProducts renders vendor products
func (app *Application) HandleVendorProducts(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "vendor_products", tc, nil)
}

// HandleCreateProduct creates a product
func (app *Application) HandleCreateProduct(w http.ResponseWriter, r *http.Request) {
	// Placeholder
	http.Redirect(w, r, "/vendor/products", http.StatusSeeOther)
}

// HandleUpdateProduct updates a product
func (app *Application) HandleUpdateProduct(w http.ResponseWriter, r *http.Request) {
	// Placeholder
	http.Redirect(w, r, "/vendor/products", http.StatusSeeOther)
}

// HandleVendorOrders renders vendor orders
func (app *Application) HandleVendorOrders(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "vendor_orders", tc, nil)
}

// HandleAcceptOrder accepts an order
func (app *Application) HandleAcceptOrder(w http.ResponseWriter, r *http.Request) {
	// Placeholder
	http.Redirect(w, r, "/vendor/orders", http.StatusSeeOther)
}

// HandleDeclineOrder declines an order
func (app *Application) HandleDeclineOrder(w http.ResponseWriter, r *http.Request) {
	// Placeholder
	http.Redirect(w, r, "/vendor/orders", http.StatusSeeOther)
}

// HandleDispatchOrder dispatches an order
func (app *Application) HandleDispatchOrder(w http.ResponseWriter, r *http.Request) {
	// Placeholder
	http.Redirect(w, r, "/vendor/orders", http.StatusSeeOther)
}

// HandleJury renders jury dashboard
func (app *Application) HandleJury(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "jury", tc, nil)
}

// HandleJuryVote records a jury vote
func (app *Application) HandleJuryVote(w http.ResponseWriter, r *http.Request) {
	// Placeholder
	http.Redirect(w, r, "/jury", http.StatusSeeOther)
}

// HandleDAO renders DAO page
func (app *Application) HandleDAO(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "dao", tc, nil)
}

// HandleCreateProposal creates a DAO proposal
func (app *Application) HandleCreateProposal(w http.ResponseWriter, r *http.Request) {
	// Placeholder
	http.Redirect(w, r, "/dao", http.StatusSeeOther)
}

// HandleVote votes on a proposal
func (app *Application) HandleVote(w http.ResponseWriter, r *http.Request) {
	// Placeholder
	http.Redirect(w, r, "/dao", http.StatusSeeOther)
}

// HandleReserves renders reserves page
func (app *Application) HandleReserves(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "reserves", tc, nil)
}

// HandleSettings renders settings page
func (app *Application) HandleSettings(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "settings", tc, nil)
}

// HandleUpdateSettings updates user settings
func (app *Application) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	// Placeholder
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// HandleNotifications renders notifications
func (app *Application) HandleNotifications(w http.ResponseWriter, r *http.Request) {
	tc := app.newTemplateContext(r)
	app.renderTemplate(w, r, "notifications", tc, nil)
}

// HandleMarkNotificationsRead marks notifications as read
func (app *Application) HandleMarkNotificationsRead(w http.ResponseWriter, r *http.Request) {
	// Placeholder
	http.Redirect(w, r, "/notifications", http.StatusSeeOther)
}

// HandleHealth returns health status
func (app *Application) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response := map[string]interface{}{
		"status":   "ok",
		"database": true,
	}

	if err := app.Db.Ping(ctx); err != nil {
		response["status"] = "error"
		response["database"] = false
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// APIGetCategories returns categories JSON
func (app *Application) APIGetCategories(w http.ResponseWriter, r *http.Request) {
	categories := []map[string]interface{}{
		{"id": "1", "name": "Digital Goods", "slug": "digital"},
		{"id": "2", "name": "Physical Goods", "slug": "physical"},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}

// APIGetProducts returns products JSON
func (app *Application) APIGetProducts(w http.ResponseWriter, r *http.Request) {
	products := []map[string]interface{}{}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

// APIGetProduct returns a product JSON
func (app *Application) APIGetProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	product := map[string]interface{}{"id": id}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

// APIGetInvoice returns invoice data
func (app *Application) APIGetInvoice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	invoice := map[string]interface{}{"id": id}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoice)
}

// newTemplateContext creates a template context
func (app *Application) newTemplateContext(r *http.Request) *TemplateContext {
	return &TemplateContext{
		Locale: "en",
		Theme:  "dark",
		User:   nil,
	}
}

// renderTemplate renders a template (placeholder)
func (app *Application) renderTemplate(w http.ResponseWriter, r *http.Request, name string, tc *TemplateContext, data map[string]interface{}) {
	// Placeholder - in production, use Templ to render templates
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<html><body><h1>%s</h1><p>Template: %s</p></body></html>", app.Config.SiteName, name)
}
