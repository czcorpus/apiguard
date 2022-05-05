// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package counting

import (
	"log"
	"net/http"
	"wum/logging"
)

type Analyzer struct {
}

func (a *Analyzer) Learn(req *http.Request, isLegit bool) {

}

func (a *Analyzer) Evaluate(req *http.Request) bool {
	ip := logging.ExtractClientIP(req)
	session, err := req.Cookie(logging.WaGSessionName)
	var sessionID string
	if err == nil {
		sessionID = session.Value[:logging.MaxSessionValueLength]
	}
	log.Printf("DEBUG: about to evaluate IP %s and sessionID %s", ip, sessionID)
	return true
}
