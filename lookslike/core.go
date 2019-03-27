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
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/elastic/lookslike/lookslike/isdefs"
	"github.com/elastic/lookslike/lookslike/paths"
	"github.com/elastic/lookslike/lookslike/results"
	"github.com/elastic/lookslike/lookslike/validator"
)

// Compose combines multiple SchemaValidators into a single one.
func Compose(validators ...validator.Validator) validator.Validator {
	return func(actual interface{}) *results.Results {
		res := make([]*results.Results, len(validators))
		for idx, validator := range validators {
			res[idx] = validator(actual)
		}

		combined := results.NewResults()
		for _, r := range res {
			r.EachResult(func(path paths.Path, vr results.ValueResult) bool {
				combined.Record(path, vr)
				return true
			})
		}
		return combined
	}
}

// Strict is used when you want any unspecified keys that are encountered to be considered errors.
func Strict(laxValidator validator.Validator) validator.Validator {
	return func(actual interface{}) *results.Results {
		res := laxValidator(actual)

		// The inner workings of this are a little weird
		// We use a hash of dotted paths to track the res
		// We can Check if a key had a test associated with it by looking up the laxValidator
		// result data
		// What's trickier is intermediate maps, maps don't usually have explicit tests, they usually just have
		// their properties tested.
		// This method counts an intermediate map as tested if a subkey is tested.
		// Since the datastructure we have to search is a flattened hashmap of the original map we take that hashmap
		// and turn it into a sorted string array, then do a binary prefix search to determine if a subkey was tested.
		// It's a little weird, but is fairly efficient. We could stop using the flattened map as a datastructure, but
		// that would add complexity elsewhere. Probably a good refactor at some point, but not worth it now.
		validatedPaths := []string{}
		for k := range res.Fields {
			validatedPaths = append(validatedPaths, k)
		}
		sort.Strings(validatedPaths)

		walk(actual, false, func(woi walkObserverInfo) error {
			_, validatedExactly := res.Fields[woi.path.String()]
			if validatedExactly {
				return nil // This key was tested, passes strict test
			}

			// Search returns the point just before an actual match (since we ruled out an exact match with the cheaper
			// hash Check above. We have to validate the actual match with a prefix Check as well
			matchIdx := sort.SearchStrings(validatedPaths, woi.path.String())
			if matchIdx < len(validatedPaths) && strings.HasPrefix(validatedPaths[matchIdx], woi.path.String()) {
				return nil
			}

			res.Merge(results.StrictFailureResult(woi.path))

			return nil
		})

		return res
	}
}

func Compile(in interface{}) (validator.Validator, error) {
	switch in.(type) {
	case validator.Map:
		return compileMap(in.(validator.Map))
	case validator.Slice:
		return compileSlice(in.(validator.Slice))
	case isdefs.IsDef:
		return compileIsDef(in.(isdefs.IsDef))
	default:
		msg := fmt.Sprintf("Cannot compile definition from %v (%T). Expected one of 'validator.Map', 'Slice', or 'IsDef'", in, in)
		return nil, errors.New(msg)
	}
}

func compileMap(in validator.Map) (validator validator.Validator, err error) {
	wo, compiled := setupWalkObserver()
	err = walkMap(in, true, wo)

	return func(actual interface{}) *results.Results {
		return compiled.Check(actual)
	}, err
}

func compileSlice(in validator.Slice) (validator validator.Validator, err error) {
	wo, compiled := setupWalkObserver()
	err = walkSlice(in, true, wo)

	// Slices are always strict in validation because
	// it would be surprising to only validate the first specified values
	return Strict(func(actual interface{}) *results.Results {
		return compiled.Check(actual)
	}), err
}

func compileIsDef(def isdefs.IsDef) (validator validator.Validator, err error) {
	return func(actual interface{}) *results.Results {
		return def.Check(paths.Path{}, actual, true)
	}, nil
}

func setupWalkObserver() (walkObserver, *CompiledSchema) {
	compiled := make(CompiledSchema, 0)
	return func(current walkObserverInfo) error {
		// Determine whether we should test this value
		// We want to test all values except collections that contain a value
		// If a collection contains a value, we Check those 'leaf' values instead
		rv := reflect.ValueOf(current.value)
		kind := rv.Kind()
		isCollection := kind == reflect.Map || kind == reflect.Slice
		isNonEmptyCollection := isCollection && rv.Len() > 0

		if !isNonEmptyCollection {
			isDef, isIsDef := current.value.(isdefs.IsDef)
			if !isIsDef {
				isDef = isdefs.IsEqual(current.value)
			}

			compiled = append(compiled, flatValidator{current.path, isDef})
		}
		return nil
	}, &compiled
}

// MustCompile compiles the given validation, panic-ing if that map is invalid.
func MustCompile(in interface{}) validator.Validator {
	compiled, err := Compile(in)
	if err != nil {
		panic(err)
	}
	return compiled
}
