package service

import (
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/oriser/bolt/order"
	"github.com/slack-go/slack"
)

var dateOneMonthAgo time.Time
var numberOfDigestRows = uint64(5) // Hard-coded due to slack.SectionBlock limitations
var numberToEmojiMap = map[int]string{
	1: ":one:",
	2: ":two:",
	3: ":three:",
	4: ":four:",
	5: ":five:",
}

func buildTopVenuesMessageBlocks(monthlyTopVenues []order.VenueOrderCount, monthlyTopVenuesTotalCounts []order.VenueOrderCount) ([]slack.Block, error) {
	venueIdToTotalOrderCount := make(map[string]int)
	for _, venue := range monthlyTopVenuesTotalCounts {
		venueIdToTotalOrderCount[venue.VenueId] = venue.OrderCount
	}

	topVenuesHeader := slack.NewSectionBlock(
		nil,
		[]*slack.TextBlockObject{
			slack.NewTextBlockObject("mrkdwn", ":cook: *Top restaurants*", false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*%s/Total*", dateOneMonthAgo.Month().String()), false, false),
		},
		nil,
	)

	topVenuesRows := make([]*slack.TextBlockObject, 0, len(monthlyTopVenues)*2)
	for i, venue := range monthlyTopVenues {
		totalOrderCount, venueExists := venueIdToTotalOrderCount[venue.VenueId]
		if !venueExists {
			return nil, fmt.Errorf("venue %s (%s) is not in monthlyTopVenuesTotalCounts", venue.VenueId, venue.VenueName)
		}

		positionEmoji, emojiExists := numberToEmojiMap[i+1]
		if !emojiExists {
			return nil, fmt.Errorf("unsupported ranking %d", i+1)
		}

		venueHyperlink := fmt.Sprintf("<%s|%s>", venue.VenueLink, venue.VenueName)
		leftColumnString := fmt.Sprintf("%s %s%s%s", positionEmoji, UnicodeLeftToRightMark, venueHyperlink, UnicodeLeftToRightMark)
		if venue.OrderCount == totalOrderCount {
			leftColumnString += " :new:"
		}

		rightColumnString := fmt.Sprintf("%d/%d", venue.OrderCount, totalOrderCount)

		topVenuesRows = append(topVenuesRows,
			slack.NewTextBlockObject("mrkdwn", leftColumnString, false, false),
			slack.NewTextBlockObject("mrkdwn", rightColumnString, false, false),
		)
	}

	topVenuesBlocks := append(
		[]slack.Block{topVenuesHeader},
		slack.NewSectionBlock(nil, topVenuesRows, nil),
	)

	return topVenuesBlocks, nil
}

func buildTopHostsMessageBlocks(monthlyTopHosts []order.HostOrderCount, monthlyTopHostsTotalCounts []order.HostOrderCount) ([]slack.Block, error) {
	hostIdToTotalOrderCount := make(map[string]int)
	for _, host := range monthlyTopHostsTotalCounts {
		hostIdToTotalOrderCount[host.HostId] = host.OrderCount
	}

	topHostsHeader := slack.NewSectionBlock(
		nil,
		[]*slack.TextBlockObject{
			slack.NewTextBlockObject("mrkdwn", ":star: *Top hosts*", false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*%s/Total*", dateOneMonthAgo.Month().String()), false, false),
		},
		nil,
	)

	topHostsRows := make([]*slack.TextBlockObject, 0, len(monthlyTopHosts)*2)
	for i, host := range monthlyTopHosts {
		totalOrderCount, hostExists := hostIdToTotalOrderCount[host.HostId]
		if !hostExists {
			return nil, fmt.Errorf("host %s (%s) is not in monthlyTopHostsTotalCounts", host.HostId, host.HostName)
		}

		positionEmoji, emojiExists := numberToEmojiMap[i+1]
		if !emojiExists {
			return nil, fmt.Errorf("unsupported ranking %d", i+1)
		}

		leftColumnString := fmt.Sprintf("%s %s%s%s", positionEmoji, UnicodeLeftToRightMark, host.HostName, UnicodeLeftToRightMark)
		if host.OrderCount == totalOrderCount {
			leftColumnString += " :new:"
		}

		rightColumnString := fmt.Sprintf("%d/%d", host.OrderCount, totalOrderCount)

		topHostsRows = append(topHostsRows,
			slack.NewTextBlockObject("mrkdwn", leftColumnString, false, false),
			slack.NewTextBlockObject("mrkdwn", rightColumnString, false, false),
		)
	}

	topHostsBlocks := append(
		[]slack.Block{topHostsHeader},
		slack.NewSectionBlock(nil, topHostsRows, nil),
	)

	return topHostsBlocks, nil
}

func venueOrderCountsToVenueIds(venueOrderCounts []order.VenueOrderCount) []string {
	var venueIds []string
	for _, venueOrderCount := range venueOrderCounts {
		venueIds = append(venueIds, venueOrderCount.VenueId)
	}
	return venueIds
}

func hostOrderCountsToHostIds(hostOrderCounts []order.HostOrderCount) []string {
	var hostIds []string
	for _, hostOrderCount := range hostOrderCounts {
		hostIds = append(hostIds, hostOrderCount.HostId)
	}
	return hostIds
}

func (h *Service) getTopVenuesMessageBlocks(channelId string) ([]slack.Block, error) {
	monthlyTopVenues, err := h.orderStore.GetVenuesWithMostOrders(dateOneMonthAgo, numberOfDigestRows, channelId, []string{})
	if err != nil {
		return nil, fmt.Errorf("error getting top venues of the last month: %w", err)
	}

	monthlyTopVenueIds := venueOrderCountsToVenueIds(monthlyTopVenues)
	monthlyTopVenuesTotalCounts, err := h.orderStore.GetVenuesWithMostOrders(time.Time{}, numberOfDigestRows, channelId, monthlyTopVenueIds)
	if err != nil {
		return nil, fmt.Errorf("error getting top venues of all time: %w", err)
	}

	return buildTopVenuesMessageBlocks(monthlyTopVenues, monthlyTopVenuesTotalCounts)
}

func (h *Service) getTopHostsMessageBlocks(channelId string) ([]slack.Block, error) {
	monthlyTopHosts, err := h.orderStore.GetHostsWithMostOrders(dateOneMonthAgo, numberOfDigestRows, channelId, []string{})
	if err != nil {
		return nil, fmt.Errorf("error getting top hosts of the last month: %w", err)
	}

	monthlyTopHostIds := hostOrderCountsToHostIds(monthlyTopHosts)
	monthlyTopHostsTotalCounts, err := h.orderStore.GetHostsWithMostOrders(time.Time{}, numberOfDigestRows, channelId, monthlyTopHostIds)
	if err != nil {
		return nil, fmt.Errorf("error getting top hosts of all time: %w", err)
	}

	return buildTopHostsMessageBlocks(monthlyTopHosts, monthlyTopHostsTotalCounts)
}

func (h *Service) sendMonthlyDigestForChannel(channelId string) {
	log.Printf("Sending monthly digest for channel %s\n", channelId)

	titleHeader := slack.NewHeaderBlock(
		&slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: fmt.Sprintf("Welcome to Bolt's %s %d digest", dateOneMonthAgo.Month().String(), dateOneMonthAgo.Year()),
		},
	)

	topVenuesMessageBlocks, err := h.getTopVenuesMessageBlocks(channelId)
	if err != nil {
		log.Printf("Error getting top venues message blocks: %v", err)
		return
	}

	topHostsMessageBlocks, err := h.getTopHostsMessageBlocks(channelId)
	if err != nil {
		log.Printf("Error getting top hosts message blocks: %v", err)
		return
	}

	digestBlocks := slices.Concat(
		[]slack.Block{titleHeader},
		[]slack.Block{slack.NewDividerBlock()},
		topVenuesMessageBlocks,
		[]slack.Block{slack.NewDividerBlock()},
		topHostsMessageBlocks,
	)

	_, err = h.eventNotification.SendBlocksMessage(channelId, digestBlocks, "")
	if err != nil {
		log.Printf("Error sending monthly digest message for for channel %s: %v", channelId, err)
		return
	}
}

func (h *Service) SendMonthlyDigest() {
	dateOneMonthAgo = time.Now().AddDate(0, -1, 0)

	activeChannelIds, err := h.orderStore.GetActiveChannelIds(dateOneMonthAgo)
	if err != nil {
		log.Printf("Error getting active channel IDs: %v", err)
		return
	}

	for _, channelId := range activeChannelIds {
		h.sendMonthlyDigestForChannel(channelId)
	}
}
