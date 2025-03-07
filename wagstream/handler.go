// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	apiRoutes http.Handler
	streams   *streams
}

// EventSourceBlock represents a single data source response
// as passed by APIGuard's data stream.
type EventSourceBlock struct {
	TileID int

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

type streamingError struct {
	Error string `json:"error"`
}

func (actions *Actions) writeStreamingError(ctx *gin.Context, err error) {
	messageJSON, err2 := sonic.Marshal(streamingError{err.Error()})
	if err2 != nil {
		ctx.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	// We use status 200 here deliberately as we don't want to trigger
	// the error handler.
	ctx.String(http.StatusOK, fmt.Sprintf("data: %s\n\n", messageJSON))
}

func (actions *Actions) Create(ctx *gin.Context) {
	var args StreamRequestJSON

	if err := ctx.BindJSON(&args); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	args.ApplyDefaults()
	id := actions.streams.Add(&args)
	ctx.Status(http.StatusCreated)
	uniresp.WriteJSONResponse(ctx.Writer, map[string]string{"id": id})
}

// Open handles the "open wstream" request which is basically a list
// of requests to individual APIs configured in APIGuard. It creates
// an EventSource stream and returns data as they arrive from those
// APIs.
func (actions *Actions) Open(ctx *gin.Context) {
	streamID := ctx.Param("id")
	args := actions.streams.Get(streamID)
	if args == nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("stream %s not found", streamID), http.StatusNotFound)
		return
	}

	responseCh := make(chan EventSourceBlock)
	var wg sync.WaitGroup
	for _, reqData := range args.Requests {
		wg.Add(1)
		go func(rd request) {
			apiWriter := NewAPIWriter()
			var bodyReader io.Reader
			if len(rd.Body) > 0 {
				bodyBuff := new(bytes.Buffer)
				bodyBuff.Write([]byte(rd.Body))
				bodyReader = bodyBuff
			}
			req, _ := http.NewRequest(rd.Method, rd.URL, bodyReader)
			req.RemoteAddr = ctx.RemoteIP()
			req.Header.Add("content-type", rd.ContentType)
			actions.apiRoutes.ServeHTTP(apiWriter, req)
			var data []byte
			if rd.Base64EncodeResult {
				data = apiWriter.GetAsBase64()

			} else {
				data = apiWriter.GetRawBytes()
			}
			responseCh <- EventSourceBlock{
				TileID: rd.TileID,
				Source: rd.URL,
				Data:   data,
				Status: apiWriter.StatusCode(),
			}
			wg.Done()
		}(*reqData)

	}

	go func() {
		wg.Wait()
		close(responseCh)
	}()

	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	for {
		select {
		case response, ok := <-responseCh:
			if !ok {
				ctx.String(
					http.StatusOK,
					"event: close\ndata: \n\n",
				)
				ctx.Writer.Flush()
				return
			}
			eventData := string(response.Data)
			ctx.String(
				http.StatusOK,
				fmt.Sprintf(
					"event: DataTile-%d\ndata: %s\n\n", response.TileID, eventData),
			)
		case <-ctx.Done():
			ctx.String(
				http.StatusOK,
				"event: close\ndata: \n\n",
			)
			ctx.Writer.Flush()
			return
		}
	}
}

func NewActions(ctx context.Context, apiRoutes http.Handler) *Actions {
	a := &Actions{
		apiRoutes: apiRoutes,
		streams:   newStreams(),
	}
	tc := time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-tc.C:
				a.streams.cleanup()
			case <-ctx.Done():
				tc.Stop()
			}
		}
	}()
	return a
}
