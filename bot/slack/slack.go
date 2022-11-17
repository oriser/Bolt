package slack

import (
	"fmt"

	"github.com/oriser/bolt/service"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type Config struct {
	SigninSecret              string   `env:"SLACK_SIGNIN_SECRET,required" json:"-"`
	ClientSecret              string   `env:"SLACK_OAUTH_TOKEN,required" json:"-"`
	Port                      uint     `env:"SLACK_SERVER_PORT" envDefault:"8080"`
	MaxConcurrentLinks        int      `env:"SLACK_MAX_CONCURRENT_LINKS" envDefault:"100"`
	MaxConcurrentMentions     int      `env:"SLACK_MAX_CONCURRENT_MENTIONS" envDefault:"100"`
	MaxConcurrentReactions    int      `env:"SLACK_MAX_CONCURRENT_REACTIONS" envDefault:"100"`
	AdminSlackUserID          []string `env:"ADMIN_SLACK_USER_IDS"`
	SlackAPIUrl               string   `env:"SLACK_API_URL"`                                  // only for testing
	DisableSecretVerification bool     `env:"DISABLE_SECRET_VERIFICATION" envDefault:"false"` // only for testing
}

type SlackBot struct {
	*slack.Client
	signinSecret              string
	port                      uint
	service                   *service.Service
	mentionsWorkers           int
	linksWorkers              int
	reactionsWorkers          int
	disableSecretVerification bool
	adminsUserIds             map[string]interface{}
	mentionsCh                chan *slackevents.AppMentionEvent
	linksCh                   chan *slackevents.LinkSharedEvent
	reactionsAddCh            chan *slackevents.ReactionAddedEvent
}

type Client struct {
	*slack.Client
	cfg Config
}

func NewClient(cfg Config, slackOptions ...slack.Option) *Client {
	if cfg.SlackAPIUrl != "" {
		slackOptions = append(slackOptions, slack.OptionAPIURL(cfg.SlackAPIUrl))
	}
	return &Client{
		Client: slack.New(cfg.ClientSecret, slackOptions...),
		cfg:    cfg,
	}
}

func (c *Client) GetSelfID() (string, error) {
	res, err := c.AuthTest()
	if err != nil {
		return "", fmt.Errorf("auth test: %w", err)
	}
	return res.UserID, nil
}

func (c *Client) SendMessage(receiver, event, messageID string) (string, error) {
	options := []slack.MsgOption{slack.MsgOptionText(event, false)}
	if messageID != "" {
		options = append(options, slack.MsgOptionTS(messageID))
	}
	_, ts, err := c.PostMessage(receiver, options...)
	if err != nil {
		return "", fmt.Errorf("posting message: %w", err)
	}
	return ts, nil
}

func (c *Client) AddReaction(receiver, messageID, reaction string) error {
	if err := c.Client.AddReaction(reaction, slack.ItemRef{
		Channel:   receiver,
		Timestamp: messageID,
	}); err != nil {
		return fmt.Errorf("add reaction: %w", err)
	}
	return nil
}

func (c *Client) ServiceBot(serviceHandler *service.Service) *SlackBot {
	sb := &SlackBot{
		Client:                    c.Client,
		signinSecret:              c.cfg.SigninSecret,
		port:                      c.cfg.Port,
		mentionsWorkers:           c.cfg.MaxConcurrentMentions,
		linksWorkers:              c.cfg.MaxConcurrentLinks,
		reactionsWorkers:          c.cfg.MaxConcurrentReactions,
		disableSecretVerification: c.cfg.DisableSecretVerification,
		mentionsCh:                make(chan *slackevents.AppMentionEvent),
		linksCh:                   make(chan *slackevents.LinkSharedEvent),
		reactionsAddCh:            make(chan *slackevents.ReactionAddedEvent),
		adminsUserIds:             make(map[string]interface{}),
		service:                   serviceHandler,
	}

	for _, userID := range c.cfg.AdminSlackUserID {
		sb.adminsUserIds[userID] = nil
	}
	return sb
}
