package dynamic

import (
	"fmt"
	"strconv"
	"context"
	"log/slog"

	appbsky "github.com/bluesky-social/indigo/api/bsky"
	"github.com/medhir/bsky-feed-generator/feedgen/pkg/db"
)

type DynamicFeed struct {
	ctx          context.Context
	db           db.DB
	log          *slog.Logger
	FeedActorDID string
	FeedName     string
	dbFunc       func(int64, int64) ([]string, error)
}

func NewDynamicFeed(ctx context.Context, feedActorDID, feedName string, dbFunc func(limit, cursor int64) ([]string, error), log *slog.Logger) (*DynamicFeed, []string) {
	return &DynamicFeed{
		ctx:          ctx,
		log:          log,
		FeedActorDID: feedActorDID,
		FeedName:     feedName,
		dbFunc:       dbFunc,
	}, []string{feedName}
}

// GetPage returns a list of FeedDefs_SkeletonFeedPost, a new cursor, and an error
// It takes a feed name, a user DID, a limit, and a cursor
// The feed name can be used to produce different feeds from the same feed generator
func (df *DynamicFeed) GetPage(ctx context.Context, feed string, userDID string, limit int64, cursor string) ([]*appbsky.FeedDefs_SkeletonFeedPost, *string, error) {
	df.log.Info(fmt.Sprintf("Getting %d most recent posts at cursor %s", limit, cursor))
	if limit > 30 {
		limit = 30
	}

	cursorAsInt := int64(0)
	var err error

	if cursor != "" {
		cursorAsInt, err = strconv.ParseInt(cursor, 10, 64)
		if err != nil {
			df.log.Warn(fmt.Sprintf("cursor is not an integer: %v", err))
			return nil, nil, fmt.Errorf("cursor is not an integer: %w", err)
		}
	}

	tmr, err := df.dbFunc(limit, cursorAsInt)
	if err != nil {
		df.log.Warn(fmt.Sprintf("error getting %d most recent posts: %v", limit, err))
		return nil, nil, fmt.Errorf("error getting %d most recent posts: %w", limit, err)
	}
	df.log.Info(fmt.Sprintf("Got %d most recent posts:\n%v", len(tmr), tmr))

	var posts []*appbsky.FeedDefs_SkeletonFeedPost
	for _, postURI := range tmr {
		if int64(len(posts)) >= limit {
			break
		}
		posts = append(posts, &appbsky.FeedDefs_SkeletonFeedPost{
			Post: postURI,
		})
	}

	cursorAsInt += int64(len(posts))

	var newCursor *string

	if cursorAsInt < int64(210) {
		newCursor = new(string)
		*newCursor = strconv.FormatInt(cursorAsInt, 10)
	}

	df.log.Info(fmt.Sprintf("Returning %d posts:\n%#v", len(posts), posts))
	if newCursor != nil {
		df.log.Info(fmt.Sprintf("New cursor: %s", *newCursor))
	}

	return posts, newCursor, nil
}

// Describe returns a list of FeedDescribeFeedGenerator_Feed, and an error
// StaticFeed is a trivial implementation of the Feed interface, so it returns a single FeedDescribeFeedGenerator_Feed
// For a more complicated feed, this function would return a list of FeedDescribeFeedGenerator_Feed with the URIs of aliases
// supported by the feed
func (df *DynamicFeed) Describe(ctx context.Context) ([]appbsky.FeedDescribeFeedGenerator_Feed, error) {
	return []appbsky.FeedDescribeFeedGenerator_Feed{
		{
			Uri: "at://" + df.FeedActorDID + "/app.bsky.feed.generator/" + df.FeedName,
		},
	}, nil
}
