package testing

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/oriser/bolt/cmd/run"
	"github.com/oriser/bolt/service"
	"github.com/oriser/bolt/testing/customslack"
	"github.com/oriser/bolt/testing/utils"
	"github.com/oriser/bolt/testing/woltserver"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/slacktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	DefaultOrderLocation = woltserver.Coordinate{
		Lat: 32.0707244997673,
		Lon: 34.78343904018402,
	}
	DefaultVenueLocation = woltserver.Coordinate{
		Lat: 32.072447331148844,
		Lon: 34.77900266647339,
	}
)

const (
	WaitForMessageTimeout  = 20 * time.Second
	OrderReadyTimeout      = 10 * time.Second
	WaitBetweenStatusCheck = 500 * time.Millisecond
	DebtReminderInterval   = 3 * time.Second
	DebtMaximumDuration    = 10 * time.Second
	AdminSlackUserID       = "ABC123"
	MaxHttpAttempts        = 10000 // A lot of attempts to make sure the request will succeed at last (we return 502 randomly for tests)
	MinHttpRetryWait       = time.Millisecond
	MaxHttpRetryWait       = 5 * time.Millisecond

	DefaultNonBotUserID     = "W012A3CDE" // From slack test package, it's not exposed, and it's constant
	MessageChannel          = "some-channel"
	DefaultExpectedDelivery = 10
	DefaultHost             = "Bolt"
)

const (
	HelloPattern = `Hey :) Just letting you know I joined the group %s`
)

var timezones = []string{
	"Europe/London",       // +0
	"Asia/Almaty",         // +6
	"Pacific/Auckland",    // +12
	"America/Mexico_City", // -6
	"Pacific/Pago_Pago",   // -11
}

type testData struct {
	woltServer  *woltserver.WoltServer
	slackServer *slacktest.Server
	customSlack *customslack.Handlers
	boltAddr    string
}

func initEnvs(t *testing.T, tdata testData) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "bolttest_*")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(tmpDir))
	})

	// Bot
	require.NoError(t, os.Setenv("SLACK_SIGNIN_SECRET", "ignored"))
	require.NoError(t, os.Setenv("SLACK_OAUTH_TOKEN", "ignored"))
	require.NoError(t, os.Setenv("SLACK_API_URL", tdata.slackServer.GetAPIURL()))
	require.NoError(t, os.Setenv("ADMIN_SLACK_USER_IDS", AdminSlackUserID))
	require.NoError(t, os.Setenv("DISABLE_SECRET_VERIFICATION", "true"))

	// Service
	require.NoError(t, os.Setenv("ORDER_READY_TIMEOUT", OrderReadyTimeout.String()))
	require.NoError(t, os.Setenv("WAIT_BETWEEN_STATUS_CHECK", WaitBetweenStatusCheck.String()))
	require.NoError(t, os.Setenv("DEBT_REMINDER_INTERVAL", DebtReminderInterval.String()))
	require.NoError(t, os.Setenv("DEBT_MAXIMUM_DURATION", DebtMaximumDuration.String()))
	require.NoError(t, os.Setenv("WOLT_BASE_ADDR", "http://"+tdata.woltServer.Addr()))
	require.NoError(t, os.Setenv("WOLT_API_BASE_ADDR", "http://"+tdata.woltServer.Addr()))
	require.NoError(t, os.Setenv("WOLT_HTTP_MAX_RETRY_COUNT", strconv.Itoa(MaxHttpAttempts)))
	require.NoError(t, os.Setenv("WOLT_HTTP_MIN_RETRY_DURATION", MinHttpRetryWait.String()))
	require.NoError(t, os.Setenv("WOLT_HTTP_MAX_RETRY_DURATION", MaxHttpRetryWait.String()))

	// main
	require.NoError(t, os.Setenv("DB_LOCATION", path.Join(tmpDir, "db.sqlite")))
}

func initTest(t *testing.T) testData {
	t.Helper()
	woltServer := woltserver.NewWoltServer(t)
	t.Log("Starting test wolt server")
	woltServer.Start()

	customHandlers := customslack.NewHandlers()
	slackServer := slacktest.NewTestServer(func(customize slacktest.Customize) {
		customHandlers.Register(customize)
	})
	t.Log("Starting test slack server")
	slackServer.Start()

	t.Cleanup(func() {
		t.Log("Stopping test wolt server")
		woltServer.Stop()
		t.Log("Stopping test slack server")
		slackServer.Stop()
	})

	tdata := testData{
		woltServer:  woltServer,
		slackServer: slackServer,
		boltAddr:    ":8080",
		customSlack: customHandlers,
	}

	initEnvs(t, tdata)

	errCh := make(chan error, 1)
	go func() {
		t.Log("Running bolt")
		err := run.Run()
		if err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(1 * time.Second):
	}

	return tdata
}

func isOutsideWorkingHours(t *testing.T, tzString string) bool {
	t.Helper()
	currentTime := time.Now()
	tz, err := time.LoadLocation(tzString)
	require.NoError(t, err)
	currentTime = currentTime.In(tz)
	return currentTime.Hour() >= service.NoMessagesAfterHour || currentTime.Hour() < service.NoMessagesBeforeHour
}

func findTimezone(t *testing.T, negate bool) string {
	t.Helper()

	for _, tzString := range timezones {
		if isOutsideWorkingHours(t, tzString) == negate {
			return tzString
		}
	}
	t.Error("No matching timezone found")
	t.FailNow()
	return ""
}

// findValidTimezone finds a timezone in the debts sending hours
func findValidTimezone(t *testing.T) string {
	t.Helper()
	return findTimezone(t, false)
}

// findInvalidTimezone finds a timezone after the debts sending hours
func findInvalidTimezone(t *testing.T) string {
	t.Helper()
	return findTimezone(t, true)
}

type minimalLinkEvent struct {
	Links []sharedLinks `json:"links"`
}
type sharedLinks struct {
	Domain string `json:"domain"`
	URL    string `json:"url"`
}

// setLinksToEvent is for overcoming the problem that sharedLinks is not exported in slackevents
// So we do an ugly trick of creating the same JSON struct then unmarshalling it to the slackevents structure and assigning
// again to the original event
func setLinksToEvent(t *testing.T, links []sharedLinks, event *slackevents.LinkSharedEvent) {
	t.Helper()

	dummyEvent := minimalLinkEvent{Links: links}
	marshaled, err := json.Marshal(dummyEvent)
	require.NoError(t, err)

	var res slackevents.LinkSharedEvent
	require.NoError(t, json.Unmarshal(marshaled, &res))
	event.Links = res.Links
}

func buildGenericSlackEvent(t *testing.T, eventData *json.RawMessage) []byte {
	t.Helper()

	e := slackevents.EventsAPICallbackEvent{
		Type:       "event_callback",
		Token:      "ignored",
		TeamID:     "ignored",
		APIAppID:   "ignored",
		InnerEvent: eventData,
	}

	marshaled, err := json.Marshal(e)
	require.NoError(t, err)
	return marshaled
}

func buildSlackReactionEvent(t *testing.T, itemUser, timestamp, reaction string, fromUser string) []byte {
	reactionEvent := &slackevents.ReactionAddedEvent{
		Type:           "reaction_added",
		User:           fromUser,
		Reaction:       reaction,
		ItemUser:       itemUser,
		Item:           slackevents.Item{Channel: MessageChannel, Timestamp: timestamp},
		EventTimestamp: "ignored",
	}

	marshaled, err := json.Marshal(reactionEvent)
	require.NoError(t, err)
	rawEvent := json.RawMessage(marshaled)

	return buildGenericSlackEvent(t, &rawEvent)
}

func buildSlackLinkEvent(t *testing.T, messageTimestamp, groupShortID string) []byte {
	t.Helper()

	linkEvent := &slackevents.LinkSharedEvent{
		Type:             "link_shared",
		User:             "not-important",
		TimeStamp:        "ignored",
		Channel:          MessageChannel,
		MessageTimeStamp: messageTimestamp,
		ThreadTimeStamp:  "ignored",
	}
	setLinksToEvent(t, []sharedLinks{
		{
			Domain: "wolt.com",
			URL:    fmt.Sprintf("https://wolt.com/group/%s", groupShortID),
		},
	}, linkEvent)

	marshaled, err := json.Marshal(linkEvent)
	require.NoError(t, err)
	rawEvent := json.RawMessage(marshaled)

	return buildGenericSlackEvent(t, &rawEvent)
}

func orderedRates(totalPerParticipant map[string]float64) []service.Rate {
	keys := make([]string, 0, len(totalPerParticipant))
	for key := range totalPerParticipant {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	rates := make([]service.Rate, len(totalPerParticipant))
	for i, key := range keys {
		rates[i] = service.Rate{
			WoltName: key,
			Amount:   totalPerParticipant[key],
		}
	}
	return rates
}

func buildRatesMessage(t *testing.T, order *woltserver.Order, expectedDelivery int, participantIDsMapping map[string]string) (rates []service.Rate, ratesMessage string) {
	t.Helper()

	totalPerParticipant := make(map[string]float64)
	for _, participant := range order.Participants {
		total := 0
		for _, item := range participant.Items {
			total += item.EndAmount
		}
		if total == 0 {
			continue
		}
		totalPerParticipant[participant.FirstName] = float64(total)
	}
	deliveryPerParticiapnt := float64(expectedDelivery) / float64(len(totalPerParticipant))
	if _, ok := totalPerParticipant[order.Host]; !ok {
		// Currently, host will be in the list even if they didn't order anything
		totalPerParticipant[order.Host] = 0
	}

	var ratesStringBuilder strings.Builder
	ratesStringBuilder.WriteString(fmt.Sprintf("Rates for Wolt order ID %s (including %d NIS for delivery):\n", order.ShortID, expectedDelivery))

	rates = orderedRates(totalPerParticipant)
	for i, rate := range rates {
		if rate.Amount != 0 {
			rate.Amount += deliveryPerParticiapnt
			rates[i] = rate
		}

		name := rate.WoltName
		if id, ok := participantIDsMapping[rate.WoltName]; ok {
			name = fmt.Sprintf("<@%s> (%s)", id, rate.WoltName)
		}
		ratesStringBuilder.WriteString(fmt.Sprintf("%s: %.2f\n", name, rate.Amount))
	}

	host := order.Host
	if id, ok := participantIDsMapping[order.Host]; ok {
		host = fmt.Sprintf("<@%s>", id)
	}
	ratesStringBuilder.WriteString(fmt.Sprintf("\nPay to: %s\n", host))
	return rates, ratesStringBuilder.String()
}

func hasUnexpectedMessages(t *testing.T, slackServer *slacktest.Server) int {
	unexpectedCount := 0
	for _, msg := range slackServer.GetSeenOutboundMessages() {
		if _, ok := seenMessages.Load(msg); ok {
			continue
		}

		unexpectedCount++
		m := slack.Message{}
		err := json.Unmarshal([]byte(msg), &m)
		require.NoError(t, err)
		t.Logf("Unexpected Slack outbound message #%d, channel %q; timestamp %q:\n------start------\n%s\n------end------", unexpectedCount, m.Channel, m.ThreadTimestamp, m.Text)
	}

	return unexpectedCount
}

// validateDebts validates debts messages are being sent as expected
func validateDebts(t *testing.T,
	tdata testData,
	host, orderID, timestamp string,
	participantIDsMapping map[string]string,
	slackUsers map[string]customslack.SlackUser,
	rates []service.Rate) {
	ratesMap := make(map[string]float64)
	for _, rate := range rates {
		ratesMap[rate.WoltName] = rate.Amount
	}
	t.Log("Waiting for a debt cycle")
	time.Sleep(DebtReminderInterval)
	willRemainDebts := make([]string, 0)

	for participant, slackUser := range slackUsers {
		if slackUser.Deleted {
			// Deleted account will not get a message
			continue
		}
		if isOutsideWorkingHours(t, slackUser.Timezone) {
			// If the user is not currently in debts sending timezone, it won't get a message and its debts will remain after the debts timeout
			willRemainDebts = append(willRemainDebts, participant)
			continue
		}
		_, err := WaitForOutboundSlackMessage(WaitForMessageTimeout, tdata.slackServer,
			fmt.Sprintf("Reminder, you should pay %.2f nis to <@%s> for Wolt order ID %s.\n",
				ratesMap[participant], participantIDsMapping[host], orderID),
			participantIDsMapping[participant], "", ContainsMatch)
		require.NoErrorf(t, err, "Could not find debt message for participant %q", participant)

		// Marking user as paid
		evt := buildSlackReactionEvent(t, DefaultNonBotUserID, timestamp, "money_mouth_face", participantIDsMapping[participant])
		resp, err := http.Post("http://"+tdata.boltAddr+"/events-endpoint", "application/json", bytes.NewReader(evt))
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		// Checking messages sent to user and host
		_, err = WaitForOutboundSlackMessage(WaitForMessageTimeout, tdata.slackServer,
			fmt.Sprintf("OK! I removed your debt for order %s", orderID),
			participantIDsMapping[participant], "", EqualMatch)
		require.NoError(t, err)

		_, err = WaitForOutboundSlackMessage(WaitForMessageTimeout, tdata.slackServer,
			fmt.Sprintf("<@%s> marked himself as paid for order ID %s", participantIDsMapping[participant], orderID),
			participantIDsMapping[host], "", EqualMatch)
		require.NoError(t, err)
	}

	// To make sure debts won't be sent to paid users anymore
	t.Log("Waiting for one more debt cycle")
	time.Sleep(DebtReminderInterval)

	// Sleeping for the remaining time and validates the host get the timeout message
	t.Log("Waiting until debt timeout will reach")
	time.Sleep(DebtMaximumDuration - 2*DebtReminderInterval)
	if len(willRemainDebts) > 0 {
		_, err := WaitForOutboundSlackMessage(WaitForMessageTimeout, tdata.slackServer,
			fmt.Sprintf("I removed all debts for order ID %s because timeout has been reached", orderID),
			participantIDsMapping[host], "", EqualMatch)
		require.NoError(t, err)
	}
}

func cancelDebts(t *testing.T,
	tdata testData,
	hostUser, orderID, timestamp string) {
	t.Helper()

	// Sending cancel debts
	evt := buildSlackReactionEvent(t, DefaultNonBotUserID, timestamp, "x", hostUser)
	resp, err := http.Post("http://"+tdata.boltAddr+"/events-endpoint", "application/json", bytes.NewReader(evt))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	_, err = WaitForOutboundSlackMessage(WaitForMessageTimeout, tdata.slackServer,
		fmt.Sprintf("I removed all debts for order ID %s because the host requested to cancel debts tracking", orderID),
		hostUser, "", EqualMatch)
	require.NoError(t, err)

	// Validating no one will get debt message
	t.Log("Waiting for a debt cycle")
	time.Sleep(DebtReminderInterval)
}

func buildAddUserSlashCommand(t *testing.T, sentUser, addedUserName string, addedUserSlackName string) string {
	t.Helper()

	data := url.Values{}
	data.Set("user_id", sentUser)
	data.Set("command", "/add-user")
	data.Set("text", fmt.Sprintf("%q @%s", addedUserName, addedUserSlackName))

	return data.Encode()
}

func sendAddUserSlashCommand(t *testing.T, tdata testData, sentUser, addedUserName, addedUserSlackName, addedUserSlackID string) {
	t.Helper()

	body := buildAddUserSlashCommand(t, sentUser, addedUserName, addedUserSlackName)
	resp, err := http.Post("http://"+tdata.boltAddr+"/add-user", "application/x-www-form-urlencoded", strings.NewReader(body))
	require.NoError(t, err)
	if sentUser != AdminSlackUserID {
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		return
	}
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, fmt.Sprintf("OK, got you. I added <@%s> as %q", addedUserSlackID, addedUserName), string(respBody))
}

func TestSlackPurchaseGroup(t *testing.T) {
	tdata := initTest(t)
	t.Cleanup(func() {
		unexpectedMessage := hasUnexpectedMessages(t, tdata.slackServer)
		assert.Zero(t, unexpectedMessage, "got some unexpected slack messages")
	})

	tests := []struct {
		name                     string
		host                     string
		participants             map[string][]int // Participant name to number of items ordered
		venueLocation            woltserver.Coordinate
		orderLocation            woltserver.Coordinate
		expectedDelivery         int
		participantsToAddToSlack map[string]customslack.SlackUser // Map of participant name to its name in slack to add
		customUsersToAddToSlack  []customslack.SlackUser          // Additional non-participants users to add
		addUserSlashCommand      map[string]string                // Map between wolt username to slack's username to add as a custom name via add-user slash command
		slashCommandSender       string
		addHostToSlack           bool
		cancelDebts              bool
	}{
		{
			name: "Simple no participants",
		},
		{
			name:         "More participants",
			participants: map[string][]int{"Loki": {20}, "Freya": {5, 10, 13, 40}, "Sigurd Hring": {22, 71}, "Björn Ironside": {10, 14}},
		},
		{
			name:         "With longer distance",
			participants: map[string][]int{"Loki": {20}, "Freya": {5, 10, 13, 40}, "Sigurd Hring": {22, 71}, "Björn Ironside": {10, 14}},
			venueLocation: woltserver.Coordinate{
				Lat: 32.071518807218276,
				Lon: 34.771948965165144,
			},
			expectedDelivery: 12,
		},
		{
			name:         "Further distance",
			participants: map[string][]int{"Loki": {}, "Freya": {5, 10, 13, 40}, "Sigurd Hring": {88}, "Björn Ironside": {10, 105}},
			venueLocation: woltserver.Coordinate{
				Lat: 32.11401145876586,
				Lon: 34.78751129658299,
			},
			expectedDelivery: 18,
		},
		{
			name:         "User exist in slack, host not",
			participants: map[string][]int{"Loki": {}, "Erika": {13, 40}, "Sigurd Hring": {88}, "Björn Ironside": {10, 105}},
			participantsToAddToSlack: map[string]customslack.SlackUser{
				"Erika": {Name: "Erika"},
			},
		},
		{
			name:         "Multiple users in slack, host not, not exact name",
			participants: map[string][]int{"Arnfinn Hiba": {10}, "Erika": {13, 40}, "Sigurd Hring": {88}, "Björn Ironside": {10, 105}},
			participantsToAddToSlack: map[string]customslack.SlackUser{
				"Erika":        {Name: "Erika"},
				"Arnfinn Hiba": {Name: "Arnfin Hibe"},
			},
		},
		{
			name:           "Add host",
			participants:   map[string][]int{"Loki": {20}, "Freya": {5, 10, 13, 40}, "Sigurd Hring": {22, 71}, "Björn Ironside": {10, 14}},
			addHostToSlack: true,
			host:           "Ori",
		},
		{
			name:         "Add host and others",
			participants: map[string][]int{"Olaf": {10}, "Eysteinn": {13, 40}, "Njord": {88}, "Björn Ironside": {10, 105}},
			participantsToAddToSlack: map[string]customslack.SlackUser{
				"Eysteinn": {Name: "Eysteinn", Timezone: findValidTimezone(t)},
				"Njord":    {Name: "Njord", Timezone: findInvalidTimezone(t)},
			},
			addHostToSlack: true,
			host:           "Ori",
		},
		{
			name:         "All users found and will mark themselves as paid",
			participants: map[string][]int{"Idunn": {10}, "Búri": {13, 40}},
			participantsToAddToSlack: map[string]customslack.SlackUser{
				"Búri":  {Name: "Búri", Timezone: findValidTimezone(t)},
				"Idunn": {Name: "Idunn", Timezone: findValidTimezone(t)},
			},
			addHostToSlack: true,
			host:           "Ori",
		},
		{
			name:         "Users exists in slack, one is deleted",
			participants: map[string][]int{"Biga": {10}, "Nori": {13, 40}},
			participantsToAddToSlack: map[string]customslack.SlackUser{
				"Biga": {Name: "Biga", Timezone: findValidTimezone(t), Deleted: true},
				"Nori": {Name: "Nori", Timezone: findValidTimezone(t)},
			},
			addHostToSlack: true,
			host:           "Ori",
		},
		{
			name:         "Cancel debts",
			participants: map[string][]int{"Idunn": {10}, "Búri": {13, 40}},
			participantsToAddToSlack: map[string]customslack.SlackUser{
				"Búri":  {Name: "Búri", Timezone: findValidTimezone(t)},
				"Idunn": {Name: "Idunn", Timezone: findValidTimezone(t)},
			},
			addHostToSlack: true,
			host:           "Ori",
			cancelDebts:    true,
		},
		{
			name:         "Add user with slash command",
			participants: map[string][]int{"Hodur": {10}},
			customUsersToAddToSlack: []customslack.SlackUser{
				{
					Name: "Different",
				},
			},
			addUserSlashCommand: map[string]string{
				"Hodur": "Different",
			},
		},
		{
			name:         "User exist both in Slack and via slash command, take the slash command",
			participants: map[string][]int{"Holda": {10}},
			customUsersToAddToSlack: []customslack.SlackUser{
				{
					Name: "AnotherOne",
				},
			},
			addUserSlashCommand: map[string]string{
				"Holda": "AnotherOne",
			},
			participantsToAddToSlack: map[string]customslack.SlackUser{
				"Holda": {Name: "Holda"},
			},
		},
		{
			name:         "Unauthorized slash command sender, take from Slack",
			participants: map[string][]int{"Arngrim": {10}},
			customUsersToAddToSlack: []customslack.SlackUser{
				{
					Name: "YetAnother",
				},
			},
			addUserSlashCommand: map[string]string{
				"Arngrim": "YetAnother",
			},
			slashCommandSender: "not-the-right-one",
			participantsToAddToSlack: map[string]customslack.SlackUser{
				"Arngrim": {Name: "Arngrim"},
			},
		},
		{
			name:         "Similar names",
			participants: map[string][]int{"Lorem": {10}},
			participantsToAddToSlack: map[string]customslack.SlackUser{
				"Lorem": {Name: "Lorem Ipsum", Timezone: findValidTimezone(t)},
			},
			customUsersToAddToSlack: []customslack.SlackUser{
				{
					Name: "Alorem Bar",
				},
			},
			addHostToSlack: true,
			host:           "Ori",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			orderLocation := DefaultOrderLocation
			venueLocation := DefaultVenueLocation
			expectedDelivery := DefaultExpectedDelivery
			host := DefaultHost

			if tc.orderLocation.Lat != 0 {
				orderLocation = tc.orderLocation
			}
			if tc.venueLocation.Lat != 0 {
				venueLocation = tc.venueLocation
			}
			if tc.expectedDelivery != 0 {
				expectedDelivery = tc.expectedDelivery
			}
			if tc.host != "" {
				host = tc.host
			}

			// Init order, venue and participants
			venueID := tdata.woltServer.CreateVenue(orderLocation)
			orderShortID, orderID := tdata.woltServer.CreateOrder(host, venueID, venueLocation)
			t.Logf("Created order %s to venue %s", orderShortID, venueID)

			for name, items := range tc.participants {
				participantID, err := tdata.woltServer.AddParticipant(orderID, name)
				require.NoError(t, err)
				for _, itemAmount := range items {
					require.NoError(t, tdata.woltServer.AddParticipantItem(orderID, participantID, itemAmount))
				}
			}

			// Sending link event (equal to sending wolt link in a channel)
			timestamp := utils.GenerateRandomString(utils.NumberLetters, 8)
			evt := buildSlackLinkEvent(t, timestamp, orderShortID)
			resp, err := http.Post("http://"+tdata.boltAddr+"/events-endpoint", "application/json", bytes.NewReader(evt))
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			// Verifying joined message
			_, err = WaitForOutboundSlackMessage(WaitForMessageTimeout, tdata.slackServer, fmt.Sprintf(HelloPattern, orderShortID),
				MessageChannel, timestamp, EqualMatch)
			require.NoError(t, err)

			// Adding relevant participants as users in Slack
			participantIDsMapping := make(map[string]string)
			for participantName, slackUser := range tc.participantsToAddToSlack {
				require.Contains(t, tc.participants, participantName, "participant %v in slack mapping doesn't exist in participants", participantName)
				id := tdata.customSlack.AddSlackUser(slackUser)
				// Add the deleted users to Slack, but avoid counting them as expected participants
				if !slackUser.Deleted {
					participantIDsMapping[participantName] = id
				}
			}

			order, err := tdata.woltServer.GetOrder(orderID)
			require.NoError(t, err)

			if tc.addHostToSlack {
				participantIDsMapping[order.Host] = tdata.customSlack.AddSlackUser(customslack.SlackUser{
					Name: order.Host,
				})
			}

			// Adding non-participants users
			customSlackUsersNameToID := make(map[string]string)
			for _, user := range tc.customUsersToAddToSlack {
				customSlackUsersNameToID[user.Name] = tdata.customSlack.AddSlackUser(user)
			}
			// Sending add-user slash commands
			for woltUser, slackUser := range tc.addUserSlashCommand {
				sendAddUserSlashCommand(t, tdata, AdminSlackUserID, woltUser, slackUser, customSlackUsersNameToID[slackUser])
				participantIDsMapping[woltUser] = customSlackUsersNameToID[slackUser]
			}

			// Finishing the order
			require.NoError(t, tdata.woltServer.UpdateOrderStatus(orderID, woltserver.StatusPurchased))

			// Validating the rates message
			rates, ratesMessage := buildRatesMessage(t, order, expectedDelivery, participantIDsMapping)
			msg, err := WaitForOutboundSlackMessage(WaitForMessageTimeout, tdata.slackServer,
				fmt.Sprintf("Rates for Wolt order ID %s", orderShortID),
				MessageChannel, timestamp, ContainsMatch)
			require.NoError(t, err)
			assert.Equal(t, ratesMessage, msg.Text)
			tdata.customSlack.AddConversationReply(MessageChannel, timestamp, *msg)

			err = WaitForOutboundReaction(2*time.Second, tdata.customSlack, customslack.Reaction{
				Name:      "money_mouth_face",
				Channel:   msg.Channel,
				Timestamp: msg.Timestamp,
			})
			assert.NoError(t, err)

			if !tc.addHostToSlack {
				// No debts mode
				_, err = WaitForOutboundSlackMessage(WaitForMessageTimeout, tdata.slackServer,
					fmt.Sprintf("I didn't find the user of the host (%s), I won't track debts for order %s", order.Host, orderShortID),
					MessageChannel, timestamp, EqualMatch)
				assert.NoError(t, err)
			} else {
				// Debt mode
				// First, verifying debts message
				_, err = WaitForOutboundSlackMessage(WaitForMessageTimeout, tdata.slackServer,
					fmt.Sprintf("<@%s>, as the host, you can react with :x: to the rates message to cancel debts tracking for Wolt order ID %s",
						participantIDsMapping[order.Host], orderShortID),
					MessageChannel, timestamp, ContainsMatch)
				assert.NoError(t, err)
				// Then, verifying all "not found users" messages
				for participant := range tc.participants {
					if _, ok := participantIDsMapping[participant]; ok {
						// participant should exist and no "not found" message should be sent
						continue
					}
					_, err = WaitForOutboundSlackMessage(WaitForMessageTimeout, tdata.slackServer,
						fmt.Sprintf("I won't track %q payment because I can't find his user.", participant),
						MessageChannel, timestamp, EqualMatch)
					assert.NoError(t, err)
				}
				if tc.cancelDebts {
					cancelDebts(t, tdata, participantIDsMapping[host], orderShortID, timestamp)
				} else {
					validateDebts(t, tdata, host, orderShortID, timestamp, participantIDsMapping, tc.participantsToAddToSlack, rates)
				}
			}

			// Giving some time for unsent messages to be sent so the verification for messages at the end of the tests will find them
			time.Sleep(50 * time.Millisecond)
		})
	}
}
