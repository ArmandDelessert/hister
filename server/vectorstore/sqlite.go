// SPDX-License-Identifier: AGPL-3.0-or-later

package vectorstore

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"

	"github.com/asciimoo/hister/config"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	// sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

//func init() {
//	sql.Register("sqlite3_vec", &sqlite3.SQLiteDriver{
//		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
//			err := conn.LoadExtension("vec0", "sqlite3_vec_init")
//			if err != nil {
//				log.Debug().Err(err).Msg("sqlite-vec extension not loaded via vec0, trying sqlite_vec")
//				err = conn.LoadExtension("sqlite_vec", "sqlite3_vec_init")
//			}
//			if err != nil {
//				log.Debug().Err(err).Msg("sqlite-vec extension not auto-loaded; vec0 must be available")
//			}
//			return nil // non-fatal: we check at Init()
//		},
//	})
//}

type sqliteVectorStore struct {
	db         *sql.DB
	dimensions int
}

func newSQLite(cfg *config.Config) (VectorStore, error) {
	sqlite_vec.Auto()
	dbPath := cfg.FullPath(cfg.Server.Database)
	dir := filepath.Dir(dbPath)
	vecDBPath := filepath.Join(dir, "vectors.sqlite3")

	db, err := sql.Open("sqlite3", vecDBPath)
	if err != nil {
		return nil, fmt.Errorf("open vector database: %w", err)
	}
	// Single connection to avoid locking issues with SQLite.
	db.SetMaxOpenConns(1)

	return &sqliteVectorStore{
		db:         db,
		dimensions: cfg.SemanticSearch.Dimensions,
	}, nil
}

func (s *sqliteVectorStore) Init() error {
	// Verify sqlite-vec is available by querying its version.
	var version string
	if err := s.db.QueryRow("SELECT vec_version()").Scan(&version); err != nil {
		return fmt.Errorf("sqlite-vec extension not available (is the vec0 shared library installed?): %w", err)
	}
	log.Info().Str("version", version).Msg("sqlite-vec loaded")

	stmt := fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS embeddings USING vec0(
		user_id INTEGER PARTITION KEY,
		doc_id TEXT PRIMARY KEY,
		embedding FLOAT[%d]
	)`, s.dimensions)
	if _, err := s.db.Exec(stmt); err != nil {
		return fmt.Errorf("create embeddings table: %w", err)
	}
	return nil
}

func (s *sqliteVectorStore) Put(docID string, userID uint, vector []float32) error {
	// sqlite-vec virtual tables (vec0) do not support INSERT OR REPLACE, so we
	// delete any existing row first and then insert the new one.
	if _, err := s.db.Exec(`DELETE FROM embeddings WHERE doc_id = ?`, docID); err != nil {
		return fmt.Errorf("upsert embedding (delete): %w", err)
	}
	blob := float32ToBlob(vector)
	if _, err := s.db.Exec(
		`INSERT INTO embeddings(user_id, doc_id, embedding) VALUES (?, ?, ?)`,
		userID, docID, blob,
	); err != nil {
		return fmt.Errorf("upsert embedding (insert): %w", err)
	}
	return nil
}

func (s *sqliteVectorStore) Delete(docID string) error {
	_, err := s.db.Exec(`DELETE FROM embeddings WHERE doc_id = ?`, docID)
	if err != nil {
		return fmt.Errorf("delete embedding: %w", err)
	}
	return nil
}

func (s *sqliteVectorStore) Search(vector []float32, topK int, threshold float64, userID uint) (_ []Result, err error) {
	blob := float32ToBlob(vector)
	rows, err := s.db.Query(
		`SELECT doc_id, distance FROM embeddings
		 WHERE embedding MATCH ?
		   AND k = ?
		   AND user_id = ?
		 ORDER BY distance`,
		blob, topK, userID,
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
		var docID string
		var distance float64
		if err := rows.Scan(&docID, &distance); err != nil {
			return nil, fmt.Errorf("scan vector result: %w", err)
		}
		similarity := 1.0 - distance
		if similarity >= threshold {
			results = append(results, Result{
				DocID:      docID,
				Similarity: similarity,
			})
		}
	}
	return results, rows.Err()
}

func (s *sqliteVectorStore) Clear() error {
	if _, err := s.db.Exec(`DELETE FROM embeddings`); err != nil {
		return fmt.Errorf("clear embeddings: %w", err)
	}
	return nil
}

func (s *sqliteVectorStore) Close() error {
	return s.db.Close()
}

// float32ToBlob converts a []float32 to a little-endian byte slice suitable
// for sqlite-vec's vec_f32 format.
func float32ToBlob(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}
