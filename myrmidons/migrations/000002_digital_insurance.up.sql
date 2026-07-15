-- Myrmidons Phase 2: Digital Goods, Insurance Fund, Jury System
-- ============================================

-- Insurance Fund
CREATE TABLE insurance_fund (
    id SERIAL PRIMARY KEY CHECK(id = 1),
    balance_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    total_contributions_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    total_claims_paid_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Initialize insurance fund
INSERT INTO insurance_fund DEFAULT VALUES;

-- ============================================
-- Jury System
-- ============================================

CREATE TABLE jurors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    bond_pico NUMERIC(39, 0) NOT NULL,
    locked BOOLEAN NOT NULL DEFAULT FALSE,
    locked_until TIMESTAMPTZ,
    total_cases INT NOT NULL DEFAULT 0,
    honest_votes INT NOT NULL DEFAULT 0,
    dishonest_votes INT NOT NULL DEFAULT 0,
    pending_cases INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_jurors_user ON jurors (user_id);
CREATE INDEX idx_jurors_locked ON jurors (locked, locked_until);

-- Jury settings
CREATE TABLE jury_settings (
    id SERIAL PRIMARY KEY CHECK(id = 1),
    min_bond_pico NUMERIC(39, 0) NOT NULL DEFAULT 100000000000,
    jury_size INT NOT NULL DEFAULT 3,
    vote_window_hours INT NOT NULL DEFAULT 72,
    require_unanimity BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO jury_settings (min_bond_pico) VALUES (100000000000);

-- Disputes
CREATE TYPE dispute_status AS ENUM ('open', 'voting', 'resolved');

CREATE TABLE disputes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    status dispute_status NOT NULL DEFAULT 'open',
    buyer_statement TEXT,
    vendor_statement TEXT,
    buyer_evidence JSONB DEFAULT '[]',
    vendor_evidence JSONB DEFAULT '[]',
    refund_amount_pico NUMERIC(39, 0),
    ruling VARCHAR(20),
    ruling_reason TEXT,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_disputes_order ON disputes (order_id);
CREATE INDEX idx_disputes_status ON disputes (status, created_at);

-- Jury Assignments
CREATE TYPE jury_vote AS ENUM ('buyer', 'vendor', 'abstain');

CREATE TABLE jury_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dispute_id UUID NOT NULL REFERENCES disputes(id) ON DELETE CASCADE,
    juror_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    seed VARCHAR(64) NOT NULL,
    vote VARCHAR(20),
    vote_signature TEXT,
    voted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(dispute_id, juror_id)
);

CREATE INDEX idx_jury_assignments_dispute ON jury_assignments (dispute_id);
CREATE INDEX idx_jury_assignments_juror ON jury_assignments (juror_id);

-- ============================================
-- Provable Reserves
-- ============================================

CREATE TABLE reserve_proofs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    view_key TEXT NOT NULL,
    total_balance_pico NUMERIC(39, 0) NOT NULL,
    user_balances_pico NUMERIC(39, 0) NOT NULL,
    insurance_balance_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    operator_bond_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    timestamp TIMESTAMPTZ NOT NULL,
    signature TEXT NOT NULL,
    block_hash VARCHAR(64) NOT NULL,
    block_height BIGINT NOT NULL,
    merkle_root VARCHAR(64) NOT NULL,
    user_merkle_proofs JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reserve_proofs_timestamp ON reserve_proofs (timestamp DESC);
CREATE INDEX idx_reserve_proofs_block ON reserve_proofs (block_height DESC);

-- Reserve verification log
CREATE TABLE reserve_verifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    proof_id UUID NOT NULL REFERENCES reserve_proofs(id),
    user_id UUID REFERENCES users(id),
    ip_address INET,
    verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reserve_verifications_proof ON reserve_verifications (proof_id);
CREATE INDEX idx_reserve_verifications_user ON reserve_verifications (user_id);

-- ============================================
-- Operator Bond
-- ============================================

CREATE TABLE operator_bonds (
    id SERIAL PRIMARY KEY,
    amount_pico NUMERIC(39, 0) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    locked_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE operator_slashing (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bond_id INT NOT NULL REFERENCES operator_bonds(id),
    amount_pico NUMERIC(39, 0) NOT NULL,
    reason TEXT NOT NULL,
    slashed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================
-- DAO Governance
-- ============================================

CREATE TYPE proposal_status AS ENUM ('draft', 'active', 'passed', 'rejected', 'expired', 'cancelled');
CREATE TYPE proposal_type AS ENUM ('general', 'treasury', 'governance', 'emergency');

CREATE TABLE dao_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    proposal_type proposal_type NOT NULL DEFAULT 'general',
    payload JSONB DEFAULT '{}',
    status proposal_status NOT NULL DEFAULT 'draft',
    yes_votes_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    no_votes_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    quorum_votes_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    author_id UUID NOT NULL REFERENCES users(id),
    voting_started_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    executed_at TIMESTAMPTZ,
    execution_result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dao_proposals_status ON dao_proposals (status, expires_at);
CREATE INDEX idx_dao_proposals_author ON dao_proposals (author_id);
CREATE INDEX idx_dao_proposals_type ON dao_proposals (proposal_type);

-- DAO Votes
CREATE TABLE dao_votes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    proposal_id UUID NOT NULL REFERENCES dao_proposals(id) ON DELETE CASCADE,
    voter_id UUID NOT NULL REFERENCES users(id),
    support BOOLEAN NOT NULL,
    voting_power_pico NUMERIC(39, 0) NOT NULL,
    signature TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(proposal_id, voter_id)
);

CREATE INDEX idx_dao_votes_proposal ON dao_votes (proposal_id);
CREATE INDEX idx_dao_votes_voter ON dao_votes (voter_id);

-- DAO Settings
CREATE TABLE dao_settings (
    id SERIAL PRIMARY KEY CHECK(id = 1),
    voting_period_hours INT NOT NULL DEFAULT 168,
    quorum_percentage INT NOT NULL DEFAULT 20,
    min_proposal_deposit_pico NUMERIC(39, 0) NOT NULL DEFAULT 10000000000000,
    min_voting_power_pico NUMERIC(39, 0) NOT NULL DEFAULT 1000000000000,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO dao_settings DEFAULT VALUES;

-- ============================================
-- Referral System
-- ============================================

CREATE TABLE referrals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    referrer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    referred_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    referral_code VARCHAR(20) NOT NULL,
    commission_earned_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    commission_claimed_pico NUMERIC(39, 0) NOT NULL DEFAULT 0,
    first_purchase_completed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(referrer_id, referred_id)
);

CREATE INDEX idx_referrals_referrer ON referrals (referrer_id);
CREATE INDEX idx_referrals_referred ON referrals (referred_id);
CREATE INDEX idx_referrals_code ON referrals (referral_code);

-- Referral payments
CREATE TABLE referral_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    referral_id UUID NOT NULL REFERENCES referrals(id),
    order_id UUID NOT NULL REFERENCES orders(id),
    commission_pico NUMERIC(39, 0) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_referral_payments_referral ON referral_payments (referral_id);

-- ============================================
-- Market Settings
-- ============================================

CREATE TABLE market_settings (
    id SERIAL PRIMARY KEY CHECK(id = 1),
    site_name VARCHAR(100) NOT NULL DEFAULT 'Myrmidons',
    tagline VARCHAR(255),
    fee_percentage DECIMAL(5,2) NOT NULL DEFAULT 5.00,
    insurance_percentage DECIMAL(5,2) NOT NULL DEFAULT 2.00,
    referral_percentage DECIMAL(5,2) NOT NULL DEFAULT 10.00,
    min_withdrawal_pico NUMERIC(39, 0) NOT NULL DEFAULT 10000000000,
    max_withdrawal_pico NUMERIC(39, 0),
    withdrawal_fee_pico NUMERIC(39, 0) NOT NULL DEFAULT 5000000000,
    payment_window_hours INT NOT NULL DEFAULT 6,
    finalized_window_days INT NOT NULL DEFAULT 14,
    auto_dispute_days INT NOT NULL DEFAULT 30,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO market_settings DEFAULT VALUES;

-- ============================================
-- Admin Audit Log
-- ============================================

CREATE TABLE admin_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id UUID NOT NULL REFERENCES users(id),
    action VARCHAR(100) NOT NULL,
    target_type VARCHAR(50),
    target_id UUID,
    details JSONB,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admin_actions_admin ON admin_actions (admin_id, created_at DESC);
CREATE INDEX idx_admin_actions_target ON admin_actions (target_type, target_id);
