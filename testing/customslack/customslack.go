package customslack

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/oriser/bolt/testing/utils"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
)

type Reaction struct {
	Name      string
	Channel   string
	Timestamp string
}

type SlackUser struct {
	Name     string
	Email    string
	Phone    string
	Timezone string
}

type SlackCustomize func(customize slacktest.Customize)

type Handlers struct {
	GotReactions          []Reaction
	Members               []slack.User
	ConversationalReplies map[string]map[string][]slack.Message // Map of channel to map of timestamps to messages
	membersIDs            map[string]string                     // map between full name to ID to avoid adding the same user twice
	membersMap            map[string]slack.User                 // map between id to slack.User obj
	l                     sync.RWMutex
}

func NewHandlers() *Handlers {
	return &Handlers{
		GotReactions:          make([]Reaction, 0),
		Members:               make([]slack.User, 0),
		ConversationalReplies: make(map[string]map[string][]slack.Message),
		membersIDs:            make(map[string]string),
		membersMap:            make(map[string]slack.User),
	}
}

func (h *Handlers) Register(customize slacktest.Customize) {
	customize.Handle("/reactions.add", h.reactionHandler)
	customize.Handle("/users.list", h.queryUsers)
	customize.Handle("/users.info", h.getUser)
	customize.Handle("/conversations.replies", h.getConversationalReplies)
}

func (h *Handlers) AddConversationReply(channel, timestamp string, msg slack.Message) {
	h.l.Lock()
	defer h.l.Unlock()

	timestamps := h.ConversationalReplies[channel]
	if timestamps == nil {
		timestamps = make(map[string][]slack.Message)
		h.ConversationalReplies[channel] = timestamps
	}

	timestamps[timestamp] = append(timestamps[timestamp], msg)
}

func (h *Handlers) AddSlackUser(user SlackUser) string {
	h.l.Lock()
	defer h.l.Unlock()

	if id, ok := h.membersIDs[user.Name]; ok {
		return id
	}

	id := utils.GenerateRandomString(append(utils.CapitalLetters, utils.NumberLetters...), 6)
	member := slack.User{
		ID:       id,
		Name:     user.Name,
		Deleted:  false,
		RealName: user.Name,
		TZ:       user.Timezone,
		Profile: slack.UserProfile{
			RealNameNormalized: user.Name,
			Email:              user.Email,
			Phone:              user.Phone,
		},
	}
	h.Members = append(h.Members, member)
	h.membersMap[id] = member
	h.membersIDs[user.Name] = id

	return id
}

func (h *Handlers) writeError(w http.ResponseWriter, msg string, err error) {
	m := fmt.Sprintf("%s: %v", msg, err)
	log.Printf("%s\n", m)
	http.Error(w, m, http.StatusInternalServerError)
}

func (h *Handlers) writeOKRes(w http.ResponseWriter) {
	res := slack.SlackResponse{
		Ok: true,
	}
	output, err := json.Marshal(res)
	if err != nil {
		h.writeError(w, "error marshaling response", err)
		return
	}
	_, _ = w.Write(output)
}

func (h *Handlers) getURLValues(r *http.Request) (url.Values, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("parse body: %w", err)
	}
	return values, nil
}

func (h *Handlers) reactionHandler(w http.ResponseWriter, r *http.Request) {
	values, err := h.getURLValues(r)
	if err != nil {
		h.writeError(w, "error parsing body url query", err)
		return
	}

	h.l.Lock()
	defer h.l.Unlock()
	h.GotReactions = append(h.GotReactions, Reaction{
		Name:      values.Get("name"),
		Channel:   values.Get("channel"),
		Timestamp: values.Get("timestamp"),
	})

	h.writeOKRes(w)
}

func (h *Handlers) queryUsers(w http.ResponseWriter, _ *http.Request) {
	res := map[string]any{ // nolint
		"members": h.Members,
	}
	output, err := json.Marshal(res)
	if err != nil {
		h.writeError(w, "error marshaling members", err)
		return
	}
	_, _ = w.Write(output)
}

func (h *Handlers) getUser(w http.ResponseWriter, r *http.Request) {
	values, err := h.getURLValues(r)
	if err != nil {
		h.writeError(w, "error parsing body url query", err)
		return
	}

	id := values.Get("user")
	h.l.RLock()
	member, ok := h.membersMap[id]
	h.l.RUnlock()
	if !ok {
		h.writeError(w, "User not found", fmt.Errorf("user not found"))
		return
	}

	res := map[string]any{ // nolint
		"user": member,
	}
	output, err := json.Marshal(res)
	if err != nil {
		h.writeError(w, "error marshaling user", err)
		return
	}
	_, _ = w.Write(output)
}

func (h *Handlers) getConversationalReplies(w http.ResponseWriter, r *http.Request) {
	values, err := h.getURLValues(r)
	if err != nil {
		h.writeError(w, "error parsing body url query", err)
		return
	}

	h.l.RLock()
	defer h.l.RUnlock()

	timestamps := h.ConversationalReplies[values.Get("channel")]
	if timestamps == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	messages := timestamps[values.Get("ts")]
	if len(messages) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	res := map[string]any{ // nolint
		"messages": messages,
	}
	output, err := json.Marshal(res)
	if err != nil {
		h.writeError(w, "error marshaling messages", err)
		return
	}
	_, _ = w.Write(output)
}
