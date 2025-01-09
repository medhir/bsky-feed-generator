package db

import (
	"context"
	"fmt"
	"os"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dbPostgres struct {
	ctx context.Context
	db  *pgxpool.Pool
}

type DB interface {
	AddPost(did, rkey, uri string) error
	DeletePost(rkey string) error
	AddLike(did, rkey, postRkey string) error
	DeleteLike(rkey string) error
	AddRepost(did, rkey, postRkey string) error
	DeleteRepost(rkey string) error

	MostRecentWithCursor(limit int64, cursor int64) ([]string, error)
	MostPopularWithCursor(limit int64, cursor int64) ([]string, error)
}

func NewDB(ctx context.Context) (DB, error) {
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		return nil, fmt.Errorf("POSTGRES_URL not set")
	}

	dbpool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}

	return &dbPostgres{
		ctx: ctx,
		db:  dbpool,
	}, nil
}

func (d *dbPostgres) MostRecentWithCursor(limit int64, cursor int64) ([]string, error) {
	rows, err := d.db.Query(d.ctx, "SELECT did, record FROM post ORDER BY indexed_at DESC OFFSET $1 LIMIT $2", cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []string
	for rows.Next() {
		var did, record string
		if err := rows.Scan(&did, &record); err != nil {
			return nil, err
		}
		atURI := fmt.Sprintf("at://%s/app.bsky.feed.post/%s", did, record)
		posts = append(posts, atURI)
	}

	return posts, nil
}

func (d *dbPostgres) MostPopularWithCursor(limit int64, cursor int64) ([]string, error) {
	query := `
        SELECT p.did, p.record 
        FROM post p
        LEFT JOIN post_like pl ON p.record = pl.post_rkey
        GROUP BY p.did, p.record
        ORDER BY COUNT(pl.record) DESC
        OFFSET $1 LIMIT $2`

	rows, err := d.db.Query(d.ctx, query, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []string
	for rows.Next() {
		var did, record string
		if err := rows.Scan(&did, &record); err != nil {
			return nil, err
		}
		atURI := fmt.Sprintf("at://%s/app.bsky.feed.post/%s", did, record)
		posts = append(posts, atURI)
	}
	return posts, nil
}

func (d *dbPostgres) AddPost(did, rkey, uri string) error {
	_, err := d.db.Exec(d.ctx, "INSERT INTO post (did, record, uri, indexed_at) VALUES ($1, $2, $3, $4)", did, rkey, uri, time.Now())
	return err
}

func (d *dbPostgres) DeletePost(rkey string) error {
	_, err := d.db.Exec(d.ctx, "DELETE FROM post WHERE record = $1", rkey)
	return err
}

func (d *dbPostgres) AddLike(did, rkey, postRkey string) error {
	tx, err := d.db.Begin(d.ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(d.ctx)

	var exists bool
	err = tx.QueryRow(d.ctx, "SELECT EXISTS(SELECT 1 FROM post WHERE record = $1)", postRkey).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	_, err = tx.Exec(d.ctx, "INSERT INTO post_like (did, record, post_rkey, indexed_at) VALUES ($1, $2, $3, $4)",
		did, rkey, postRkey, time.Now())
	if err != nil {
		return err
	}

	return tx.Commit(d.ctx)
}

func (d *dbPostgres) DeleteLike(rkey string) error {
	_, err := d.db.Exec(d.ctx, "DELETE FROM post_like WHERE record = $1", rkey)
	return err
}

func (d *dbPostgres) AddRepost(did, rkey, postRkey string) error {
	tx, err := d.db.Begin(d.ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(d.ctx)

	var exists bool
	err = tx.QueryRow(d.ctx, "SELECT EXISTS(SELECT 1 FROM post WHERE record = $1)", postRkey).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	_, err = tx.Exec(d.ctx, "INSERT INTO post_repost (did, record, post_rkey, indexed_at) VALUES ($1, $2, $3, $4)",
		did, rkey, postRkey, time.Now())
	if err != nil {
		return err
	}

	return tx.Commit(d.ctx)
}

func (d *dbPostgres) DeleteRepost(rkey string) error {
	_, err := d.db.Exec(d.ctx, "DELETE FROM post_repost WHERE record = $1", rkey)
	return err
}
