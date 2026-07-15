-- Myrmidons Initial Database Schema
-- Phase 1: Foundation

-- ============================================
-- Users & Authentication
-- ============================================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(20) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    salt VARCHAR(32) NOT NULL,
    email VARCHAR(255),
    pgp_key TEXT,
    twofa_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    locale VARCHAR(10) NOT NULL DEFAULT 'en',
    theme VARCHAR(10) NOT NULL DEFAULT 'dark',
    is_vendor BOOLEAN NOT NULL DEFAULT FALSE,
    is_juror BOOLEAN NOT NULL DEFAULT FALSE,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_username ON users (username);
CREATE INDEX idx_users_email ON users (email) WHERE email IS NOT NULL;

-- ============================================
-- Sessions
-- ============================================

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions (user_id);
CREATE INDEX idx_sessions_expires ON sessions (expires_at);

-- ============================================
-- Login Attempts (Security)
-- ============================================

CREATE TABLE login_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ip_address INET NOT NULL,
    username VARCHAR(20),
    success BOOLEAN NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_login_attempts_ip ON login_attempts (ip_address, attempted_at);
CREATE INDEX idx_login_attempts_user ON login_attempts (user_id, attempted_at);

-- ============================================
-- Password History (Prevent Reuse)
-- ============================================

CREATE TABLE password_history (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, created_at)
);

CREATE INDEX idx_password_history_user ON password_history (user_id, created_at DESC);

-- ============================================
-- Wallets & Balances
-- ============================================

CREATE TABLE wallets (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    locked_balance_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================
-- Categories
-- ============================================

CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    icon VARCHAR(50),
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_categories_slug ON categories (slug);

-- ============================================
-- Products
-- ============================================

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id),
    title VARCHAR(200) NOT NULL,
    slug VARCHAR(200) NOT NULL,
    description TEXT,
    is_digital BOOLEAN NOT NULL DEFAULT FALSE,
    is_physical BOOLEAN NOT NULL DEFAULT TRUE,
    ships_from VARCHAR(100),
    ships_to VARCHAR(100)[] DEFAULT ARRAY[]::VARCHAR(100)[],
    rating DECIMAL(3,2) DEFAULT 0,
    review_count INT NOT NULL DEFAULT 0,
    is_legacy BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(vendor_id, slug)
);

CREATE INDEX idx_products_vendor ON products (vendor_id);
CREATE INDEX idx_products_category ON products (category_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_products_slug ON products (slug);

-- ============================================
-- Product Images
-- ============================================

CREATE TABLE product_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    filename VARCHAR(255) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_product_images_product ON product_images (product_id);

-- ============================================
-- Product Price Tiers
-- ============================================

CREATE TABLE price_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    quantity INT NOT NULL DEFAULT 1,
    price_pico NUMERIC(39, 0) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(product_id, quantity)
);

CREATE INDEX idx_price_tiers_product ON price_tiers (product_id);

-- ============================================
-- Delivery Methods
-- ============================================

CREATE TABLE delivery_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price_pico NUMERIC(39, 0) NOT NULL,
    estimated_days_min INT,
    estimated_days_max INT,
    is_digital BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_delivery_methods_vendor ON delivery_methods (vendor_id);

-- ============================================
-- Orders
-- ============================================

CREATE TYPE order_status AS ENUM (
    'pending', 'paid', 'accepted', 'declined', 
    'dispatched', 'delivered', 'disputed', 
    'finalized', 'refunded', 'cancelled'
);

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES users(id),
    vendor_id UUID NOT NULL REFERENCES users(id),
    status order_status NOT NULL DEFAULT 'pending',
    total_price_pico NUMERIC(39, 0) NOT NULL,
    shipping_price_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    fee_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    delivery_method_id UUID REFERENCES delivery_methods(id),
    shipping_address TEXT,
    details TEXT,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_customer ON orders (customer_id, created_at DESC);
CREATE INDEX idx_orders_vendor ON orders (vendor_id, created_at DESC);
CREATE INDEX idx_orders_status ON orders (status, created_at DESC);
CREATE INDEX idx_orders_created ON orders (created_at DESC);

-- ============================================
-- Order Items
-- ============================================

CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id),
    price_tier_id UUID NOT NULL REFERENCES price_tiers(id),
    quantity INT NOT NULL,
    price_pico NUMERIC(39, 0) NOT NULL,
    title VARCHAR(200) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_items_order ON order_items (order_id);

-- ============================================
-- Order Invoices
-- ============================================

CREATE TYPE invoice_status AS ENUM ('pending', 'confirmed', 'expired');

CREATE TABLE invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id),
    amount_pico NUMERIC(39, 0) NOT NULL,
    amount_unlocked_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    address VARCHAR(95),
    status invoice_status NOT NULL DEFAULT 'pending',
    permanent BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(order_id)
);

CREATE INDEX idx_invoices_status ON invoices (status, created_at);
CREATE INDEX idx_invoices_address ON invoices (address);

-- ============================================
-- Digital Items (Encrypted)
-- ============================================

CREATE TABLE digital_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_item_id UUID NOT NULL REFERENCES order_items(id) ON DELETE CASCADE,
    ciphertext BYTEA NOT NULL,
    nonce BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_digital_items_order_item ON digital_items (order_item_id);

-- ============================================
-- Order Chat
-- ============================================

CREATE TABLE order_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id),
    message TEXT NOT NULL,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_messages_order ON order_messages (order_id, created_at);

-- ============================================
-- Reviews
-- ============================================

CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    reviewer_id UUID NOT NULL REFERENCES users(id),
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(order_id)
);

CREATE INDEX idx_reviews_reviewer ON reviews (reviewer_id);
CREATE INDEX idx_reviews_created ON reviews (created_at DESC);

-- ============================================
-- FAQ
-- ============================================

CREATE TABLE faq_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE faq_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES faq_categories(id) ON DELETE CASCADE,
    question TEXT NOT NULL,
    answer TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_faq_items_category ON faq_items (category_id);

-- ============================================
-- Notifications
-- ============================================

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    message TEXT NOT NULL,
    link VARCHAR(500),
    is_seen BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user ON notifications (user_id, is_seen, created_at DESC);
