package stream

import (
	"encoding/json"
	"fmt"
	appbsky "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/jetstream/pkg/models"
	"strings"
)

func (s *subscriber) handleCreatePost(event *models.Event) error {
	var post appbsky.FeedPost
	if err := json.Unmarshal(event.Commit.Record, &post); err != nil {
		s.log.Warn(fmt.Sprintf("failed to parse app.bsky.feed.post record: %s", err.Error()))
		return err
	}
	// if post is a parent post and contains an image, classify it
	isParent := post.Reply == nil
	if isParent && post.Embed != nil && post.Embed.EmbedImages != nil {
		for _, img := range post.Embed.EmbedImages.Images {
			response, err := s.classify(event.Did, img)
			if err != nil {
				s.log.Warn(fmt.Sprintf("failed to classify image: %s", err.Error()))
				continue
			}
			// if post contains picture with high confidence, add to DB
			if response.Label == "bird" && response.Confidence > 0.85 {
				did := event.Did
				rkey := event.Commit.RKey
				postURL := fmt.Sprintf("https://bsky.app/profile/%s/post/%s", did, rkey)
				s.log.Info("Bird Identified")
				s.log.Info(fmt.Sprintf("Post URL: %s", postURL))
				s.log.Info(fmt.Sprintf("Confidence: %f", response.Confidence))
				err := s.db.AddPotentialBirdPost(did, rkey, postURL)
				if err != nil {
					s.log.Warn(fmt.Sprintf("failed to add post to DB: %s", err.Error()))
					continue
				}
				s.log.Info(fmt.Sprintf("Added post to DB: %s", rkey))
				// only add one record per post, skip other images
				break
			}
		}
	}
	return nil
}

func (s *subscriber) handleDeletePost(event *models.Event) error {
	rkey := event.Commit.RKey
	if err := s.db.DeletePotentialBirdPost(rkey); err != nil {
		s.log.Warn(fmt.Sprintf("failed to delete post from DB: %s", err.Error()))
		return err
	}
	return nil
}

func (s *subscriber) handleCreateLike(event *models.Event) error {
	var like appbsky.FeedLike
	if err := json.Unmarshal(event.Commit.Record, &like); err != nil {
		s.log.Warn(fmt.Sprintf("failed to parse app.bsky.feed.like record: %s", err.Error()))
		return err
	}
	rkey := strings.Split(like.Subject.Uri, "/")[4]
	err := s.db.AddLike(rkey)
	if err != nil {
		s.log.Warn(fmt.Sprintf("failed to increment like: %s", err.Error()))
		return err
	}
	return nil
}

func (s *subscriber) handleDeleteLike(event *models.Event) error {
	return nil
}

func (s *subscriber) handleCreateRepost(event *models.Event) error {
	var repost appbsky.FeedRepost
	if err := json.Unmarshal(event.Commit.Record, &repost); err != nil {
		s.log.Warn(fmt.Sprintf("failed to parse app.bsky.feed.repost record: %s", err.Error()))
		return err
	}
	rkey := strings.Split(repost.Subject.Uri, "/")[4]
	err := s.db.AddRepost(rkey)
	if err != nil {
		s.log.Warn(fmt.Sprintf("failed to increment repost: %s", err.Error()))
		return err
	}
	return nil
}

func (s *subscriber) handleDeleteRepost(event *models.Event) error {
	return nil
}
