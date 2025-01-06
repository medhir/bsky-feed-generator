package stream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bluesky-social/indigo/api/atproto"
	appbsky "github.com/bluesky-social/indigo/api/bsky"
	"net/http"
)

type followCounts struct {
	followers int64
	follows   int64
}

func (s *subscriber) getFollowCounts(did string) (*followCounts, error) {
	profile, err := appbsky.ActorGetProfile(s.ctx, s.xrpcClient, did)
	if err != nil {
		s.log.Warn(fmt.Sprintf("failed to get profile: %s", err.Error()))
		return nil, err
	}
	return &followCounts{
		followers: *profile.FollowersCount,
		follows:   *profile.FollowsCount,
	}, nil
}

func (s *subscriber) getPostLabels(did, rkey string) (*[]string, error) {
	labelsOutput, err := atproto.LabelQueryLabels(
		s.ctx,
		s.xrpcClient,
		"",
		100,
		[]string{s.actorDID},
		[]string{fmt.Sprintf("at://%s/app.bsky.feed.post/%s", did, rkey)},
	)
	if err != nil {
		s.log.Warn(fmt.Sprintf("failed to query labels: %s", err.Error()))
		return nil, err
	}
	var labels []string
	for _, label := range labelsOutput.Labels {
		labels = append(labels, label.Val)
	}
	return &labels, nil
}

type classifyResponse struct {
	Confidence float64 `json:"confidence"`
	Label      string  `json:"label"`
}

func (s *subscriber) classify(did string, img *appbsky.EmbedImages_Image) (classifyResponse, error) {
	type classifyRequest struct {
		ImageURL string `json:"image_url"`
	}
	imageURL := fmt.Sprintf("https://cdn.bsky.app/image/feed_fullsize/plain/%s/%s@jpeg", did, img.Image.Ref.String())
	reqBody := classifyRequest{ImageURL: imageURL}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		s.log.Warn(fmt.Sprintf("failed to marshal classify request: %s", err.Error()))
		return classifyResponse{}, err
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/selfie", s.classifierURL), bytes.NewBuffer(jsonBody))
	if err != nil {
		s.log.Warn(fmt.Sprintf("failed to create classify request: %s", err.Error()))
		return classifyResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return classifyResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		s.log.Warn(fmt.Sprintf("classify request failed with status code: %d", resp.StatusCode))
		return classifyResponse{}, err
	}

	var classifyResp classifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&classifyResp); err != nil {
		s.log.Warn(fmt.Sprintf("failed to decode response body: %s", err.Error()))
		return classifyResponse{}, err
	}
	err = resp.Body.Close()
	if err != nil {
		s.log.Warn(fmt.Sprintf("failed to close response body: %s", err.Error()))
		return classifyResponse{}, err
	}
	return classifyResp, nil
}
