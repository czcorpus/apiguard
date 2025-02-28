// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	apiRoutes http.Handler
}

// EventSourceBlock represents a single data source response
// as passed by APIGuard's data stream.
type EventSourceBlock struct {

	// Source is a unique identifier specifying requested data. Naturally,
	// original APIGuard URL which would be used in the "proxy" mode,
	// is the best solution for this. Such value is easy to register
	// by WaG API clients which would use such URL anyway.
	Source string

	// Data returned by an API. The format depends on the API and possibly
	// on the fact whether the client required base64 encoding for returned
	// data.
	Data []byte

	// Status contains the original HTTP status code as obtained
	// from an API
	Status int
}

// Open handles the "open wstream" request which is basically a list
// of requests to individual APIs configured in APIGuard. It creates
// an EventSource stream and returns data as they arrive from those
// APIs.
func (actions *Actions) Open(ctx *gin.Context) {

	var args StreamRequestJSON

	if err := ctx.BindJSON(&args); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	args.ApplyDefaults()

	responseCh := make(chan EventSourceBlock)
	var wg sync.WaitGroup

	for _, reqData := range args.Requests {
		wg.Add(1)
		go func(rd request) {
			apiWriter := NewAPIWriter()
			var bodyReader io.Reader
			if len(reqData.Body) > 0 {
				bodyBuff := new(bytes.Buffer)
				bodyBuff.Write(reqData.Body)
				bodyReader = bodyBuff
			}
			req, _ := http.NewRequest(reqData.Method, reqData.URL, bodyReader)
			req.RemoteAddr = ctx.RemoteIP()
			actions.apiRoutes.ServeHTTP(apiWriter, req)
			var data []byte
			if reqData.Base64EncodeResult {
				data = apiWriter.GetAsBase64()

			} else {
				data = apiWriter.GetRawBytes()
			}
			responseCh <- EventSourceBlock{
				Source: reqData.URL,
				Data:   data,
				Status: apiWriter.StatusCode(),
			}
			wg.Done()
		}(reqData)

	}

	go func() {
		wg.Wait()
		close(responseCh)
	}()

	ctx.Stream(func(w io.Writer) bool {
		response, ok := <-responseCh
		if !ok {
			return false
		}
		eventName := response.Source
		eventData := string(response.Data)
		fmt.Fprintf(w, "event: %s\nstatus: %d\ndata: %s\n\n", eventName, response.Status, eventData)
		return true
	})
}

func NewActions(apiRoutes http.Handler) *Actions {
	return &Actions{
		apiRoutes: apiRoutes,
	}
}
