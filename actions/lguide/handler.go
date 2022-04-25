// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

import "net/http"

type LanguageGuideActions struct {
}

func (a *LanguageGuideActions) Query(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("OK"))
}
