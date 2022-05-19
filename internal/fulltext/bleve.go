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

package fulltext

import (
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/matrix-org/dendrite/setup/config"
	"github.com/matrix-org/gomatrixserverlib"
)

// Search contains all existing bleve.Index
type Search struct {
	FulltextIndex bleve.Index
}

// IndexElement describes the layout of an element to index
type IndexElement struct {
	EventID        string
	RoomID         string
	Content        string
	ContentType    string
	StreamPosition int64
}

// SetContentType sets i.ContentType given an identifier
func (i *IndexElement) SetContentType(v string) {
	switch v {
	case "m.room.message":
		i.ContentType = "content.body"
	case gomatrixserverlib.MRoomName:
		i.ContentType = "content.name"
	case gomatrixserverlib.MRoomTopic:
		i.ContentType = "content.topic"
	}
}

// New opens a new/existing fulltext index
func New(cfg config.Fulltext) (fts *Search, err error) {
	fts = &Search{}
	fts.FulltextIndex, err = openIndex(cfg)
	if err != nil {
		return nil, err
	}
	return fts, nil
}

// Close closes the fulltext index
func (f *Search) Close() error {
	return f.FulltextIndex.Close()
}

// FulltextIndex indexes a given element
func (f *Search) Index(e IndexElement) error {
	return f.FulltextIndex.Index(e.EventID, e)
}

// BatchIndex indexes the given elements
func (f *Search) BatchIndex(elements []IndexElement) error {
	batch := f.FulltextIndex.NewBatch()

	for _, element := range elements {
		err := batch.Index(element.EventID, element)
		if err != nil {
			return err
		}
	}
	return f.FulltextIndex.Batch(batch)
}

// Delete deletes an indexed element by the eventID
func (f *Search) Delete(eventID string) error {
	return f.FulltextIndex.Delete(eventID)
}

// Search searches the index given a search term, roomIDs and keys.
func (f *Search) Search(term string, roomIDs, keys []string, limit, from int, orderByStreamPos bool) (*bleve.SearchResult, error) {
	qry := bleve.NewConjunctionQuery()
	termQuery := bleve.NewBooleanQuery()

	terms := strings.Split(term, " ")
	for _, term := range terms {
		matchQuery := bleve.NewMatchQuery(term)
		matchQuery.SetField("Content")
		termQuery.AddMust(matchQuery)
	}
	qry.AddQuery(termQuery)

	roomQuery := bleve.NewBooleanQuery()
	for _, roomID := range roomIDs {
		roomSearch := bleve.NewMatchQuery(roomID)
		roomSearch.SetField("RoomID")
		roomQuery.AddShould(roomSearch)
	}
	if len(roomIDs) > 0 {
		qry.AddQuery(roomQuery)
	}
	keyQuery := bleve.NewBooleanQuery()
	for _, key := range keys {
		keySearch := bleve.NewMatchQuery(key)
		keySearch.SetField("ContentType")
		keyQuery.AddShould(keySearch)
	}
	if len(keys) > 0 {
		qry.AddQuery(keyQuery)
	}

	s := bleve.NewSearchRequestOptions(qry, limit, from, false)
	s.Fields = []string{"*"}
	s.SortBy([]string{"_score"})
	if orderByStreamPos {
		s.SortBy([]string{"-StreamPosition"})
	}

	return f.FulltextIndex.Search(s)
}

func openIndex(cfg config.Fulltext) (bleve.Index, error) {
	m := getMapping(cfg)
	if cfg.InMemory {
		return bleve.NewMemOnly(m)
	}
	if index, err := bleve.Open(string(cfg.IndexPath)); err == nil {
		return index, nil
	}

	index, err := bleve.New(string(cfg.IndexPath), m)
	if err != nil {
		return nil, err
	}
	return index, nil
}

func getMapping(cfg config.Fulltext) *mapping.IndexMappingImpl {
	enFieldMapping := bleve.NewTextFieldMapping()
	enFieldMapping.Analyzer = cfg.Language

	eventMapping := bleve.NewDocumentMapping()
	eventMapping.AddFieldMappingsAt("Content", enFieldMapping)
	eventMapping.AddFieldMappingsAt("StreamPosition", bleve.NewNumericFieldMapping())

	// Index entries as is
	idFieldMapping := bleve.NewKeywordFieldMapping()
	eventMapping.AddFieldMappingsAt("ContentType", idFieldMapping)
	eventMapping.AddFieldMappingsAt("RoomID", idFieldMapping)
	eventMapping.AddFieldMappingsAt("EventID", idFieldMapping)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.AddDocumentMapping("Event", eventMapping)
	indexMapping.DefaultType = "Event"
	return indexMapping
}