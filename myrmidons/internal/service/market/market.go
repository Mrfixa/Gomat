package market

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service handles marketplace operations
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a new market service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// Product represents a marketplace product
type Product struct {
	ID          uuid.UUID
	VendorID    uuid.UUID
	CategoryID  uuid.UUID
	Title       string
	Slug        string
	Description string
	IsDigital   bool
	IsPhysical  bool
	ShipsFrom   string
	ShipsTo     []string
	Rating      decimal.Decimal
	ReviewCount int
	Images      []string
	Prices      []PriceTier
	CreatedAt   time.Time
}

// PriceTier represents a quantity-based price
type PriceTier struct {
	ID        uuid.UUID
	Quantity  int
	PricePico decimal.Decimal
}

// CreateProduct creates a new product
func (s *Service) CreateProduct(ctx context.Context, vendorID uuid.UUID, params CreateProductParams) (*Product, error) {
	// Generate slug from title
	slug := generateSlug(params.Title)

	// Check slug uniqueness
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM products WHERE vendor_id = $1 AND slug = $2)
	`, vendorID, slug).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		slug = fmt.Sprintf("%s-%s", slug, uuid.New().String()[:8])
	}

	productID := uuid.New()
	now := time.Now()

	_, err = s.db.Exec(ctx, `
		INSERT INTO products (
			id, vendor_id, category_id, title, slug, description,
			is_digital, is_physical, ships_from, ships_to,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)
	`, productID, vendorID, params.CategoryID, params.Title, slug, params.Description,
		params.IsDigital, params.IsPhysical, params.ShipsFrom, params.ShipsTo, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	// Create price tiers
	for _, tier := range params.PriceTiers {
		_, err = s.db.Exec(ctx, `
			INSERT INTO price_tiers (id, product_id, quantity, price_pico, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, uuid.New(), productID, tier.Quantity, tier.PricePico.BigInt(), now)
		if err != nil {
			return nil, fmt.Errorf("failed to create price tier: %w", err)
		}
	}

	return s.GetProduct(ctx, productID)
}

// CreateProductParams contains parameters for product creation
type CreateProductParams struct {
	CategoryID  uuid.UUID
	Title       string
	Description string
	IsDigital   bool
	IsPhysical  bool
	ShipsFrom   string
	ShipsTo     []string
	PriceTiers  []PriceTierInput
}

// PriceTierInput contains price tier input
type PriceTierInput struct {
	Quantity   int
	PricePico decimal.Decimal
}

// GetProduct retrieves a product by ID
func (s *Service) GetProduct(ctx context.Context, id uuid.UUID) (*Product, error) {
	var product Product
	var shipsTo []string

	err := s.db.QueryRow(ctx, `
		SELECT id, vendor_id, category_id, title, slug, description,
			is_digital, is_physical, ships_from, ships_to,
			rating, review_count, created_at
		FROM products WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&product.ID, &product.VendorID, &product.CategoryID,
		&product.Title, &product.Slug, &product.Description,
		&product.IsDigital, &product.IsPhysical, &product.ShipsFrom, &shipsTo,
		&product.Rating, &product.ReviewCount, &product.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}
	product.ShipsTo = shipsTo

	// Get images
	rows, _ := s.db.Query(ctx, `
		SELECT filename FROM product_images WHERE product_id = $1 ORDER BY sort_order
	`, id)
	defer rows.Close()

	for rows.Next() {
		var filename string
		rows.Scan(&filename)
		product.Images = append(product.Images, filename)
	}

	// Get price tiers
	tierRows, _ := s.db.Query(ctx, `
		SELECT id, quantity, price_pico FROM price_tiers WHERE product_id = $1 ORDER BY quantity
	`, id)
	defer tierRows.Close()

	for tierRows.Next() {
		var tier PriceTier
		tierRows.Scan(&tier.ID, &tier.Quantity, &tier.PricePico)
		product.Prices = append(product.Prices, tier)
	}

	return &product, nil
}

// GetProductBySlug retrieves a product by slug
func (s *Service) GetProductBySlug(ctx context.Context, slug string) (*Product, error) {
	var product Product
	var shipsTo []string

	err := s.db.QueryRow(ctx, `
		SELECT id, vendor_id, category_id, title, slug, description,
			is_digital, is_physical, ships_from, ships_to,
			rating, review_count, created_at
		FROM products WHERE slug = $1 AND deleted_at IS NULL
	`, slug).Scan(
		&product.ID, &product.VendorID, &product.CategoryID,
		&product.Title, &product.Slug, &product.Description,
		&product.IsDigital, &product.IsPhysical, &product.ShipsFrom, &shipsTo,
		&product.Rating, &product.ReviewCount, &product.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}
	product.ShipsTo = shipsTo

	// Get images, prices, etc.
	return s.GetProduct(ctx, product.ID)
}

// ListProducts returns products with filters
func (s *Service) ListProducts(ctx context.Context, params ListProductsParams) ([]*Product, int, error) {
	// Build query
	query := `
		SELECT p.id, p.vendor_id, p.category_id, p.title, p.slug, p.description,
			p.is_digital, p.is_physical, p.ships_from, p.ships_to,
			p.rating, p.review_count, p.created_at
		FROM products p
		WHERE p.deleted_at IS NULL
	`
	countQuery := `SELECT COUNT(*) FROM products WHERE deleted_at IS NULL`

	var args []interface{}
	argIndex := 1

	if params.CategoryID != uuid.Nil {
		query += fmt.Sprintf(" AND p.category_id = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND category_id = $%d", argIndex)
		args = append(args, params.CategoryID)
		argIndex++
	}

	if params.Search != "" {
		search := "%" + params.Search + "%"
		query += fmt.Sprintf(" AND (p.title ILIKE $%d OR p.description ILIKE $%d)", argIndex, argIndex)
		countQuery += fmt.Sprintf(" AND (title ILIKE $%d OR description ILIKE $%d)", argIndex, argIndex)
		args = append(args, search)
		argIndex++
	}

	if params.IsDigital {
		query += " AND p.is_digital = true"
		countQuery += " AND is_digital = true"
	}

	if params.VendorID != uuid.Nil {
		query += fmt.Sprintf(" AND p.vendor_id = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND vendor_id = $%d", argIndex)
		args = append(args, params.VendorID)
		argIndex++
	}

	// Count total
	var total int
	err := s.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Apply sorting
	switch params.Sort {
	case "price_asc":
		query += " ORDER BY (SELECT MIN(price_pico) FROM price_tiers WHERE product_id = p.id) ASC"
	case "price_desc":
		query += " ORDER BY (SELECT MAX(price_pico) FROM price_tiers WHERE product_id = p.id) DESC"
	case "rating":
		query += " ORDER BY p.rating DESC"
	default:
		query += " ORDER BY p.created_at DESC"
	}

	// Pagination
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, params.Limit, params.Offset)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		var product Product
		var shipsTo []string
		err := rows.Scan(
			&product.ID, &product.VendorID, &product.CategoryID,
			&product.Title, &product.Slug, &product.Description,
			&product.IsDigital, &product.IsPhysical, &product.ShipsFrom, &shipsTo,
			&product.Rating, &product.ReviewCount, &product.CreatedAt,
		)
		if err != nil {
			continue
		}
		product.ShipsTo = shipsTo
		products = append(products, &product)
	}

	return products, total, nil
}

// ListProductsParams contains filtering parameters
type ListProductsParams struct {
	CategoryID uuid.UUID
	Search     string
	IsDigital  bool
	VendorID   uuid.UUID
	Sort       string // price_asc, price_desc, rating, newest
	Limit      int
	Offset     int
}

// Category represents a product category
type Category struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	Description string
	Icon        string
	SortOrder   int
}

// GetCategories returns all categories
func (s *Service) GetCategories(ctx context.Context) ([]*Category, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, name, slug, description, icon, sort_order
		FROM categories ORDER BY sort_order
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*Category
	for rows.Next() {
		var cat Category
		rows.Scan(&cat.ID, &cat.Name, &cat.Slug, &cat.Description, &cat.Icon, &cat.SortOrder)
		categories = append(categories, &cat)
	}

	return categories, nil
}

// generateSlug creates a URL-friendly slug
func generateSlug(title string) string {
	// Convert to lowercase
	slug := strings.ToLower(title)
	
	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	
	// Remove non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9\-]`)
	slug = reg.ReplaceAllString(slug, "")
	
	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")
	
	// Trim hyphens from ends
	slug = strings.Trim(slug, "-")
	
	return slug
}

// Vendor represents a vendor profile
type Vendor struct {
	UserID       uuid.UUID
	Username     string
	Rating       decimal.Decimal
	TotalSales   int
	MemberSince  time.Time
	IsLegacy     bool
}

// GetVendor returns vendor profile
func (s *Service) GetVendor(ctx context.Context, userID uuid.UUID) (*Vendor, error) {
	var vendor Vendor
	err := s.db.QueryRow(ctx, `
		SELECT u.id, u.username, COALESCE(AVG(p.rating), 0), COUNT(DISTINCT o.id), u.created_at
		FROM users u
		LEFT JOIN products p ON p.vendor_id = u.id
		LEFT JOIN orders o ON o.vendor_id = u.id AND o.status = 'finalized'
		WHERE u.id = $1 AND u.is_vendor = true
		GROUP BY u.id
	`, userID).Scan(&vendor.UserID, &vendor.Username, &vendor.Rating, &vendor.TotalSales, &vendor.MemberSince)
	if err != nil {
		return nil, fmt.Errorf("vendor not found: %w", err)
	}

	// Check if legacy
	var legacyCount int
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE vendor_id = $1 AND is_legacy = true`, userID).Scan(&legacyCount)
	vendor.IsLegacy = legacyCount > 0

	return &vendor, nil
}

// AddReview adds a review to an order
func (s *Service) AddReview(ctx context.Context, orderID, reviewerID uuid.UUID, rating int, comment string) error {
	// Verify order is finalized
	var orderStatus string
	err := s.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&orderStatus)
	if err != nil {
		return err
	}
	if orderStatus != "finalized" {
		return fmt.Errorf("can only review finalized orders")
	}

	// Get vendor ID
	var vendorID uuid.UUID
	s.db.QueryRow(ctx, `SELECT vendor_id FROM orders WHERE id = $1`, orderID).Scan(&vendorID)

	reviewID := uuid.New()
	_, err = s.db.Exec(ctx, `
		INSERT INTO reviews (id, order_id, reviewer_id, rating, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (order_id) DO UPDATE SET
			rating = $4, comment = $5
	`, reviewID, orderID, reviewerID, rating, comment, time.Now())
	if err != nil {
		return err
	}

	// Update vendor rating
	_, err = s.db.Exec(ctx, `
		UPDATE users
		SET rating = (SELECT AVG(rating) FROM reviews WHERE reviewer_id IN (
			SELECT customer_id FROM orders WHERE vendor_id = $1
		))
		WHERE id = $1
	`, vendorID)

	return nil
}
