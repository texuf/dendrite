// Copyright 2022 The Matrix.org Foundation C.I.C.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deltas

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/matrix-org/dendrite/internal/sqlutil"
	"github.com/matrix-org/gomatrixserverlib"
)

func LoadAddHistoryVisibilityColumn(m *sqlutil.Migrations) {
	m.AddMigration(UpAddHistoryVisibilityColumn, DownAddHistoryVisibilityColumn)
}

func UpAddHistoryVisibilityColumn(tx *sql.Tx) error {
	// SQLite doesn't have "if exists", so check if the column exists. If the query doesn't return an error, it already exists.
	// Required for unit tests, as otherwise a duplicate column error will show up.
	_, err := tx.Query("SELECT history_visibility FROM syncapi_output_room_events LIMIT 1")
	if err == nil {
		return nil
	}
	_, err = tx.Exec(`
		ALTER TABLE syncapi_output_room_events ADD COLUMN history_visibility SMALLINT NOT NULL DEFAULT 2;
		UPDATE syncapi_output_room_events SET history_visibility = 4 WHERE type IN ('m.room.message', 'm.room.encrypted');
	`)
	if err != nil {
		return fmt.Errorf("failed to execute upgrade: %w", err)
	}

	_, err = tx.Query("SELECT history_visibility FROM syncapi_current_room_state LIMIT 1")
	if err == nil {
		return nil
	}
	_, err = tx.Exec(`
		ALTER TABLE syncapi_current_room_state ADD COLUMN history_visibility SMALLINT NOT NULL DEFAULT 2;
		UPDATE syncapi_current_room_state SET history_visibility = 4 WHERE type IN ('m.room.message', 'm.room.encrypted');
	`)
	if err != nil {
		return fmt.Errorf("failed to execute upgrade: %w", err)
	}

	// get the current room history visibility
	rows, err := tx.Query(`SELECT DISTINCT room_id, headered_event_json FROM syncapi_current_room_state
		WHERE type = 'm.room.history_visibility' AND state_key = '';
`)
	if err != nil {
		return fmt.Errorf("failed to query current room state: %w", err)
	}
	var eventBytes []byte
	var roomID string
	var event gomatrixserverlib.HeaderedEvent
	var hisVis gomatrixserverlib.HistoryVisibility
	historyVisibilities := make(map[string]gomatrixserverlib.HistoryVisibility)
	for rows.Next() {
		if err = rows.Scan(&roomID, &eventBytes); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		if err = json.Unmarshal(eventBytes, &event); err != nil {
			return fmt.Errorf("failed to unmarshal event: %w", err)
		}
		historyVisibilities[roomID] = gomatrixserverlib.HistoryVisibilityShared
		if hisVis, err = event.HistoryVisibility(); err == nil {
			historyVisibilities[roomID] = hisVis
		}
	}

	// update the history visibility
	for roomID, hisVis = range historyVisibilities {
		_, err = tx.Exec(`UPDATE syncapi_output_room_events SET history_visibility = $1 
                        WHERE type IN ('m.room.message', 'm.room.encrypted') AND room_id = $2`, hisVis, roomID)
		if err != nil {
			return fmt.Errorf("failed to update history visibility: %w", err)
		}
	}

	return nil
}

func DownAddHistoryVisibilityColumn(tx *sql.Tx) error {
	// SQLite doesn't have "if exists", so check if the column exists.
	_, err := tx.Query("SELECT history_visibility FROM syncapi_output_room_events LIMIT 1")
	if err != nil {
		// The column probably doesn't exist
		return nil
	}
	_, err = tx.Exec(`
		ALTER TABLE syncapi_output_room_events DROP COLUMN history_visibility;
	`)
	if err != nil {
		return fmt.Errorf("failed to execute downgrade: %w", err)
	}
	_, err = tx.Query("SELECT history_visibility FROM syncapi_current_room_state LIMIT 1")
	if err != nil {
		// The column probably doesn't exist
		return nil
	}
	_, err = tx.Exec(`
		ALTER TABLE syncapi_current_room_state DROP COLUMN history_visibility;
	`)
	if err != nil {
		return fmt.Errorf("failed to execute downgrade: %w", err)
	}
	return nil
}
