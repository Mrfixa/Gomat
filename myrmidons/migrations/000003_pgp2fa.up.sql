-- Myrmidons Phase 3: Security - PGP 2FA, CAPTCHA, Session Management
-- ============================================

-- ============================================
-- PGP 2FA Sessions
-- ============================================

CREATE TABLE twofa_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    encrypted_token TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_twofa_sessions_user ON twofa_sessions (user_id, expires_at);

-- ============================================
-- CAPTCHA System
-- ============================================

CREATE TABLE captcha_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    onion_address VARCHAR(100) NOT NULL,
    solution VARCHAR(10) NOT NULL,
    hidden_indices INT[] NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_captcha_challenges_expires ON captcha_challenges (expires_at);

-- Jail records (IPs that failed challenges)
CREATE TABLE jail_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_address INET NOT NULL,
    reason VARCHAR(100),
    expires_at TIMESTAMPTZ NOT NULL,
    released_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_jail_records_ip ON jail_records (ip_address, expires_at);

-- ============================================
-- Session Management Improvements
-- ============================================

-- Add additional session metadata
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS locale VARCHAR(10);
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS theme VARCHAR(10);

-- Create session revocation table
CREATE TABLE session_revocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reason VARCHAR(100)
);

CREATE INDEX idx_session_revocations_user ON session_revocations (user_id, revoked_at);

-- ============================================
-- Vendor Licensing
-- ============================================

CREATE TABLE vendor_applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    application_text TEXT,
    admin_notes TEXT,
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vendor_applications_user ON vendor_applications (user_id);
CREATE INDEX idx_vendor_applications_status ON vendor_applications (status);

-- Vendor licenses
CREATE TABLE vendor_licenses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    bond_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    license_type VARCHAR(20) NOT NULL DEFAULT 'standard',
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    revocation_reason TEXT
);

CREATE INDEX idx_vendor_licenses_user ON vendor_licenses (user_id);

-- ============================================
-- Order Status Indexes (for performance)
-- ============================================

CREATE INDEX IF NOT EXISTS idx_orders_status_created ON orders (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_customer_status ON orders (customer_id, status);
CREATE INDEX IF NOT EXISTS idx_orders_vendor_status ON orders (vendor_id, status);

-- ============================================
-- Invoice Indexes
-- ============================================

CREATE INDEX IF NOT EXISTS idx_invoices_order ON invoices (order_id);
CREATE INDEX IF NOT EXISTS idx_invoices_status_expires ON invoices (status, created_at) WHERE status = 'pending';

-- ============================================
-- Notification Settings
-- ============================================

CREATE TABLE notification_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    email_orders BOOLEAN NOT NULL DEFAULT TRUE,
    email_disputes BOOLEAN NOT NULL DEFAULT TRUE,
    email_jury BOOLEAN NOT NULL DEFAULT TRUE,
    email_dao BOOLEAN NOT NULL DEFAULT TRUE,
    email_marketing BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================
-- Monero Wallet Integration
-- ============================================

-- Store wallet addresses for users
CREATE TABLE wallet_addresses (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    primary_address VARCHAR(95) NOT NULL,
    view_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_wallet_addresses_address ON wallet_addresses (primary_address);

-- Pending deposits (for tracking)
CREATE TABLE pending_deposits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_pico NUMERIC(39, 0) NOT NULL,
    tx_hash VARCHAR(64),
    confirmed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    confirmed_at TIMESTAMPTZ
);

CREATE INDEX idx_pending_deposits_user ON pending_deposits (user_id);
CREATE INDEX idx_pending_deposits_txhash ON pending_deposits (tx_hash);

-- Pending withdrawals
CREATE TABLE pending_withdrawals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_pico NUMERIC(39, 0) NOT NULL,
    fee_pico NUMERIC(39, 0) NOT NULL,
    destination_address VARCHAR(95) NOT NULL,
    tx_hash VARCHAR(64),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX idx_pending_withdrawals_user ON pending_withdrawals (user_id);
CREATE INDEX idx_pending_withdrawals_status ON pending_withdrawals (status);
