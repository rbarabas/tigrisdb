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

package kv

import (
	"context"
	"unsafe"
)

type KeyValue struct {
	Key    Key
	FDBKey []byte
	Value  []byte
}

type crud interface {
	Insert(ctx context.Context, table []byte, key Key, data []byte) error
	Replace(ctx context.Context, table []byte, key Key, data []byte) error
	Delete(ctx context.Context, table []byte, key Key) error
	DeleteRange(ctx context.Context, table []byte, lKey Key, rKey Key) error
	Read(ctx context.Context, table []byte, key Key) (Iterator, error)
	ReadRange(ctx context.Context, table []byte, lkey Key, rkey Key) (Iterator, error)
	Update(ctx context.Context, table []byte, key Key, apply func([]byte) ([]byte, error)) error
	UpdateRange(ctx context.Context, table []byte, lKey Key, rKey Key, apply func([]byte) ([]byte, error)) error
	SetVersionstampedValue(ctx context.Context, key []byte, value []byte) error
	Get(ctx context.Context, key []byte) ([]byte, error)
}

type Tx interface {
	crud
	Commit(context.Context) error
	Rollback(context.Context) error
}

type KV interface {
	crud
	Tx(ctx context.Context) (Tx, error)
	Batch() (Tx, error)
	CreateTable(ctx context.Context, name []byte) error
	DropTable(ctx context.Context, name []byte) error
}

type Iterator interface {
	Next(*KeyValue) bool
	Err() error
}

type KeyPart interface{}
type Key []KeyPart

func BuildKey(parts ...interface{}) Key {
	ptr := unsafe.Pointer(&parts)
	return *(*Key)(ptr)
}

func (k *Key) AddPart(part interface{}) {
	*k = append(*k, KeyPart(part))
}
