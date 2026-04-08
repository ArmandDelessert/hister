// SPDX-License-Identifier: AGPL-3.0-or-later

package vectorstore

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/asciimoo/hister/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog/log"
)

type pgVectorStore struct {
	db         *sql.DB
	dimensions int
}

func newPostgres(cfg *config.Config) (VectorStore, error) {
	_, dsn := cfg.DatabaseConnection()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres for vectors: %w", err)
	}
	return &pgVectorStore{
		db:         db,
		dimensions: cfg.SemanticSearch.Dimensions,
	}, nil
}

func (p *pgVectorStore) Init() error {
	if _, err := p.db.Exec(`CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		return fmt.Errorf("create pgvector extension: %w", err)
	}
	log.Info().Msg("pgvector extension enabled")

	stmt := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS embeddings (
		doc_id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL DEFAULT 0,
		embedding vector(%d)
	)`, p.dimensions)
	if _, err := p.db.Exec(stmt); err != nil {
		return fmt.Errorf("create embeddings table: %w", err)
	}

	// HNSW index for cosine distance.
	_, err := p.db.Exec(`CREATE INDEX IF NOT EXISTS embeddings_hnsw_idx
		ON embeddings USING hnsw (embedding vector_cosine_ops)`)
	if err != nil {
		return fmt.Errorf("create HNSW index: %w", err)
	}
	// Index on user_id for efficient filtering.
	_, err = p.db.Exec(`CREATE INDEX IF NOT EXISTS embeddings_user_idx ON embeddings (user_id)`)
	if err != nil {
		return fmt.Errorf("create user_id index: %w", err)
	}
	return nil
}

func (p *pgVectorStore) Put(docID string, userID uint, vector []float32) error {
	vecStr := pgVectorLiteral(vector)
	_, err := p.db.Exec(
		`INSERT INTO embeddings(doc_id, user_id, embedding) VALUES ($1, $2, $3)
		 ON CONFLICT (doc_id) DO UPDATE SET user_id = EXCLUDED.user_id, embedding = EXCLUDED.embedding`,
		docID, userID, vecStr,
	)
	if err != nil {
		return fmt.Errorf("upsert embedding: %w", err)
	}
	return nil
}

func (p *pgVectorStore) Delete(docID string) error {
	_, err := p.db.Exec(`DELETE FROM embeddings WHERE doc_id = $1`, docID)
	if err != nil {
		return fmt.Errorf("delete embedding: %w", err)
	}
	return nil
}

func (p *pgVectorStore) Search(vector []float32, topK int, threshold float64, userID uint) (_ []Result, err error) {
	vecStr := pgVectorLiteral(vector)
	rows, err := p.db.Query(
		`SELECT doc_id, 1 - (embedding <=> $1::vector) AS similarity
		 FROM embeddings
		 WHERE 1 - (embedding <=> $1::vector) >= $2
		   AND user_id = $4
		 ORDER BY embedding <=> $1::vector
		 LIMIT $3`,
		vecStr, threshold, topK, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); err == nil {
			err = cerr
		}
	}()

	var results []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.DocID, &r.Similarity); err != nil {
			return nil, fmt.Errorf("scan vector result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (p *pgVectorStore) Clear() error {
	if _, err := p.db.Exec(`DELETE FROM embeddings`); err != nil {
		return fmt.Errorf("clear embeddings: %w", err)
	}
	return nil
}

func (p *pgVectorStore) Close() error {
	return p.db.Close()
}

// pgVectorLiteral formats a []float32 as a pgvector literal string "[1.0,2.0,3.0]".
func pgVectorLiteral(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
