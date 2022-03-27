// Copyright 2022 Tigris Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cdc

import (
	"context"
	"errors"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
	"github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog/log"
)

type ctxKey struct{}

type Listener struct{}

type queue struct {
	Entries []Entry
}

type Entry struct {
	Type string
	LKey []byte
	RKey []byte `json:",omitempty"`
	Data []byte `json:",omitempty"`
}

const (
	setType   = "set"
	clearType = "clear"
)

func (q *queue) addEntry(entry Entry) {
	q.Entries = append(q.Entries, entry)
}

func encode(q *queue) ([]byte, error) {
	return jsoniter.Marshal(q)
}

func decode(data []byte) (*queue, error) {
	q := queue{}
	return &q, jsoniter.Unmarshal(data, &q)
}

func (q *queue) dump() {
	for _, e := range q.Entries {
		log.Info().
			Str("type", e.Type).
			Bytes("lkey", e.LKey).
			Bytes("rkey", e.RKey).
			Bytes("data", e.Data).
			Msg("entry")
	}
}

func WrapContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, &queue{})
}

func (l *Listener) OnSet(ctx context.Context, key fdb.Key, data []byte) error {
	q, err := getQueue(ctx)
	if err != nil {
		return err
	}
	q.addEntry(Entry{Type: setType, LKey: key, Data: data})
	return nil
}

func (l *Listener) OnClearRange(ctx context.Context, kr fdb.KeyRange) error {
	q, err := getQueue(ctx)
	if err != nil {
		return err
	}
	q.addEntry(Entry{Type: clearType, LKey: kr.Begin.FDBKey(), RKey: kr.End.FDBKey()})
	return nil
}

func (l *Listener) OnCommit(ctx context.Context, tx *fdb.Transaction) error {
	q, err := getQueue(ctx)
	if err != nil {
		return err
	}

	defer func() {
		q.Entries = nil
	}()

	q.dump()
	data, err := encode(q)
	if err != nil {
		return err
	}

	key, err := getCDCKey()
	if err != nil {
		return err
	}

	tx.SetVersionstampedKey(key, data)

	return nil
}

func getCDCKey() (fdb.Key, error) {
	s := subspace.FromBytes([]byte("cdc"))
	v := tuple.IncompleteVersionstamp(0)
	t := []tuple.TupleElement{v}
	return s.PackWithVersionstamp(t)
}

func (l *Listener) OnCancel(ctx context.Context) {
	q, _ := getQueue(ctx)
	if q != nil {
		q.Entries = nil
	}
}

func getQueue(ctx context.Context) (*queue, error) {
	q, ok := ctx.Value(ctxKey{}).(*queue)
	if !ok {
		return nil, errors.New("failed to cast to *queue")
	} else if q == nil {
		return nil, errors.New("no queue on context")
	} else {
		return q, nil
	}
}

func Replay(db fdb.Database) error {
	tx, err := db.CreateTransaction()
	if err != nil {
		return err
	}

	lk := fdb.Key("cdc\x00")
	rk := fdb.Key("cdc\xff")
	r := tx.GetRange(fdb.KeyRange{Begin: lk, End: rk}, fdb.RangeOptions{})

	i := r.Iterator()
	for i.Advance() {
		kv, err := i.Get()
		if err != nil {
			return err
		}

		q, err := decode(kv.Value)
		if err != nil {
			return err
		}

		q.dump()

		for _, entry := range q.Entries {
			switch entry.Type {
			case setType:
				tx.Set(getFDBKey(entry.LKey), entry.Data)
			case clearType:
				tx.ClearRange(fdb.KeyRange{Begin: getFDBKey(entry.LKey), End: getFDBKey(entry.RKey)})
			}
		}
	}

	err = tx.Commit().Get()
	if err != nil {
		return err
	}

	return nil
}

func getFDBKey(data []byte) fdb.Key {
	return data
}
