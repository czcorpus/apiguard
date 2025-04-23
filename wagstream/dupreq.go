// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

// groupedRequests maps between tile URLs (here keys)
// and tileIDs (here values)
type groupedRequests map[string][]*request

// register adds a new tileID=>URL pair.
func (ureqs groupedRequests) register(req *request) {
	reqKey := req.dedupKey()
	vals, ok := ureqs[reqKey]
	if ok {
		vals = append(vals, req)
		ureqs[reqKey] = vals

	} else {
		ureqs[reqKey] = make([]*request, 1, 10)
		ureqs[reqKey][0] = req
	}
}

func (ureqs groupedRequests) keyIter(yield func(item string) bool) {
	for k := range ureqs {
		if !yield(k) {
			return
		}
	}
}

func (ureqs groupedRequests) valIter(yield func(item *request, tiles []int) bool) {
	for _, v := range ureqs {
		tiles := make([]int, len(v))
		for i, t := range v {
			tiles[i] = t.TileID
		}
		if !yield(v[0], tiles) {
			return
		}
	}
}
