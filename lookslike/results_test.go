// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package lookslike

import (
	"testing"

	"github.com/elastic/lookslike/lookslike/paths"
	"github.com/elastic/lookslike/lookslike/results"

	"github.com/stretchr/testify/assert"
)

func TestEmpty(t *testing.T) {
	r := results.NewResults()
	assert.True(t, r.Valid)
	assert.Empty(t, r.DetailedErrors().Fields)
	assert.Empty(t, r.Errors())
}

func TestWithError(t *testing.T) {
	r := results.NewResults()
	r.Record(paths.MustParsePath("foo"), results.KeyMissingVR)
	r.Record(paths.MustParsePath("bar"), results.ValidVR)

	assert.False(t, r.Valid)

	assert.Equal(t, results.KeyMissingVR, r.Fields["foo"][0])
	assert.Equal(t, results.ValidVR, r.Fields["bar"][0])

	assert.Equal(t, results.KeyMissingVR, r.DetailedErrors().Fields["foo"][0])
	assert.NotContains(t, r.DetailedErrors().Fields, "bar")

	assert.False(t, r.DetailedErrors().Valid)
	assert.NotEmpty(t, r.Errors())
}
