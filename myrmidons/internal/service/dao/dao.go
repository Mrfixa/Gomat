package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service handles DAO governance
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a new DAO service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// CreateProposal creates a new DAO proposal
func (s *Service) CreateProposal(ctx context.Context, params CreateProposalParams) (*Proposal, error) {
	// Check minimum deposit
	var minDeposit string
	err := s.db.QueryRow(ctx, `SELECT min_proposal_deposit_pico FROM dao_settings LIMIT 1`).Scan(&minDeposit)
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}
	minDepositDecimal, _ := decimal.NewFromString(minDeposit)

	if params.Deposit.LessThan(minDepositDecimal) {
		return nil, fmt.Errorf("deposit below minimum: %s", minDeposit)
	}

	// Get voting period
	var votingPeriod int
	s.db.QueryRow(ctx, `SELECT voting_period_hours FROM dao_settings LIMIT 1`).Scan(&votingPeriod)
	if votingPeriod == 0 {
		votingPeriod = 168 // Default 1 week
	}

	proposalID := uuid.New()
	now := time.Now()
	expiresAt := now.Add(time.Duration(votingPeriod) * time.Hour)

	// Serialize payload
	payloadJSON, err := json.Marshal(params.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize payload: %w", err)
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO dao_proposals (
			id, title, description, proposal_type, payload,
			status, author_id, voting_started_at, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, 'active', $6, $7, $8, $7)
	`, proposalID, params.Title, params.Description, params.Type,
		payloadJSON, params.AuthorID, now, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create proposal: %w", err)
	}

	// Burn deposit (transfer to null/burn address - handled by payment service)
	// For now, just record it

	return s.GetProposal(ctx, proposalID)
}

// CreateProposalParams contains parameters for proposal creation
type CreateProposalParams struct {
	Title       string
	Description string
	Type        string // general, treasury, governance, emergency
	Payload     map[string]interface{}
	AuthorID    uuid.UUID
	Deposit     decimal.Decimal
}

// GetProposal retrieves a proposal by ID
func (s *Service) GetProposal(ctx context.Context, id uuid.UUID) (*Proposal, error) {
	var proposal Proposal
	var payloadJSON []byte
	var votingStartedAt *time.Time

	err := s.db.QueryRow(ctx, `
		SELECT id, title, description, proposal_type, payload,
			status, yes_votes_pico, no_votes_pico, quorum_votes_pico,
			author_id, voting_started_at, expires_at, created_at
		FROM dao_proposals WHERE id = $1
	`, id).Scan(
		&proposal.ID, &proposal.Title, &proposal.Description,
		&proposal.Type, &payloadJSON,
		&proposal.Status, &proposal.YesVotesPico, &proposal.NoVotesPico,
		&proposal.QuorumVotesPico, &proposal.AuthorID,
		&votingStartedAt, &proposal.ExpiresAt, &proposal.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("proposal not found: %w", err)
	}

	if votingStartedAt != nil {
		proposal.VotingStartedAt = votingStartedAt
	}

	if err := json.Unmarshal(payloadJSON, &proposal.Payload); err != nil {
		proposal.Payload = make(map[string]interface{})
	}

	return &proposal, nil
}

// CastVote casts a vote on a proposal
func (s *Service) CastVote(ctx context.Context, params VoteParams) error {
	// Get proposal
	proposal, err := s.GetProposal(ctx, params.ProposalID)
	if err != nil {
		return err
	}

	// Check if voting is active
	if proposal.Status != "active" {
		return fmt.Errorf("proposal is not accepting votes")
	}

	// Check if expired
	if time.Now().After(proposal.ExpiresAt) {
		return fmt.Errorf("voting period has ended")
	}

	// Get voter's balance (voting power)
	var balance string
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(balance_pico, 0)::TEXT FROM wallets WHERE user_id = $1
	`, params.VoterID).Scan(&balance)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}
	votingPower, _ := decimal.NewFromString(balance)

	// Generate signature (placeholder)
	signature := fmt.Sprintf("VOTE-%s-%s-%d", params.ProposalID.String(), params.VoterID.String(), time.Now().Unix())

	// Insert vote
	_, err = s.db.Exec(ctx, `
		INSERT INTO dao_votes (id, proposal_id, voter_id, support, voting_power_pico, signature)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (proposal_id, voter_id) DO UPDATE SET
			support = $4, voting_power_pico = $5, signature = $6
	`, uuid.New(), params.ProposalID, params.VoterID, params.Support, votingPower.BigInt(), signature)
	if err != nil {
		return fmt.Errorf("failed to cast vote: %w", err)
	}

	// Update vote totals
	if params.Support {
		_, err = s.db.Exec(ctx, `
			UPDATE dao_proposals 
			SET yes_votes_pico = yes_votes_pico + $1
			WHERE id = $2
		`, votingPower.BigInt(), params.ProposalID)
	} else {
		_, err = s.db.Exec(ctx, `
			UPDATE dao_proposals 
			SET no_votes_pico = no_votes_pico + $1
			WHERE id = $2
		`, votingPower.BigInt(), params.ProposalID)
	}
	if err != nil {
		return fmt.Errorf("failed to update vote totals: %w", err)
	}

	return nil
}

// VoteParams contains parameters for voting
type VoteParams struct {
	ProposalID uuid.UUID
	VoterID    uuid.UUID
	Support    bool
}

// GetActiveProposals returns active proposals
func (s *Service) GetActiveProposals(ctx context.Context) ([]*Proposal, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, title, description, proposal_type, payload,
			status, yes_votes_pico, no_votes_pico, quorum_votes_pico,
			author_id, expires_at, created_at
		FROM dao_proposals
		WHERE status = 'active' AND expires_at > NOW()
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query proposals: %w", err)
	}
	defer rows.Close()

	var proposals []*Proposal
	for rows.Next() {
		var proposal Proposal
		var payloadJSON []byte
		err := rows.Scan(
			&proposal.ID, &proposal.Title, &proposal.Description,
			&proposal.Type, &payloadJSON,
			&proposal.Status, &proposal.YesVotesPico, &proposal.NoVotesPico,
			&proposal.QuorumVotesPico, &proposal.AuthorID,
			&proposal.ExpiresAt, &proposal.CreatedAt,
		)
		if err != nil {
			continue
		}
		json.Unmarshal(payloadJSON, &proposal.Payload)
		proposals = append(proposals, &proposal)
	}
	return proposals, nil
}

// GetUserVotes returns votes by a user
func (s *Service) GetUserVotes(ctx context.Context, userID uuid.UUID) ([]*Vote, error) {
	rows, err := s.db.Query(ctx, `
		SELECT dv.id, dv.proposal_id, dv.support, dv.voting_power_pico,
			dv.created_at, dp.title
		FROM dao_votes dv
		JOIN dao_proposals dp ON dv.proposal_id = dp.id
		WHERE dv.voter_id = $1
		ORDER BY dv.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query votes: %w", err)
	}
	defer rows.Close()

	var votes []*Vote
	for rows.Next() {
		var vote Vote
		err := rows.Scan(
			&vote.ID, &vote.ProposalID, &vote.Support,
			&vote.VotingPowerPico, &vote.CreatedAt, &vote.ProposalTitle,
		)
		if err != nil {
			continue
		}
		votes = append(votes, &vote)
	}
	return votes, nil
}

// ProcessExpiredProposals checks and processes expired proposals
func (s *Service) ProcessExpiredProposals(ctx context.Context) error {
	// Get quorum threshold
	var quorumPct int
	err := s.db.QueryRow(ctx, `SELECT quorum_percentage FROM dao_settings LIMIT 1`).Scan(&quorumPct)
	if err != nil {
		quorumPct = 20 // Default
	}

	// Get expired proposals
	rows, err := s.db.Query(ctx, `
		SELECT id, yes_votes_pico, no_votes_pico, quorum_votes_pico
		FROM dao_proposals
		WHERE status = 'active' AND expires_at <= NOW()
	`)
	if err != nil {
		return fmt.Errorf("failed to query expired proposals: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var yesVotes, noVotes, quorum string
		if err := rows.Scan(&id, &yesVotes, &noVotes, &quorum); err != nil {
			continue
		}

		yesDecimal, _ := decimal.NewFromString(yesVotes)
		noDecimal, _ := decimal.NewFromString(noVotes)
		totalVotes := yesDecimal.Add(noDecimal)

		quorumDecimal, _ := decimal.NewFromString(quorum)
		threshold := quorumDecimal.Mul(decimal.NewFromInt(int64(quorumPct))).Div(decimal.NewFromInt(100))

		var newStatus string
		if totalVotes.LessThan(threshold) {
			newStatus = "expired" // Not enough participation
		} else if yesDecimal.GreaterThan(noDecimal) {
			newStatus = "passed"
		} else {
			newStatus = "rejected"
		}

		_, err = s.db.Exec(ctx, `
			UPDATE dao_proposals SET status = $1 WHERE id = $2
		`, newStatus, id)
		if err != nil {
			continue
		}
	}

	return nil
}

// Proposal represents a DAO proposal
type Proposal struct {
	ID              uuid.UUID
	Title           string
	Description     string
	Type            string
	Payload         map[string]interface{}
	Status          string
	YesVotesPico    decimal.Decimal
	NoVotesPico     decimal.Decimal
	QuorumVotesPico decimal.Decimal
	AuthorID        uuid.UUID
	VotingStartedAt *time.Time
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

// Vote represents a DAO vote
type Vote struct {
	ID             uuid.UUID
	ProposalID     uuid.UUID
	Support        bool
	VotingPowerPico decimal.Decimal
	CreatedAt      time.Time
	ProposalTitle  string
}
