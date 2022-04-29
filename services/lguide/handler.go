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
	"math"
	"math/rand"
	"net/http"
	"time"
	"wum/services"
)

var (
	// baseServiceURL = "https://prirucka.ujc.cas.cz"
	baseServiceURL = "http://localhost:8081"
	//targetServiceURLPath = "https://prirucka.ujc.cas.cz/?slovo=%s"
	targetServiceURLPath = "/?slovo=%s"
	//targetServicePingURLPath = "https://prirucka.ujc.cas.cz/?id=%s&action=single"
	targetServicePingURLPath = "?id=%s&action=single"
)

type LanguageGuideActions struct {
}

func createPingRequest(query string) {
	time.Sleep(500 * time.Millisecond)
	resp, err := http.Get(fmt.Sprintf(baseServiceURL+targetServicePingURLPath, query))
	if err != nil {
		log.Print("ERROR: ", err)
	}
	defer resp.Body.Close()
}

func createResourceRequest(url string) {
	resp, err := http.Get(url)
	if err != nil {
		log.Print("ERROR: ", err)
	}
	defer resp.Body.Close()
}

func triggerDummyRequests(query string, data *ParsedData) {
	urls := make([]string, 0, len(data.CSSLinks)+len(data.Scripts)+1)
	urls = append(urls, data.Scripts...)
	urls = append(urls, data.CSSLinks...)
	for i, u := range urls {
		urls[i] = baseServiceURL + u
	}
	urls = append(urls, fmt.Sprintf(baseServiceURL+targetServicePingURLPath, query))
	rand.Shuffle(len(urls), func(i, j int) {
		urls[i], urls[j] = urls[j], urls[i]
	})
	for _, url := range urls {
		go func(curl string) {
			time.Sleep(time.Duration(time.Duration(math.RoundToEven(rand.NormFloat64()+1.5*1000)) * time.Millisecond))
			createResourceRequest(curl)
		}(url)
	}
}

func (a *LanguageGuideActions) Query(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query().Get("q")
	resp, err := http.Get(fmt.Sprintf(baseServiceURL+targetServiceURLPath, query))
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}
	defer resp.Body.Close()
	responseSrc, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		services.WriteJSONErrorResponse(w, services.NewActionError(err.Error()), 500)
		return
	}
	parsed := Parse(string(responseSrc))
	triggerDummyRequests(query, parsed)
	services.WriteJSONResponse(w, parsed)
}
