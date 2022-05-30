// Copyright 2017 Vector Creations Ltd
// Copyright 2018 New Vector Ltd
// Copyright 2019-2020 The Matrix.org Foundation C.I.C.
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

package state

import (
	"testing"

	"github.com/matrix-org/dendrite/roomserver/types"
)

func TestDifferenceBetweeenStateSnapshots(t *testing.T) {
	oldEntries := []types.StateEntry{
		{
			StateKeyTuple: types.StateKeyTuple{EventTypeNID: 1, EventStateKeyNID: 1},
			EventNID:      1,
		},
		{
			StateKeyTuple: types.StateKeyTuple{EventTypeNID: 1, EventStateKeyNID: 1},
			EventNID:      1,
		},
	}
	newEntries := []types.StateEntry{
		{
			StateKeyTuple: types.StateKeyTuple{EventTypeNID: 1, EventStateKeyNID: 1},
			EventNID:      2,
		},
		{
			StateKeyTuple: types.StateKeyTuple{EventTypeNID: 2, EventStateKeyNID: 2},
			EventNID:      2,
		},
		{
			StateKeyTuple: types.StateKeyTuple{EventTypeNID: 2, EventStateKeyNID: 2},
			EventNID:      2,
		},
	}

	var removed, added []types.StateEntry
	func() {
		var oldI int
		var newI int
		for {
			switch {
			case oldI == len(oldEntries):
				// We've reached the end of the old entries.
				// The rest of the new list must have been newly added.
				added = append(added, newEntries[newI:]...)
				return
			case newI == len(newEntries):
				// We've reached the end of the new entries.
				// The rest of the old list must be have been removed.
				removed = append(removed, oldEntries[oldI:]...)
				return
			case oldEntries[oldI] == newEntries[newI]:
				// The entry is in both lists so skip over it.
				oldI++
				newI++
			case oldEntries[oldI].LessThan(newEntries[newI]):
				// The lists are sorted so the old entry being less than the new entry means that it only appears in the old list.
				removed = append(removed, oldEntries[oldI])
				oldI++
			default:
				// Reaching the default case implies that the new entry is less than the old entry.
				// Since the lists are sorted this means that it only appears in the new list.
				added = append(added, newEntries[newI])
				newI++
			}
		}
	}()
	t.Log("Added:", added)
	t.Log("Removed:", removed)
}

func TestFindDuplicateStateKeys(t *testing.T) {
	testCases := []struct {
		Input []types.StateEntry
		Want  []types.StateEntry
	}{{
		Input: []types.StateEntry{
			{StateKeyTuple: types.StateKeyTuple{EventTypeNID: 1, EventStateKeyNID: 1}, EventNID: 1},
			{StateKeyTuple: types.StateKeyTuple{EventTypeNID: 1, EventStateKeyNID: 1}, EventNID: 2},
			{StateKeyTuple: types.StateKeyTuple{EventTypeNID: 2, EventStateKeyNID: 2}, EventNID: 3},
		},
		Want: []types.StateEntry{
			{StateKeyTuple: types.StateKeyTuple{EventTypeNID: 1, EventStateKeyNID: 1}, EventNID: 1},
			{StateKeyTuple: types.StateKeyTuple{EventTypeNID: 1, EventStateKeyNID: 1}, EventNID: 2},
		},
	}, {
		Input: []types.StateEntry{
			{StateKeyTuple: types.StateKeyTuple{EventTypeNID: 1, EventStateKeyNID: 1}, EventNID: 1},
			{StateKeyTuple: types.StateKeyTuple{EventTypeNID: 1, EventStateKeyNID: 2}, EventNID: 2},
		},
		Want: nil,
	}}

	for _, test := range testCases {
		got := findDuplicateStateKeys(test.Input)
		if len(got) != len(test.Want) {
			t.Fatalf("Wanted %v, got %v", test.Want, got)
		}
		for i := range got {
			if got[i] != test.Want[i] {
				t.Fatalf("Wanted %v, got %v", test.Want, got)
			}
		}
	}
}
