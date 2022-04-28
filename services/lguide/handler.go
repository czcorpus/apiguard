// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
	"wum/services"
)

var (
	//targetServiceURLPattern = "https://prirucka.ujc.cas.cz/?slovo=%s"
	targetServiceURLPattern = "http://localhost:8081/?slovo=%s"
	//targetServicePingURLPattern = "https://prirucka.ujc.cas.cz/?id=%s&action=single"
	targetServicePingURLPattern = "http://localhost:8081?id=%s&action=single"
)

type LanguageGuideActions struct {
}

func createPingRequest(query string) {
	time.Sleep(500 * time.Millisecond)
	resp, err := http.Get(fmt.Sprintf(targetServicePingURLPattern, query))
	if err != nil {
		log.Print("ERROR: ", err)
	}
	defer resp.Body.Close()
}

func (a *LanguageGuideActions) Query(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query().Get("q")
	resp, err := http.Get(fmt.Sprintf(targetServiceURLPattern, query))
	if err != nil {
		log.Print("ERROR: ", err)
	}
	defer resp.Body.Close()
	responseSrc, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("ERROR: ", err)
	}
	fmt.Println("RESP ", string(responseSrc))
	go createPingRequest(query)

	parsed := Parse(string(responseSrc))
	services.WriteJSONResponse(w, parsed)
	//w.Write([]byte("OK"))
}
