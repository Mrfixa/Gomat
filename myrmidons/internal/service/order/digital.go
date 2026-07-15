package order

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/google/uuid"
)

// DigitalService handles encrypted digital product delivery
type DigitalService struct {
	orderSvc *Service
	key      []byte // Master encryption key (should come from config/env)
}

// NewDigitalService creates a new digital goods service
func NewDigitalService(orderSvc *Service, masterKey []byte) *DigitalService {
	return &DigitalService{
		orderSvc: orderSvc,
		key:      masterKey,
	}
}

// StoreDigitalItem encrypts and stores a digital item for an order
func (s *DigitalService) StoreDigitalItem(ctx context.Context, orderItemID uuid.UUID, plaintext []byte) error {
	// Generate random nonce
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Generate item-specific key from master key
	itemKey := deriveKey(s.key, orderItemID.String())

	// Encrypt the content
	ciphertext, err := encrypt(itemKey, nonce, plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt: %w", err)
	}

	// Store in database (nonce + ciphertext)
	_, err = s.orderSvc.db.Exec(ctx, `
		INSERT INTO digital_items (id, order_item_id, ciphertext, nonce)
		VALUES ($1, $2, $3, $4)
	`, uuid.New(), orderItemID, ciphertext, nonce)
	if err != nil {
		return fmt.Errorf("failed to store digital item: %w", err)
	}

	return nil
}

// RetrieveDigitalItem decrypts and returns a digital item
func (s *DigitalService) RetrieveDigitalItem(ctx context.Context, orderItemID uuid.UUID) ([]byte, error) {
	var ciphertext, nonce []byte
	err := s.orderSvc.db.QueryRow(ctx, `
		SELECT ciphertext, nonce FROM digital_items WHERE order_item_id = $1
	`, orderItemID).Scan(&ciphertext, &nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve digital item: %w", err)
	}

	// Derive item key
	itemKey := deriveKey(s.key, orderItemID.String())

	// Decrypt
	plaintext, err := decrypt(itemKey, nonce, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// deriveKey derives an item-specific key from the master key
func deriveKey(masterKey []byte, context string) []byte {
	// Simple key derivation - in production use HKDF
	result := make([]byte, 32)
	for i := 0; i < 32; i++ {
		if i < len(masterKey) {
			result[i] = masterKey[i] ^ byte(context[i%len(context)])
		} else {
			result[i] = byte(context[i%len(context)])
		}
	}
	return result
}

// encrypt encrypts data using AES-GCM
func encrypt(key, nonce, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Seal(nil, nonce, plaintext, nil), nil
}

// decrypt decrypts data using AES-GCM
func decrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, ciphertext, nil)
}
