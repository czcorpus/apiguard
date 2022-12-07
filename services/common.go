// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package services

import "net/http"

type Cache interface {
	Get(url string) (string, *http.Header, error)
	Set(url, body string, req *http.Request) error
}
