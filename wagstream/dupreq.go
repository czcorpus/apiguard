// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wagstream

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/czcorpus/cnc-gokit/collections"
)

// groupedRequests maps between tile URLs (here keys)
// and tileIDs (here values)
type groupedRequests map[string][]*request

// register adds a new tileID=>URL pair.
func (ureqs groupedRequests) register(req *request) {
	reqKey := req.groupingKey()
	vals, ok := ureqs[reqKey]
	if ok {
		if req.OtherTileID == nil { // request without a dependency must be always first
			vals = append([]*request{req}, vals...)

		} else {
			vals = append(vals, req)
		}
		ureqs[reqKey] = vals

	} else {
		ureqs[reqKey] = make([]*request, 1, 10)
		ureqs[reqKey][0] = req
	}
}

func (ureqs groupedRequests) String() string {
	var ans strings.Builder
	var i int
	for k, v := range ureqs {
		values := strings.Join(
			collections.SliceMap(v, func(v *request, i int) string {
				return strconv.Itoa(v.TileID)
			}),
			", ",
		)
		if i > 0 {
			ans.WriteString("; ")
		}
		ans.WriteString(fmt.Sprintf("%s => %s", k, values))
		i++
	}
	return fmt.Sprintf("groupedRequests{ %s }", ans.String())
}

type reqIdent struct {
	TileID   int
	QueryIdx int
}

func (ureqs groupedRequests) valIter(yield func(item *request, tiles []reqIdent) bool) {
	for _, v := range ureqs {
		tiles := make([]reqIdent, len(v))
		for i, t := range v {
			tiles[i] = reqIdent{TileID: t.TileID, QueryIdx: t.QueryIdx}
		}
		if !yield(v[0], tiles) {
			return
		}
	}
}
