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

package transaction

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	t.Run("manager_creation", func(t *testing.T) {
		m := NewManager(nil)
		require.NotNil(t, m.tracker)
	})
	t.Run("manager_get_tx", func(t *testing.T) {
		m := NewManager(nil)
		tx, err := m.GetTx(nil)
		require.Nil(t, tx)
		require.Equal(t, ErrTxCtxMissing, err)
	})
}
