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

type Offset [12]byte
type Handle string

type StartOptions struct{}
type StopOptions struct{}
type ReadOptions struct{}

type Publisher interface {
	Start(handle Handle, options StartOptions)
	Stop(handle Handle, options StopOptions)
	Checkpoint(handle Handle, offset Offset)
}

type Subscriber interface {
	Read(handle Handle, options ReadOptions) ([]Entry, Offset)
}

type Sink interface {
	Append(entry Entry)
	Trim(offset Offset)
}

type Message struct {
	Version byte
	Ops     []Op
}

type Op struct {
	Rows []Entry
}
