package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/shlex"
	"github.com/oriser/bolt/service"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func (s *SlackBot) ListenAndServe(ctx context.Context) error {
	for i := 0; i < s.mentionsWorkers; i++ {
		go s.mentionsWorker(ctx)
	}

	for i := 0; i < s.linksWorkers; i++ {
		go s.linksWorker(ctx)
	}

	for i := 0; i < s.reactionsWorkers; i++ {
		go s.reactionsAddWorker(ctx)
	}

	http.HandleFunc("/events-endpoint", s.eventsEndpoint)
	http.HandleFunc("/add-user", func(w http.ResponseWriter, r *http.Request) {
		responseWritten, err := s.handleAddUserCommand(ctx, r, w)
		if err != nil {
			log.Printf("handleAddUserCommand: %v\n", err)
			if !responseWritten {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	})

	log.Println("Server listening on port", s.port)
	return http.ListenAndServe(fmt.Sprintf(":%d", s.port), nil)
}

// eventsEndpoint handles all event callbacks from Slack
func (s *SlackBot) eventsEndpoint(w http.ResponseWriter, r *http.Request) {
	body, event, err := s.parseMessage(w, r)
	if err != nil {
		log.Println("Error parsing message: ", err)
		return
	}

	if event.Type == slackevents.URLVerification {
		if err := s.handleURLVerification(body, w); err != nil {
			log.Println("Error responding URL verification")
		}
		return
	}

	if event.Type == slackevents.CallbackEvent {
		innerEvent := event.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.ReactionAddedEvent:
			select {
			case s.reactionsAddCh <- ev:
			case <-time.After(1 * time.Second):
				w.WriteHeader(http.StatusTooManyRequests)
			}
		case *slackevents.AppMentionEvent:
			select {
			case s.mentionsCh <- ev:
			case <-time.After(1 * time.Second):
				w.WriteHeader(http.StatusTooManyRequests)
			}
		case *slackevents.LinkSharedEvent:
			select {
			case s.linksCh <- ev:
			case <-time.After(1 * time.Second):
				w.WriteHeader(http.StatusTooManyRequests)
			}
		}
	}
}

func (s *SlackBot) parseMessage(w http.ResponseWriter, r *http.Request) ([]byte, slackevents.EventsAPIEvent, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, slackevents.EventsAPIEvent{}, fmt.Errorf("read body: %w", err)
	}

	if !s.disableSecretVerification {

		sv, err := slack.NewSecretsVerifier(r.Header, s.signinSecret)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return body, slackevents.EventsAPIEvent{}, fmt.Errorf("create secret verifier: %w", err)
		}
		if _, err := sv.Write(body); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return body, slackevents.EventsAPIEvent{}, fmt.Errorf("write to secret verifier: %w", err)
		}
		if err := sv.Ensure(); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return body, slackevents.EventsAPIEvent{}, fmt.Errorf("ensure message signature: %w", err)
		}
	}

	eventsAPIEvent, err := slackevents.ParseEvent(body, slackevents.OptionNoVerifyToken())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return body, slackevents.EventsAPIEvent{}, fmt.Errorf("parse event: %w", err)
	}

	return body, eventsAPIEvent, nil
}

func (s *SlackBot) handleURLVerification(body []byte, w http.ResponseWriter) error {
	var r *slackevents.ChallengeResponse
	err := json.Unmarshal([]byte(body), &r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("unmarshal body: %w", err)
	}
	w.Header().Set("Content-Type", "text")
	_, _ = w.Write([]byte(r.Challenge))
	return nil
}

func (s *SlackBot) handleMention(_ *slackevents.AppMentionEvent) error {
	// Currently unimplemented
	return nil
}

func (s *SlackBot) mentionsWorker(ctx context.Context) {
	for {
		select {
		case event := <-s.mentionsCh:
			if err := s.handleMention(event); err != nil {
				log.Println("Error handling mention:", err)
			}
		case <-ctx.Done():
			log.Println("Finishing mention worker due to context cancellation")
			return
		}
	}
}

func (s *SlackBot) handleLink(linkEvent *slackevents.LinkSharedEvent) error {
	if linkEvent.Channel == "COMPOSER" {
		// COMPOSER link are the unfurl link event sent when the link is unfurled in the client.
		// This is not interesting for us as the link didn't send yet and we don't have a valid channel.
		return nil
	}

	links := make([]service.Link, len(linkEvent.Links))
	for i, l := range linkEvent.Links {
		links[i] = service.Link{
			Domain: l.Domain,
			URL:    l.URL,
		}
	}

	response, err := s.service.HandleLinkMessage(service.LinksRequest{
		Links:     links,
		MessageID: linkEvent.MessageTimeStamp,
		Channel:   linkEvent.Channel,
	})
	if err != nil {
		return fmt.Errorf("link handler: %w", err)
	}

	if response != "" {
		if _, _, err := s.PostMessage(linkEvent.Channel, slack.MsgOptionText(response, false)); err != nil {
			return fmt.Errorf("post message: %w", err)
		}
	}

	return nil
}

func (s *SlackBot) linksWorker(ctx context.Context) {
	for {
		select {
		case event := <-s.linksCh:
			if err := s.handleLink(event); err != nil {
				log.Println("Error handling link:", err)
			}
		case <-ctx.Done():
			log.Println("Finishing mention worker due to context cancellation")
			return
		}
	}
}

func (s *SlackBot) handleReactionAdd(event *slackevents.ReactionAddedEvent) error {
	msgs, _, _, err := s.GetConversationReplies(&slack.GetConversationRepliesParameters{
		ChannelID: event.Item.Channel,
		Timestamp: event.Item.Timestamp,
		Limit:     1,
	})
	if err != nil {
		return fmt.Errorf("GetConversationReplies: %w", err)
	}
	if len(msgs) == 0 {
		return fmt.Errorf("no messages found for conversation with ts %s", event.Item.Timestamp)
	}

	response, err := s.service.HandleReactionAdded(service.ReactionAddRequest{
		Reaction:      event.Reaction,
		FromUserID:    event.User,
		Channel:       event.Item.Channel,
		MessageUserID: event.ItemUser,
		MessageText:   msgs[0].Text,
	})
	if err != nil {
		return fmt.Errorf("reaction add handler: %w", err)
	}

	if response != "" {
		if _, _, err := s.PostMessage(event.Item.Channel, slack.MsgOptionText(response, false)); err != nil {
			return fmt.Errorf("post message: %w", err)
		}
	}

	return nil
}

func (s *SlackBot) reactionsAddWorker(ctx context.Context) {
	for {
		select {
		case event := <-s.reactionsAddCh:
			if err := s.handleReactionAdd(event); err != nil {
				log.Println("Error handling reaction:", err)
			}
		case <-ctx.Done():
			log.Println("Finishing reaction worker due to context cancellation")
			return
		}
	}
}

func (s *SlackBot) getUserByUserName(ctx context.Context, userName string) (slack.User, error) {
	var err error
	paginatedUsers := s.GetUsersPaginated()
	for {
		paginatedUsers, err = paginatedUsers.Next(ctx)
		if err != nil {
			break
		}
		for _, user := range paginatedUsers.Users {
			if user.Name == userName {
				return user, nil
			}
		}
	}
	if err = paginatedUsers.Failure(err); err != nil {
		return slack.User{}, fmt.Errorf("list users: %w", err)
	}

	return slack.User{}, fmt.Errorf("user %q not found", userName)
}

func (s *SlackBot) handleAddUserCommand(ctx context.Context, r *http.Request, w http.ResponseWriter) (responseWritten bool, err error) {
	if err := r.ParseForm(); err != nil {
		return false, fmt.Errorf("parse form: %w", err)
	}

	userID := r.Form.Get("user_id")
	if _, ok := s.adminsUserIds[userID]; !ok {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
		return true, fmt.Errorf("unauthorized")
	}

	if r.Form.Get("command") != "/add-user" {
		return false, fmt.Errorf("unknown command %q", r.Form.Get("command"))
	}

	// Parse command
	splitted, err := shlex.Split(r.Form.Get("text"))
	if err != nil {
		return false, fmt.Errorf("shlex split %q: %w", r.Form.Get("text"), err)
	}
	if len(splitted) != 2 || !strings.HasPrefix(splitted[1], "@") {
		_, _ = w.Write([]byte("USAGE: \"<name>\" @<user>"))
		return true, fmt.Errorf("bad usage")
	}

	// Get user from Slack (unfortunately can't get directly, need to search for it)
	user, err := s.getUserByUserName(ctx, splitted[1][1:])
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			_, _ = w.Write([]byte(err.Error()))
			return true, err
		}
		return false, fmt.Errorf("getUserByUserName: %w", err)
	}

	if err := s.service.HandleAddUser(splitted[0], user); err != nil {
		_, _ = w.Write([]byte(fmt.Sprintf("Error adding user: %v", err)))
		return true, err
	}
	_, _ = w.Write([]byte(fmt.Sprintf("OK, got you. I added <@%s> as %q", user.ID, splitted[0])))
	return true, nil
}
