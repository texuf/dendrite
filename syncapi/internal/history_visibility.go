package internal

import (
	"context"

	"github.com/matrix-org/dendrite/syncapi/storage"
	"github.com/sirupsen/logrus"
)

// EventVisibility contains the history visibility and membership state
type EventVisibility struct {
	Visibility         string
	Membership         string
	MembershipPosition int // the topological position of the membership event
	HistoryPosition    int // the topological position of the history event
}

// Visibility is a map from event_id to EvVis, which contains the history visibility and membership for a given user.
type Visibility map[string]EventVisibility

func (v Visibility) allowed(eventID string) bool {
	ev, ok := v[eventID]
	if !ok {
		return false
	}
	if ev.Visibility == "world_readable" {
		return true
	}
	if ev.Membership == "join" {
		return true
	}
	// If history_visibility was set to shared, and the user joined the room at any point after the event was sent, allow.
	if ev.Visibility == "shared" && ev.MembershipPosition > ev.HistoryPosition {
		return true
	}
	//  If the user’s membership was invite, and the history_visibility was set to invited, allow.
	if ev.Membership == "invite" && ev.Visibility == "invited" {
		return true
	}
	return false
}

func RoomVisibilities(ctx context.Context, syncDB storage.Database, roomID, userID, eventID string) bool {

	eventTopologyPos, err := syncDB.EventPositionInTopology(ctx, eventID)
	if err != nil {
		logrus.WithError(err).Error("unable to fetch event topology position")
		return false
	}

	var (
		hisVis     string
		membership string
	)
	// next, get the matching history visibility event
	historyEvent, historyTopologyToken, err := syncDB.SelectTopologicalEvent(ctx, int(eventTopologyPos.Depth), "m.room.history_visibility", roomID, nil)
	if err == nil {
		hisVis, err = historyEvent.HistoryVisibility()
		if err == nil && hisVis == "world_readable" {
			return true
		}
	} else {
		logrus.WithError(err).Warn("unable to fetch history visibility, defaulting to 'shared'")
		historyTopologyToken.Depth = 0
		hisVis = "shared"
	}

	// finally the membership event, if any
	memberEvent, memberTopologyToken, err := syncDB.SelectTopologicalEvent(ctx, int(eventTopologyPos.Depth), "m.room.member", roomID, &userID)
	if err == nil {
		membership, err = memberEvent.Membership()
		if err == nil {
			v := EventVisibility{
				HistoryPosition:    int(historyTopologyToken.Depth),
				MembershipPosition: int(memberTopologyToken.Depth),
				Membership:         membership,
				Visibility:         hisVis,
			}
			allowedToSee := v.allowedToSeeEvent()
			logrus.Debugf("Allowed to see event: %v", allowedToSee)
			return allowedToSee
		}
	} else {
		logrus.WithError(err).Errorf("unable to fetch room membership for: %s", userID)
		return false
	}

	return false
}

func (v EventVisibility) allowedToSeeEvent() bool {
	// If the history_visibility was set to world_readable, allow.
	if v.Visibility == "world_readable" {
		logrus.Debugf("returning true, world readable")
		return true
	}
	// If the user’s membership was join, allow.
	if v.Membership == "join" {
		logrus.Debugf("returning true, member is joined")
		return true
	}

	// If history_visibility was set to shared, and the user joined the room at any point after the event was sent, allow.
	if v.Visibility == "shared" && v.MembershipPosition > v.HistoryPosition {
		logrus.Debugf("returning true, room is shared, user joined afterwards")
		return true
	}
	//  If the user’s membership was invite, and the history_visibility was set to invited, allow.
	if v.Membership == "invite" && v.Visibility == "invited" {
		logrus.Debugf("returning true, member was invited and visiblity invited")
		return true
	}

	// Otherwise, deny.
	return false
}
