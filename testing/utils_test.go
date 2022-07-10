package testing

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/oriser/bolt/testing/customslack"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
)

var seenMessages sync.Map

type MessageMatchedFunc func(text string, searchFor string) (bool, error)

func RegexMatch(text string, searchFor string) (bool, error) {
	return regexp.Match(searchFor, []byte(text))
}

func ContainsMatch(text string, searchFor string) (bool, error) {
	return strings.Contains(text, searchFor), nil
}

func EqualMatch(text string, searchFor string) (bool, error) {
	return text == searchFor, nil
}

func WaitForOutboundSlackMessage(timeout time.Duration, slackServer *slacktest.Server, searchFor, channel, expectedTimestamp string, matchFunc MessageMatchedFunc) (*slack.Message, error) {
	checkInterval := time.NewTicker(50 * time.Millisecond)
	timeoutChan := time.After(timeout)
	for {
		select {
		case <-checkInterval.C:
			messages := slackServer.GetSeenOutboundMessages()
			for _, message := range messages {
				m := slack.Message{}
				err := json.Unmarshal([]byte(message), &m)
				if _, ok := seenMessages.Load(message); ok {
					// Optimization to avoid going over messages that someone already requested.
					// That means that we can't wait for the same message twice
					continue
				}
				if err != nil {
					return nil, fmt.Errorf("unmarshal message: %w", err)
				}

				if m.Channel != channel {
					continue
				}
				if expectedTimestamp != "" && m.ThreadTimestamp != expectedTimestamp {
					continue
				}

				match, err := matchFunc(m.Text, searchFor)
				if err != nil {
					return nil, fmt.Errorf("match func: %w", err)
				}
				if !match {
					continue
				}

				seenMessages.Store(message, nil)
				return &m, nil
			}
		case <-timeoutChan:
			return nil, fmt.Errorf("timeout waiting for message in channel %q, timestamp %q and message %q", channel, expectedTimestamp, searchFor)
		}
	}
}

func WaitForOutboundReaction(timeout time.Duration, slackHandlers *customslack.Handlers, expectedReaction customslack.Reaction) error {
	checkInterval := time.NewTicker(50 * time.Millisecond)
	timeoutChan := time.After(timeout)
	for {
		select {
		case <-checkInterval.C:
			for _, reaction := range slackHandlers.GotReactions {
				if reaction.Timestamp == expectedReaction.Timestamp &&
					reaction.Name == expectedReaction.Name &&
					reaction.Channel == expectedReaction.Channel {
					return nil
				}
			}
		case <-timeoutChan:
			return fmt.Errorf("timeout waiting for reaction %#v", expectedReaction)
		}
	}
}
