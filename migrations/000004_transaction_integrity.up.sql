-- SECURE: Database transaction integrity improvements

-- Add indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_orders_customer_status ON orders (customer_id, status);
CREATE INDEX IF NOT EXISTS idx_orders_vendor_status ON orders (vendor_id, status);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders (created_at DESC);

-- Add index for invoice status queries
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices (status, created_at);

-- Add index for notifications
CREATE INDEX IF NOT EXISTS idx_notifications_user_seen ON notifications (user_id, is_seen);

-- Add index for product category lookups
CREATE INDEX IF NOT EXISTS idx_products_category ON products (category_id) WHERE deleted_at IS NULL;

-- Create function for advisory locks
CREATE OR REPLACE FUNCTION acquire_advisory_lock(lock_id BIGINT)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN pg_try_advisory_lock(lock_id);
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION release_advisory_lock(lock_id BIGINT)
RETURNS VOID AS $$
BEGIN
    PERFORM pg_advisory_unlock(lock_id);
END;
$$ LANGUAGE plpgsql;

-- Add order version column for optimistic locking (can be used in future)
-- ALTER TABLE orders ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1;
