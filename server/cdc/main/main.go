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

package main

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/rs/zerolog/log"
	"github.com/tigrisdata/tigrisdb/server/cdc"
	"github.com/tigrisdata/tigrisdb/server/config"
	ulog "github.com/tigrisdata/tigrisdb/util/log"
)

func main() {
	ulog.Configure(config.DefaultConfig.Log)

	cfg := &config.DefaultConfig.FoundationDB

	fdb.MustAPIVersion(630)
	db, err := fdb.OpenDatabase(cfg.ClusterFile)
	if err != nil {
		log.Fatal().Err(err).Msg("error")
	}

	err = cdc.Replay(db)
	if err != nil {
		log.Fatal().Err(err).Msg("error")
	}
}
