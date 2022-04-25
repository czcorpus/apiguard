// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

// LGuideResponse represents a parsed response
// from the language guide
type LGuideResponse struct {
}

type LGuideResponseParser struct {
}

func (parser *LGuideResponseParser) Parse(src string) LGuideResponse {
	return LGuideResponse{}
}
