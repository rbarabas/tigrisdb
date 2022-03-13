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

package v1

import (
	"context"
	"encoding/json"
	api "github.com/tigrisdata/tigrisdb/api/server/v1"
	"github.com/tigrisdata/tigrisdb/cdc"
	"github.com/tigrisdata/tigrisdb/encoding"
	"github.com/tigrisdata/tigrisdb/query/filter"
	"github.com/tigrisdata/tigrisdb/query/read"
	"github.com/tigrisdata/tigrisdb/query/update"
	"github.com/tigrisdata/tigrisdb/server/transaction"
	"github.com/tigrisdata/tigrisdb/store/kv"
	ulog "github.com/tigrisdata/tigrisdb/util/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"
)

// QueryRunner is responsible for executing the current query and return the response
type QueryRunner interface {
	Run(ctx context.Context, req *Request) (*Response, error)
}

// QueryRunnerFactory is responsible for creating query runners for different queries
type QueryRunnerFactory struct {
	txMgr   *transaction.Manager
	encoder encoding.Encoder
}

// NewQueryRunnerFactory returns QueryRunnerFactory object
func NewQueryRunnerFactory(txMgr *transaction.Manager, encoder encoding.Encoder) *QueryRunnerFactory {
	return &QueryRunnerFactory{
		txMgr:   txMgr,
		encoder: encoder,
	}
}

// GetTxQueryRunner returns TxQueryRunner
func (f *QueryRunnerFactory) GetTxQueryRunner() *TxQueryRunner {
	return &TxQueryRunner{
		txMgr:   f.txMgr,
		encoder: f.encoder,
	}
}

// GetStreamingQueryRunner returns StreamingQueryRunner
func (f *QueryRunnerFactory) GetStreamingQueryRunner(streaming Streaming) *StreamingQueryRunner {
	return &StreamingQueryRunner{
		txMgr:     f.txMgr,
		encoder:   f.encoder,
		streaming: streaming,
	}
}

// TxQueryRunner is a runner used for Queries mainly writes that needs to be executed in the context of the transaction
type TxQueryRunner struct {
	txMgr   *transaction.Manager
	encoder encoding.Encoder
}

// Run is responsible for running/executing the query
func (q *TxQueryRunner) Run(ctx context.Context, req *Request) (*Response, error) {
	tx, err := q.txMgr.GetInheritedOrStartTx(ctx, api.GetTransaction(req.apiRequest), false)
	if err != nil {
		return nil, err
	}

	ctx = cdc.WrapContext(ctx)

	var txErr error
	defer func() {
		var err error
		if txErr == nil {
			err = tx.Commit(ctx)
		} else {
			err = tx.Rollback(ctx)
		}
		if txErr == nil {
			txErr = err
		}
	}()

	if reqFilter := api.GetFilter(req.apiRequest); reqFilter != nil {
		txErr = q.iterateFilter(ctx, req, tx, reqFilter)
	} else {
		txErr = q.iterateDocument(ctx, req, tx)
	}

	if ulog.E(txErr) {
		return nil, txErr
	}

	return &Response{}, err
}

func (q *TxQueryRunner) iterateFilter(ctx context.Context, req *Request, tx transaction.Tx, reqFilter []byte) error {
	filters, err := filter.Build(reqFilter)
	if err != nil {
		return err
	}

	kb := filter.NewKeyBuilder(filter.NewStrictEqKeyComposer(req.collection.StorageName()))
	iKeys, err := kb.Build(filters, req.collection.PrimaryKeys())
	if err != nil {
		return err
	}

	switch api.RequestType(req.apiRequest) {
	case api.Update:
		var factory *update.FieldOperatorFactory
		factory, err = update.BuildFieldOperators(req.apiRequest.(*api.UpdateRequest).Fields)
		if err != nil {
			return err
		}

		for _, key := range iKeys {
			// decode the fields now
			err = tx.Update(ctx, key, func(existingDoc []byte) ([]byte, error) {
				merged, er := factory.MergeAndGet(existingDoc)
				if er != nil {
					return nil, er
				}
				return merged, nil
			})
		}
	case api.Delete:
		for _, key := range iKeys {
			err = tx.Delete(ctx, key)
		}
	}

	return err
}

func (q *TxQueryRunner) iterateDocument(ctx context.Context, req *Request, tx transaction.Tx) error {
	var err error
	for _, d := range req.documents {
		// ToDo: need to implement our own decoding to only extract custom keys
		var s = &structpb.Struct{}
		if err = json.Unmarshal(d, s); err != nil {
			return err
		}

		key, err := q.encoder.BuildKey(s.GetFields(), req.collection)
		if err != nil {
			return err
		}

		switch api.RequestType(req.apiRequest) {
		case api.Insert:
			err = tx.Insert(ctx, key, d)
			if err != nil && err.Error() == "file already exists" {
				// FDB returning it as string, probably we need to move this check in KV
				return api.Errorf(codes.AlreadyExists, "row already exists")
			}
		case api.Replace:
			err = tx.Replace(ctx, key, d)
		}

		if err != nil {
			return err
		}
	}

	return err
}

// StreamingQueryRunner is a runner used for Queries that are reads and needs to return result in streaming fashion
type StreamingQueryRunner struct {
	txMgr     *transaction.Manager
	encoder   encoding.Encoder
	streaming Streaming
}

// Run is responsible for running/executing the query
func (q *StreamingQueryRunner) Run(ctx context.Context, req *Request) (*Response, error) {
	_, err := q.txMgr.GetInherited(api.GetTransaction(req.apiRequest))
	if err != nil {
		return nil, err
	}

	filters, err := filter.Build(api.GetFilter(req.apiRequest))
	if err != nil {
		return nil, err
	}

	kb := filter.NewKeyBuilder(filter.NewStrictEqKeyComposer(req.collection.StorageName()))
	iKeys, err := kb.Build(filters, req.collection.PrimaryKeys())
	if err != nil {
		return nil, err
	}

	fieldFactory, err := read.BuildFields(req.apiRequest.(*api.ReadRequest).Fields)
	if ulog.E(err) {
		return nil, err
	}

	var totalResults int64 = 0
	for _, key := range iKeys {
		it, err := q.txMgr.GetKV().Read(ctx, req.collection.StorageName(), kv.BuildKey(key.PrimaryKeys()...))
		if err != nil {
			return nil, err
		}

		for it.More() {
			if req.apiRequest.(*api.ReadRequest).GetOptions().GetLimit() > 0 && req.apiRequest.(*api.ReadRequest).GetOptions().GetLimit() <= totalResults {
				return &Response{}, nil
			}

			v, err := it.Next()
			if err != nil {
				return nil, err
			}

			newValue, err := fieldFactory.Apply(v.Value)
			if err != nil {
				return nil, err
			}

			if err := q.streaming.Send(&api.ReadResponse{
				Doc: newValue,
				Key: v.FDBKey,
			}); ulog.E(err) {
				return nil, err
			}

			totalResults++
		}
	}

	return &Response{}, nil
}
