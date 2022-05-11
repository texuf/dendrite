package storage_test

import (
	"context"
	"testing"

	"github.com/matrix-org/dendrite/roomserver/storage"
	"github.com/matrix-org/dendrite/roomserver/types"
	"github.com/matrix-org/dendrite/setup/base"
	"github.com/matrix-org/dendrite/setup/config"
	"github.com/matrix-org/dendrite/test"
	"github.com/stretchr/testify/assert"
)

func mustCreateDatabase(t *testing.T, dbType test.DBType) (storage.Database, func()) {
	connStr, close := test.PrepareDBConnectionString(t, dbType)
	cfg := &config.Dendrite{}
	cfg.Defaults(true)
	b := base.NewBaseDendrite(cfg, "Monolith", base.DisableMetrics)
	db, err := storage.NewRoomserverDatabase(nil, &config.DatabaseOptions{
		ConnectionString: config.DataSource(connStr),
	}, b.Caches)
	if err != nil {
		t.Fatalf("NewSyncServerDatasource returned %s", err)
	}
	return db, close
}

func TestStateBlock(t *testing.T) {
	alice := test.NewUser()
	room := test.NewRoom(t, alice)
	ctx := context.Background()
	test.WithAllDatabases(t, func(t *testing.T, dbType test.DBType) {
		db, close := mustCreateDatabase(t, dbType)
		defer close()
		_ = close

		var latestEvents []types.StateAtEventAndReference
		var stateEntries []types.StateEntry
		var roomNID types.RoomNID
		var stateAtEvent types.StateAtEvent
		var eventNID types.EventNID
		var authEventIDs map[string]types.EventNID
		for _, ev := range room.Events() {
			var err error
			authEventIDs, err = db.EventNIDs(ctx, ev.AuthEventIDs())
			assert.NoError(t, err)
			authEventNIDs := make([]types.EventNID, 0, len(authEventIDs))
			for _, v := range authEventIDs {
				authEventNIDs = append(authEventNIDs, v)
			}
			eventNID, roomNID, stateAtEvent, _, _, err = db.StoreEvent(ctx, ev.Event, authEventNIDs, false)
			assert.NoError(t, err)

			//
			stateEntries = append(stateEntries, stateAtEvent.StateEntry)

			stateSnapshotNID, err := db.AddState(ctx, roomNID, []types.StateBlockNID{}, stateEntries)
			assert.NoError(t, err)

			latestEvents = append(latestEvents, types.StateAtEventAndReference{
				StateAtEvent:   stateAtEvent,
				EventReference: ev.EventReference(),
			})
			info, err := db.RoomInfo(ctx, ev.RoomID())
			assert.NoError(t, err)
			t.Logf("stateAtEvent: %+v", stateAtEvent)

			roomUpdater, err := db.GetRoomUpdater(ctx, info)
			assert.NoError(t, err)

			err = roomUpdater.StorePreviousEvents(eventNID, ev.PrevEvents())
			assert.NoError(t, err)
			err = roomUpdater.SetLatestEvents(roomNID, latestEvents, eventNID, stateSnapshotNID)
			assert.NoError(t, err)
			err = roomUpdater.Commit()
			assert.NoError(t, err)
		}
	})
}
