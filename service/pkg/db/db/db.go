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
	AddPost(did, rkey, uri string, nsfw bool, confidence float32) error
	DeletePost(rkey string) error
	AddLike(rkey string) error
	DeleteLike(rkey string) error
	AddRepost(rkey string) error
	DeleteRepost(rkey string) error

	MostRecentWithCursor(limit int64, cursor int64) ([]string, error)
	HottestWithCursor(limit int64, cursor int64) ([]string, error)
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
	rows, err := d.db.Query(d.ctx, "", cursor, limit)
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

func (d *dbPostgres) HottestWithCursor(limit int64, cursor int64) ([]string, error) {
	rows, err := d.db.Query(d.ctx, "", cursor, limit)
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

func (d *dbPostgres) AddPost(did, rkey, uri string, nsfw bool, confidence float32) error {
	_, err := d.db.Exec(d.ctx, "INSERT INTO post (did, record, uri, indexed_at) VALUES ($1, $2, $3, $4, $5, $6)", did, rkey, uri, nsfw, confidence, time.Now())
	return err
}

func (d *dbPostgres) DeletePost(rkey string) error {
	_, err := d.db.Exec(d.ctx, "DELETE FROM post WHERE record = $1", rkey)
	return err
}

func (d *dbPostgres) AddLike(rkey string) error {
	return nil
}

func (d *dbPostgres) DeleteLike(rkey string) error {
	return nil
}

func (d *dbPostgres) AddRepost(rkey string) error {
	return nil
}

func (d *dbPostgres) DeleteRepost(rkey string) error {
	return nil
}
