package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/myrmidons/myrmidons/internal/config"
)

// Application holds all application dependencies
type Application struct {
	// Database
	Db *pgxpool.Pool

	// Configuration
	Config *config.Config

	// Router
	Router *chi.Mux
}

// New creates a new application instance
func New(ctx context.Context, cfg *config.Config) (*Application, error) {
	// Connect to database
	db, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Verify database connection
	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	app := &Application{
		Db:     db,
		Config: cfg,
	}

	// Setup router
	app.Router = app.setupRouter()

	return app, nil
}

// setupRouter configures the HTTP router with all routes and middleware
func (app *Application) setupRouter() *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(middleware.Timeout(30 * 1e9)) // 30 seconds

	// Rate limiting - 30 requests per second per IP
	r.Use(httprate.LimitByIP(30, 1))

	// Security headers middleware
	r.Use(app.securityHeaders)

	// Logging middleware
	r.Use(app.requestLogger)

	// Session middleware (placeholder - add SCS session manager)
	r.Use(app.sessionMiddleware)

	// Mount routes
	r.Route("/", func(r chi.Router) {
		// Public routes
		r.Get("/", app.HandleHome)
		r.Get("/browse", app.HandleBrowse)
		r.Get("/product/{slug}", app.HandleProduct)
		r.Get("/category/{slug}", app.HandleCategory)

		// Auth routes
		r.Group(func(r chi.Router) {
			r.Get("/login", app.HandleLoginPage)
			r.Post("/login", app.HandleLogin)
			r.Get("/register", app.HandleRegisterPage)
			r.Post("/register", app.HandleRegister)
			r.Post("/logout", app.HandleLogout)
		})

		// Wallet routes (authenticated)
		r.Group(func(r chi.Router) {
			r.Use(app.requireAuth)
			r.Get("/wallet", app.HandleWallet)
			r.Post("/wallet/deposit", app.HandleDeposit)
			r.Post("/wallet/withdraw", app.HandleWithdraw)
		})

		// Order routes (authenticated)
		r.Group(func(r chi.Router) {
			r.Use(app.requireAuth)
			r.Get("/orders", app.HandleOrders)
			r.Get("/order/{id}", app.HandleOrder)
			r.Post("/order/create", app.HandleCreateOrder)
			r.Post("/order/{id}/finalize", app.HandleFinalize)
			r.Post("/order/{id}/dispute", app.HandleDispute)
			r.Post("/order/{id}/message", app.HandleOrderMessage)
		})

		// Vendor routes (authenticated, vendor only)
		r.Group(func(r chi.Router) {
			r.Use(app.requireAuth)
			r.Use(app.requireVendor)
			r.Get("/vendor/dashboard", app.HandleVendorDashboard)
			r.Get("/vendor/products", app.HandleVendorProducts)
			r.Post("/vendor/products", app.HandleCreateProduct)
			r.Post("/vendor/products/{id}", app.HandleUpdateProduct)
			r.Get("/vendor/orders", app.HandleVendorOrders)
			r.Post("/vendor/order/{id}/accept", app.HandleAcceptOrder)
			r.Post("/vendor/order/{id}/decline", app.HandleDeclineOrder)
			r.Post("/vendor/order/{id}/dispatch", app.HandleDispatchOrder)
		})

		// Jury routes (authenticated, juror only)
		r.Group(func(r chi.Router) {
			r.Use(app.requireAuth)
			r.Use(app.requireJuror)
			r.Get("/jury", app.HandleJury)
			r.Post("/jury/vote", app.HandleJuryVote)
		})

		// DAO routes (authenticated)
		r.Group(func(r chi.Router) {
			r.Use(app.requireAuth)
			r.Get("/dao", app.HandleDAO)
			r.Post("/dao/proposals", app.HandleCreateProposal)
			r.Post("/dao/proposals/{id}/vote", app.HandleVote)
		})

		// Reserves
		r.Get("/reserves", app.HandleReserves)

		// User routes (authenticated)
		r.Group(func(r chi.Router) {
			r.Use(app.requireAuth)
			r.Get("/settings", app.HandleSettings)
			r.Post("/settings", app.HandleUpdateSettings)
			r.Get("/notifications", app.HandleNotifications)
			r.Post("/notifications/mark-read", app.HandleMarkNotificationsRead)
		})

		// API routes
		r.Route("/api", func(r chi.Router) {
			r.Get("/categories", app.APIGetCategories)
			r.Get("/products", app.APIGetProducts)
			r.Get("/products/{id}", app.APIGetProduct)
			r.Get("/orders/{id}/invoice", app.APIGetInvoice)
		})

		// Health check
		r.Get("/health", app.HandleHealth)
	})

	return r
}

// securityHeaders adds security-related HTTP headers
func (app *Application) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content Security Policy
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self';")

		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Referrer policy
		w.Header().Set("Referrer-Policy", "same-origin")

		// Permissions policy
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// HSTS (only on HTTPS)
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// requestLogger logs HTTP requests
func (app *Application) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This is a placeholder - implement proper logging
		next.ServeHTTP(w, r)
	})
}

// sessionMiddleware handles session management
func (app *Application) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Placeholder for session management
		// In production, integrate with SCS session manager
		next.ServeHTTP(w, r)
	})
}

// requireAuth middleware requires authentication
func (app *Application) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Placeholder - check session for user ID
		// In production, implement proper session validation
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

// requireVendor middleware requires vendor status
func (app *Application) requireVendor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Placeholder - check if user is vendor
		http.Error(w, "Vendor access required", http.StatusForbidden)
	})
}

// requireJuror middleware requires juror status
func (app *Application) requireJuror(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Placeholder - check if user is juror
		http.Error(w, "Juror access required", http.StatusForbidden)
	})
}

// Close closes all application resources
func (app *Application) Close() {
	app.Db.Close()
}
