-- Rollback transaction integrity improvements

DROP FUNCTION IF EXISTS release_advisory_lock(BIGINT);
DROP FUNCTION IF EXISTS acquire_advisory_lock(BIGINT);
DROP INDEX IF EXISTS idx_products_category;
DROP INDEX IF EXISTS idx_notifications_user_seen;
DROP INDEX IF EXISTS idx_invoices_status;
DROP INDEX IF EXISTS idx_orders_created_at;
DROP INDEX IF EXISTS idx_orders_vendor_status;
DROP INDEX IF EXISTS idx_orders_customer_status;
