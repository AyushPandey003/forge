package forge

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MaxAIProviders bounds how many provider connections one user can keep.
const MaxAIProviders = 10

// AIProvider is a user's saved model-provider connection. The API key is only
// ever held as ciphertext here; it is never serialised.
type AIProvider struct {
	ID            string    `json:"id"`
	Label         string    `json:"label"`
	Kind          string    `json:"kind"`
	BaseURL       string    `json:"base_url"`
	KeyHint       string    `json:"key_hint"`
	HasKey        bool      `json:"has_key"`
	DefaultModel  string    `json:"default_model"`
	CreatedAt     time.Time `json:"created_at"`
	KeyCiphertext []byte    `json:"-"`
}

const providerCols = `id, label, kind, base_url, key_hint, default_model, created_at, key_ciphertext`

func scanProvider(row pgx.Row) (AIProvider, error) {
	var p AIProvider
	if err := row.Scan(&p.ID, &p.Label, &p.Kind, &p.BaseURL, &p.KeyHint, &p.DefaultModel, &p.CreatedAt, &p.KeyCiphertext); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return p, ErrNotFound
		}
		return p, err
	}
	p.HasKey = len(p.KeyCiphertext) > 0
	return p, nil
}

func (s *Store) ListAIProviders(ctx context.Context, userID string) ([]AIProvider, error) {
	rows, err := s.db.Query(ctx, `SELECT `+providerCols+` FROM forge_ai_providers WHERE user_id=$1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AIProvider{}
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetAIProvider only ever returns the caller's own provider.
func (s *Store) GetAIProvider(ctx context.Context, userID, id string) (AIProvider, error) {
	return scanProvider(s.db.QueryRow(ctx, `SELECT `+providerCols+` FROM forge_ai_providers WHERE user_id=$1 AND id=$2`, userID, id))
}

func (s *Store) CreateAIProvider(ctx context.Context, userID string, p AIProvider) (AIProvider, error) {
	var n int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM forge_ai_providers WHERE user_id=$1`, userID).Scan(&n); err != nil {
		return AIProvider{}, err
	}
	if n >= MaxAIProviders {
		return AIProvider{}, ErrInvalid
	}
	p.ID = newID("aip")
	row := s.db.QueryRow(ctx, `INSERT INTO forge_ai_providers (id, user_id, label, kind, base_url, key_ciphertext, key_hint, default_model)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+providerCols,
		p.ID, userID, p.Label, p.Kind, p.BaseURL, p.KeyCiphertext, p.KeyHint, p.DefaultModel)
	out, err := scanProvider(row)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return AIProvider{}, ErrConflict
	}
	return out, err
}

// UpdateAIProvider changes a provider; a nil key keeps the stored one.
func (s *Store) UpdateAIProvider(ctx context.Context, userID string, p AIProvider, newKey []byte, newHint string) (AIProvider, error) {
	key, hint := p.KeyCiphertext, p.KeyHint
	if newKey != nil {
		key, hint = newKey, newHint
	}
	row := s.db.QueryRow(ctx, `UPDATE forge_ai_providers SET label=$3, kind=$4, base_url=$5, default_model=$6,
		key_ciphertext=$7, key_hint=$8, updated_at=now() WHERE user_id=$1 AND id=$2 RETURNING `+providerCols,
		userID, p.ID, p.Label, p.Kind, p.BaseURL, p.DefaultModel, key, hint)
	out, err := scanProvider(row)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return AIProvider{}, ErrConflict
	}
	return out, err
}

func (s *Store) DeleteAIProvider(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM forge_ai_providers WHERE user_id=$1 AND id=$2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// KeyHint shows the last characters of a key so users can tell keys apart.
func KeyHint(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 8 {
		return ""
	}
	return "…" + key[len(key)-4:]
}
