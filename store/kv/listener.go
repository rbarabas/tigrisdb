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

	"github.com/apple/foundationdb/bindings/go/src/fdb"
)

type Listener interface {
	OnSet(ctx context.Context, key fdb.Key, data []byte) error
	OnClearRange(ctx context.Context, kr fdb.KeyRange) error
	OnCommit(ctx context.Context, tx *fdb.Transaction) error
	OnCancel(ctx context.Context)
}

type NoListener struct {
	Listener
}

func (l *NoListener) OnSet(context.Context, fdb.Key, []byte) error {
	return nil
}

func (l *NoListener) OnClearRange(context.Context, fdb.KeyRange) error {
	return nil
}

func (l *NoListener) OnCommit(context.Context, *fdb.Transaction) error {
	return nil
}

func (l *NoListener) OnCancel(ctx context.Context) {
}
