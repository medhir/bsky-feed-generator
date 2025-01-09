package stream

import (
	"context"
	"fmt"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/bluesky-social/jetstream/pkg/models"
	"github.com/medhir/bsky-feed-generator/feedgen/pkg/db"
	"log/slog"
	"os"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/jetstream/pkg/client"
	"github.com/bluesky-social/jetstream/pkg/client/schedulers/parallel"
)

const (
	jetstreamUri  = "wss://jetstream.atproto.tools/subscribe"
	bskySocialUri = "https://bsky.social"
)

type Subscriber interface {
	Run(ctx context.Context) error
}

type subscriber struct {
	ctx           context.Context
	db            db.DB
	sched         *parallel.Scheduler
	log           *slog.Logger
	xrpcClient    *xrpc.Client
	actorDID      string
	classifierURL string
}

func NewSubscriber(ctx context.Context, db db.DB, log *slog.Logger) (*subscriber, error) {
	xrpcClient := &xrpc.Client{
		Host: bskySocialUri,
	}
	actorDID := os.Getenv("FEED_ACTOR_DID")
	if actorDID == "" {
		return nil, fmt.Errorf("missing env var FEED_ACTOR_DID")
	}
	handle := os.Getenv("FEED_ACTOR_HANDLE")
	if handle == "" {
		return nil, fmt.Errorf("missing env var FEED_ACTOR_HANDLE")
	}
	password := os.Getenv("FEED_ACTOR_APP_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("missing env var FEED_ACTOR_APP_PASSWORD")
	}
	classifierURL := os.Getenv("CLASSIFIER_URL")
	if classifierURL == "" {
		return nil, fmt.Errorf("missing env var CLASSIFIER_URL")
	}
	auth, err := atproto.ServerCreateSession(ctx, xrpcClient, &atproto.ServerCreateSession_Input{
		Identifier: handle,
		Password:   password,
	})
	if err != nil {
		log.Warn(fmt.Sprintf("failed to create session: %s", err.Error()))
		return nil, err
	}
	xrpcClient.Auth = &xrpc.AuthInfo{
		AccessJwt:  auth.AccessJwt,
		RefreshJwt: auth.RefreshJwt,
	}

	// session refresh
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// https://github.com/bluesky-social/indigo/blob/c130614850e554f9862d8e649373b53cee86dd3b/cmd/beemo/notify_reports.go#L61-L63
				xrpcClient.Auth.AccessJwt = xrpcClient.Auth.RefreshJwt
				auth, err := atproto.ServerRefreshSession(ctx, xrpcClient)
				if err != nil {
					log.Warn(fmt.Sprintf("failed to refresh session: %s", err.Error()))
					continue
				}
				xrpcClient.Auth.AccessJwt = auth.AccessJwt
				xrpcClient.Auth.RefreshJwt = auth.RefreshJwt
			}
		}
	}()

	return &subscriber{
		ctx:           ctx,
		db:            db,
		log:           log,
		xrpcClient:    xrpcClient,
		classifierURL: classifierURL,
	}, nil
}

func (s *subscriber) refreshTokens() error {
	auth, err := atproto.ServerRefreshSession(s.ctx, s.xrpcClient)
	if err != nil {
		s.log.Warn(fmt.Sprintf("failed to refresh session: %s", err.Error()))
		return err
	}
	s.xrpcClient.Auth = &xrpc.AuthInfo{
		AccessJwt:  auth.AccessJwt,
		RefreshJwt: auth.RefreshJwt,
	}
	return nil
}

func (s *subscriber) connect() error {
	config := client.DefaultClientConfig()
	config.WebsocketURL = jetstreamUri
	config.Compress = true
	s.sched = parallel.NewScheduler(2, "jetstream", s.log, func(ctx context.Context, event *models.Event) error {
		return s.handleCommit(event)
	})
	c, err := client.NewClient(config, s.log, s.sched)
	if err != nil {
		s.log.Warn(fmt.Sprintf("failed to create client: %s", err.Error()))
		return err
	}

	// Every 5 seconds print the events read and bytes read and average event size
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-ticker.C:
				eventsRead := c.EventsRead.Load()
				bytesRead := c.BytesRead.Load()
				if eventsRead == 0 {
					s.log.Info("stats", "no events read")
					continue
				}
				avgEventSize := bytesRead / eventsRead
				s.log.Info("stats", "events_read", eventsRead, "bytes_read", bytesRead, "avg_event_size", avgEventSize)
			}
		}
	}()
	return c.ConnectAndRead(s.ctx, nil)
}

func (s *subscriber) Run() error {
	return s.connect()
}

const (
	CollectionKindFeedPost   = "app.bsky.feed.post"
	CollectionKindFeedRepost = "app.bsky.feed.repost"
	CollectionKindFeedLike   = "app.bsky.feed.like"
)

func (s *subscriber) handleCommit(event *models.Event) error {
	if event.Commit == nil {
		return nil
	}
	switch event.Commit.Operation {
	case models.CommitOperationCreate:
		switch event.Commit.Collection {
		case CollectionKindFeedPost:
			err := s.handleCreatePost(event)
			if err != nil {
				s.log.Warn(fmt.Sprintf("failed to handle create %s: %s", CollectionKindFeedPost, err.Error()))
				return err
			}
		case CollectionKindFeedLike:
			err := s.handleCreateLike(event)
			if err != nil {
				s.log.Warn(fmt.Sprintf("failed to handle create %s: %s", CollectionKindFeedLike, err.Error()))
				return err
			}
		case CollectionKindFeedRepost:
			err := s.handleCreateRepost(event)
			if err != nil {
				s.log.Warn(fmt.Sprintf("failed to handle create %s: %s", CollectionKindFeedRepost, err.Error()))
				return err
			}
		}
	case models.CommitOperationDelete:
		switch event.Commit.Collection {
		case CollectionKindFeedPost:
			err := s.handleDeletePost(event)
			if err != nil {
				s.log.Warn(fmt.Sprintf("failed to handle delete %s: %s", CollectionKindFeedPost, err.Error()))
				return err
			}
		case CollectionKindFeedLike:
			err := s.handleDeleteLike(event)
			if err != nil {
				s.log.Warn(fmt.Sprintf("failed to handle delete %s: %s", CollectionKindFeedLike, err.Error()))
				return err
			}
		case CollectionKindFeedRepost:
			err := s.handleDeleteRepost(event)
			if err != nil {
				s.log.Warn(fmt.Sprintf("failed to handle delete %s: %s", CollectionKindFeedRepost, err.Error()))
				return err
			}
		}
	}
	return nil
}
