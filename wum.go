// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package main

import (
	"net/http"

	"wum/actions/lguide"

	"github.com/gorilla/mux"
)

func coreMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func main() {

	router := mux.NewRouter()
	router.Use(coreMiddleware)

	langGuideActions := lguide.LanguageGuideActions{}
	router.HandleFunc("/language-guide", langGuideActions.Query).Methods(http.MethodPost)
}
