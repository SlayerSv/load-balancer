package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/SlayerSv/load-balancer/internal/apperrors"
	"github.com/SlayerSv/load-balancer/internal/models"

	"github.com/lib/pq"
)

// PostgreSQL is the postgres implementation for DataBase interface
type PostgreSQL struct {
	db *sql.DB
}

// NewPostgresDB returns new instance of PostgreSQL or error
func NewPostgresDB() (*PostgreSQL, error) {
	host, ok := os.LookupEnv("POSTGRES_HOST")
	if !ok {
		return nil, fmt.Errorf("missing POSTGRES_HOST env variable")
	}
	port, ok := os.LookupEnv("POSTGRES_PORT")
	if !ok {
		return nil, fmt.Errorf("missing POSTGRES_PORT env variable")
	}
	user, ok := os.LookupEnv("POSTGRES_USER")
	if !ok {
		return nil, fmt.Errorf("missing POSTGRES_USER env variable")
	}
	pass, ok := os.LookupEnv("POSTGRES_PASSWORD")
	if !ok {
		return nil, fmt.Errorf("missing POSTGRES_PASSWORD env variable")
	}
	dbname, ok := os.LookupEnv("POSTGRES_DB")
	if !ok {
		return nil, fmt.Errorf("missing POSTGRES_DB env variable")
	}
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, pass, dbname)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	return &PostgreSQL{db}, nil
}

// GetClient gets client from database by clientID
func (p *PostgreSQL) GetClient(ctx context.Context, clientID string) (models.Client, error) {
	var client models.Client
	row := p.db.QueryRowContext(ctx,
		`SELECT * FROM clients
		WHERE
			client_id = $1;
		`,
		clientID,
	)
	err := row.Scan(&client.ClientID, &client.APIKey, &client.Capacity,
		&client.RatePerSec, &client.Tokens, &client.LastRefill)
	return client, p.WrapError(err)
}

// GetClientByAPIKey gets client from database by APIKey
func (p *PostgreSQL) GetClientByAPIKey(ctx context.Context, clientAPIKey string) (models.Client, error) {
	var client models.Client
	row := p.db.QueryRowContext(ctx,
		`SELECT * FROM clients
		WHERE
			api_key = $1;
		`,
		clientAPIKey,
	)
	err := row.Scan(&client.ClientID, &client.APIKey, &client.Capacity,
		&client.RatePerSec, &client.Tokens, &client.LastRefill)
	return client, p.WrapError(err)
}

// AddClient adds client to database. Fiels clientID and APIKey must be unique
func (p *PostgreSQL) AddClient(ctx context.Context, client models.Client) (models.Client, error) {
	var newClient models.Client
	row := p.db.QueryRowContext(ctx,
		`INSERT INTO clients
			(client_id, api_key, capacity, rate_per_sec)
		VALUES
			($1, $2, $3, $4)
		RETURNING *;
		`,
		client.ClientID, client.APIKey, client.Capacity, client.RatePerSec,
	)
	err := row.Scan(&newClient.ClientID, &newClient.APIKey, &newClient.Capacity,
		&newClient.RatePerSec, &newClient.Tokens, &newClient.LastRefill)
	return newClient, p.WrapError(err)
}

// UpdateClient updates client's ratePerSec and Capacity in database.
func (p *PostgreSQL) UpdateClient(ctx context.Context, client models.Client) (models.Client, error) {
	var updatedClient models.Client
	row := p.db.QueryRowContext(ctx,
		`UPDATE clients
		SET
			capacity = $1,
			rate_per_sec = $2,
			tokens = CASE
				WHEN $1 < tokens
				THEN $1
				ELSE tokens
				END
		WHERE client_id = $3
		RETURNING *;
		`,
		client.Capacity, client.RatePerSec, client.ClientID,
	)
	err := row.Scan(&updatedClient.ClientID, &updatedClient.APIKey, &updatedClient.Capacity,
		&updatedClient.RatePerSec, &updatedClient.Tokens, &updatedClient.LastRefill)
	return updatedClient, p.WrapError(err)
}

// UpdateClient deletes client from database.
func (p *PostgreSQL) DeleteClient(ctx context.Context, clientID string) (models.Client, error) {
	row := p.db.QueryRowContext(ctx,
		`DELETE FROM clients
		WHERE
			client_id = $1
		RETURNING *;
		`,
		clientID,
	)
	var deletedClient models.Client
	err := row.Scan(&deletedClient.ClientID, &deletedClient.APIKey, &deletedClient.Capacity,
		&deletedClient.RatePerSec, &deletedClient.Tokens, &deletedClient.LastRefill)
	return deletedClient, p.WrapError(err)
}

// UpdateTokens updates client Tokens and lastRefill date
func (p *PostgreSQL) UpdateTokens(ctx context.Context, client models.Client) (models.Client, error) {
	var updatedClient models.Client
	row := p.db.QueryRowContext(ctx,
		`UPDATE clients
		SET
			tokens = $1, last_refill = $2
		WHERE
			client_id = $3
		RETURNING *;
		`,
		client.Tokens, client.LastRefill, client.ClientID,
	)
	err := row.Scan(&updatedClient.ClientID, &updatedClient.APIKey, &updatedClient.Capacity,
		&updatedClient.RatePerSec, &updatedClient.Tokens, &updatedClient.LastRefill)
	return updatedClient, p.WrapError(err)
}

func (p *PostgreSQL) WrapError(err error) error {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperrors.ErrNotFound
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			switch pqErr.Code {
			case "23514":
				return apperrors.ErrBadRequest
			case "23505":
				return apperrors.ErrAlreadyExists
			default:
				return fmt.Errorf("%w: %w", apperrors.ErrInternal, err)
			}
		}
		return fmt.Errorf("%w: %w", apperrors.ErrInternal, err)
	}
	return nil
}
