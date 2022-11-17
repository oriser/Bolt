package slack

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	userDomain "github.com/oriser/bolt/user"
	fuzzy "github.com/paul-mannino/go-fuzzywuzzy"
	"github.com/slack-go/slack"
)

const (
	FuzzyLimit        = 10
	FuzzyMinimumScore = 75
)

type Config struct {
	OauthToken        string        `env:"SLACK_OAUTH_TOKEN,required" json:"-"`
	MaxCacheEntryTime time.Duration `env:"SLACK_STORE_MAX_CACHE_ENTRY_TIME" envDefault:"144h"` // 6 days
	SlackAPIUrl       string        `env:"SLACK_API_URL"`                                      // only for testing
}

type cacheEntry struct {
	user    *userDomain.User
	expired time.Time
}

type SlackStorage struct {
	client            *slack.Client
	lock              sync.RWMutex
	cache             map[string]cacheEntry
	maxCacheEntryTime time.Duration
}

type MatchUser struct {
	User       slack.User
	matchScore int
}

func New(cfg Config) *SlackStorage {
	var slackOptions []slack.Option
	if cfg.SlackAPIUrl != "" {
		slackOptions = append(slackOptions, slack.OptionAPIURL(cfg.SlackAPIUrl))
	}
	return &SlackStorage{
		client:            slack.New(cfg.OauthToken, slackOptions...),
		maxCacheEntryTime: cfg.MaxCacheEntryTime,
		cache:             make(map[string]cacheEntry),
	}
}

func (s *SlackStorage) AddUser(_ context.Context, _ *userDomain.User) error {
	return fmt.Errorf("not implemented for slack storage")
}

func (s *SlackStorage) saveCache(name string, user *userDomain.User) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.cache[name] = cacheEntry{
		user:    user,
		expired: time.Now().Add(s.maxCacheEntryTime),
	}
}

func (s *SlackStorage) getFromCache(name string) *userDomain.User {
	s.lock.RLock()
	entry, ok := s.cache[name]
	s.lock.RUnlock()

	if !ok {
		return nil
	}

	if time.Now().After(entry.expired) {
		s.lock.Lock()
		delete(s.cache, name)
		s.lock.Unlock()
		log.Printf("user %q expied for cache entry", entry.user.FullName)
		return nil
	}
	return entry.user
}

// findMatchedUsers find matching users names from Slack users list by fuzzy search them
func (s *SlackStorage) findMatchedUsers(searchFor string, users []slack.User) ([]*MatchUser, error) {
	searchedValues := make([]string, len(users))
	searchedValueToUser := make(map[string]slack.User)
	for i, user := range users {
		searchedValues[i] = user.Profile.RealNameNormalized
		searchedValueToUser[user.Profile.RealNameNormalized] = user
	}

	fuzzyFunc := fuzzy.Ratio
	if len(strings.Split(searchFor, " ")) == 1 {
		// If just first\last name use the WRation which knows how to handle partial
		fuzzyFunc = fuzzy.WRatio
	}

	findings, err := fuzzy.Extract(searchFor, searchedValues, FuzzyLimit, FuzzyMinimumScore, fuzzyFunc)
	if err != nil {
		return nil, fmt.Errorf("search function: %w", err)
	}

	foundUsers := make([]*MatchUser, 0)
	foundUsersMap := make(map[string]*MatchUser)
	for _, finding := range findings {
		user, ok := searchedValueToUser[finding.Match]
		if !ok {
			return nil, fmt.Errorf("mapping finding value back to user object. Got value %q from fuzzy search but didn't find it's belonging user", finding.Match)
		}

		foundUser, ok := foundUsersMap[user.ID]
		if !ok {
			foundUser = &MatchUser{User: user, matchScore: finding.Score}
			foundUsers = append(foundUsers, foundUser)
			foundUsersMap[user.ID] = foundUser
		}
		if finding.Score > foundUser.matchScore {
			foundUser.matchScore = finding.Score
		}
	}

	return foundUsers, nil
}

func (s *SlackStorage) slackUserToUser(user slack.User) *userDomain.User {
	return &userDomain.User{
		ID:                 user.ID,
		FullName:           user.Profile.RealNameNormalized,
		Email:              user.Profile.Email,
		Phone:              user.Profile.Phone,
		PaymentPreferences: nil,
		TransportID:        user.ID,
		Timezone:           user.TZ,
	}
}

func (s *SlackStorage) filterForSpecificName(users []slack.User, name string, oldFinding *MatchUser) (*MatchUser, error) {
	finalFinding := &MatchUser{matchScore: -1}
	if oldFinding != nil {
		finalFinding = oldFinding
	}

	foundUsers, err := s.findMatchedUsers(name, users)
	if err != nil {
		return nil, fmt.Errorf("find matched users: %w", err)
	}
	for _, matchedUser := range foundUsers {
		if matchedUser.matchScore > finalFinding.matchScore {
			finalFinding = matchedUser
		}
	}

	if finalFinding.matchScore == -1 {
		return nil, nil
	}
	return finalFinding, nil
}

func (s *SlackStorage) ListUsers(ctx context.Context, filter userDomain.ListFilter) ([]*userDomain.User, error) {
	findings := make(map[string]*MatchUser) // Map between filtered name to a map of matched users ID to the user itself
	paginatedUsers := s.client.GetUsersPaginated()

	ret := make([]*userDomain.User, 0)
	if filter.TransportID != "" {
		user, err := s.GetUser(ctx, filter.TransportID)
		if err == nil && user != nil {
			ret = append(ret, user)
		}
	}

	filterByNames := len(filter.Names) > 0

	if filter.TransportID != "" && !filterByNames {
		// If we asked to filter just by TransportID and the names filter is empty, returning here to avoid listing all users
		return ret, nil
	}

	usersToFilter := make([]string, 0, len(filter.Names))

	for _, name := range filter.Names {
		cachedUser := s.getFromCache(name)
		if cachedUser == nil {
			usersToFilter = append(usersToFilter, name)
			continue
		}
		ret = append(ret, cachedUser)
	}

	if len(usersToFilter) == 0 && filterByNames {
		// Everything in cache
		return ret, nil
	}

	var err error
	for {
		paginatedUsers, err = paginatedUsers.Next(ctx)
		if err != nil {
			break
		}

		if !filterByNames {
			// Just list the users
			for _, user := range paginatedUsers.Users {
				ret = append(ret, s.slackUserToUser(user))
			}
			continue
		}

		// Filtering
		for _, name := range usersToFilter {
			currentFinding := findings[name]
			currentFinding, err = s.filterForSpecificName(paginatedUsers.Users, name, currentFinding)
			if err != nil {
				return nil, fmt.Errorf("filter for specific name: %w", err)
			}
			if currentFinding != nil {
				findings[name] = currentFinding
			}
		}
	}

	if err = paginatedUsers.Failure(err); err != nil {
		return nil, fmt.Errorf("get users from slack: %w", err)
	}

	for name, matchedUser := range findings {
		user := s.slackUserToUser(matchedUser.User)
		s.saveCache(name, user)
		ret = append(ret, user)
	}

	return ret, nil
}

func (s *SlackStorage) GetUser(_ context.Context, id string) (*userDomain.User, error) {
	user, err := s.client.GetUserInfo(id)
	if err != nil {
		return nil, fmt.Errorf("get user info: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("got nil user from slack")
	}
	return s.slackUserToUser(*user), nil
}
