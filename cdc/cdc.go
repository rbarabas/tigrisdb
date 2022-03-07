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
	"bytes"
	"context"
	"encoding/gob"
	"errors"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/rs/zerolog/log"
)

type ctxKey struct{}

type queue struct {
	Entries []interface{}
}

type setEntry struct {
	Key  fdb.Key
	Data []byte
}

type clearEntry struct {
	Kr fdb.KeyRange
}

func init() {
	gob.Register(setEntry{})
	gob.Register(clearEntry{})
}

func (q *queue) addEntry(entry interface{}) {
	q.Entries = append(q.Entries, entry)
}

func encode(q *queue) ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode(q)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func decode(data []byte) (*queue, error) {
	q := queue{}
	b := bytes.Buffer{}
	b.Write(data)
	d := gob.NewDecoder(&b)
	err := d.Decode(&q)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (q *queue) dump() {
	for _, entry := range q.Entries {
		switch e := entry.(type) {
		case setEntry:
			log.Info().Bytes("key", e.Key).Bytes("data", e.Data).Msg("set")
		case clearEntry:
			log.Info().Bytes("lk", e.Kr.Begin.FDBKey()).Bytes("rk", e.Kr.End.FDBKey()).Msg("clear")
		}
	}
}

func WrapContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, &queue{})
}

func OnSet(ctx context.Context, key fdb.Key, data []byte) error {
	q, err := getQueue(ctx)
	if err != nil {
		return err
	}
	q.addEntry(setEntry{Key: key, Data: data})
	return nil
}

func OnClearRange(ctx context.Context, kr fdb.KeyRange) error {
	q, err := getQueue(ctx)
	if err != nil {
		return err
	}
	q.addEntry(clearEntry{Kr: kr})
	return nil
}

func OnCommit(ctx context.Context) ([]byte, error) {
	q, err := getQueue(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		q.Entries = nil
	}()

	q.dump()
	return encode(q)
}

func OnRollback(ctx context.Context) {
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
			switch e := entry.(type) {
			case setEntry:
				tx.Set(e.Key, e.Data)
			case clearEntry:
				tx.ClearRange(e.Kr)
			}
		}
	}

	err = tx.Commit().Get()
	if err != nil {
		return err
	}

	return nil
}
