// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

import (
	"apiguard/wagstream/tileconf"
	loader "apiguard/wagstream/tileconf"
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
	"github.com/rs/zerolog/log"
)

const (
	esAPIWriterChanBufferSize = 40
	respPingInterval          = time.Second * 5
)

type Actions struct {
	apiRoutes  http.Handler
	streams    *streams
	confLoader wagConfLoader
}

type wagConfLoader interface {
	GetConf(id string) ([]byte, error)
}

func (actions *Actions) writeStreamingError(ctx *gin.Context, tileID int, err error) {
	messageJSON, err2 := sonic.Marshal(streamingError{err.Error()})
	if err2 != nil {
		ctx.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	// We use status 200 here deliberately as we don't want to trigger
	// the error handler.
	ctx.String(
		http.StatusOK,
		fmt.Sprintf("event: DataTile-%d\ndata: %s\n\n", tileID, messageJSON),
	)
}

func (actions *Actions) CreateStream(ctx *gin.Context) {
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

func (actions *Actions) emptyResponse(req *http.Request) []byte {
	if req.Header.Get("content-type") == "application/json" {
		return []byte("null")
	}
	return []byte{}
}

// StartStream handles the "open wstream" request which is basically a list
// of requests to individual APIs configured in APIGuard. It creates
// an EventSource stream and returns data as they arrive from those
// APIs.
func (actions *Actions) StartStream(ctx *gin.Context) {
	streamID := ctx.Param("id")
	args := actions.streams.Get(streamID)
	if args == nil {
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("stream %s not found", streamID), http.StatusNotFound)
		return
	}

	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")
	// The following one is super-important; otherwise Nginx's buffering
	// may interfere with how individual events are sent and the output
	// may pause in the middle of already available event data.
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")

	responseCh := make(chan any, len(args.Requests)*2)
	var wg sync.WaitGroup

	ticker := time.NewTicker(respPingInterval)
	go func() {
		for c := range ticker.C {
			responseCh <- &PingResp{TS: c}
		}
	}()

	// WaG may intentionally (if two or more tiles require the same data) produce requests
	// which just read data from other tiles. For that matter, we need to group the tiles.
	groupedRequests := make(groupedRequests)
	for _, reqData := range args.Requests {
		groupedRequests.register(reqData)
	}

	log.Debug().Any("groups", groupedRequests).Msg("created grouped wstream requests")

	for reqData, tiles := range groupedRequests.valIter {
		wg.Add(1)
		go func(rd request) {
			var bodyReader io.Reader
			if len(rd.Body) > 0 {
				bodyBuff := new(bytes.Buffer)
				bodyBuff.Write([]byte(rd.Body))
				bodyReader = bodyBuff
			}
			log.Debug().Str("url", rd.URL).Str("method", rd.Method).Msg("registering wstream backend request")
			req, _ := http.NewRequest(rd.Method, rd.URL, bodyReader)
			req.RemoteAddr = ctx.RemoteIP()
			req.Header.Add("content-type", rd.ContentType)
			for _, ck := range ctx.Request.Cookies() {
				req.AddCookie(ck)
			}

			if rd.URL == "" { // this for situations where WaG needs an empty response
				responseCh <- &StreamingReadyResp{
					TileID:   tiles[0].TileID,
					QueryIdx: tiles[0].QueryIdx,
					Source:   rd.URL,
					Data:     actions.emptyResponse(req),
					Status:   http.StatusOK,
				}
				wg.Done()

			} else if rd.IsEventSource {
				apiWriter := NewESAPIWriter(esAPIWriterChanBufferSize)
				// Important note:
				// Here we rely on the fact that a respective
				// route handler will call ctx.Writer.Flush()
				// (see the handler and our custom AfterHandlerCallback)
				// Without that, the channel won't get closed which
				// would cause the main data stream to never finish!
				go func() {
					for resp := range apiWriter.Responses() {
						responseCh <- &RawStreamingReadyResp{
							TileID:   tiles[0].TileID,
							QueryIdx: tiles[0].QueryIdx,
							Data:     resp,
							Status:   apiWriter.statusCode,
						}
					}
					wg.Done()
				}()
				actions.apiRoutes.ServeHTTP(apiWriter, req)

			} else {
				apiWriter := NewAPIWriter()

				actions.apiRoutes.ServeHTTP(apiWriter, req)
				var data []byte
				if rd.Base64EncodeResult {
					data = apiWriter.GetAsBase64()

				} else {
					data = apiWriter.GetRawBytes()
				}
				responseCh <- &StreamingReadyResp{
					TileID:   tiles[0].TileID,
					QueryIdx: tiles[0].QueryIdx,
					Source:   rd.URL,
					Data:     data,
					Status:   apiWriter.StatusCode(),
				}
				wg.Done()
			}
		}(*reqData)

	}

	go func() {
		wg.Wait()
		ticker.Stop()
		close(responseCh)
	}()

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
			switch tResponse := response.(type) {
			case *StreamingReadyResp:
				eventData := string(tResponse.Data)
				_, err := ctx.Writer.WriteString(
					fmt.Sprintf(
						"event: DataTile-%d.%d\ndata: %s\n\n", tResponse.TileID, tResponse.QueryIdx, eventData),
				)
				if err != nil {
					// not much we can do here
					log.Error().Err(err).Msg("failed to write EventSource data")
					return
				}

				ctx.Writer.Flush()
			case *RawStreamingReadyResp:
				if tResponse.Status >= 200 && tResponse.Status < 300 {
					ctx.Writer.WriteHeader(http.StatusOK)
					_, err := ctx.Writer.Write(tResponse.Data)
					if err != nil {
						actions.writeStreamingError(ctx, tResponse.TileID, err)
						ctx.Writer.Flush()
						return
					}

				} else {
					actions.writeStreamingError(
						ctx,
						tResponse.TileID,
						fmt.Errorf("received error response from backend (status: %d)", tResponse.Status),
					)
					ctx.Writer.Flush()
				}
			case *PingResp:
				_, err := fmt.Fprintf(ctx.Writer, "data: %d\n\n", tResponse.TS.Unix())
				if err != nil {
					// not much we can do here
					log.Error().Err(err).Msg("failed to write EventSource data")
					return
				}
				ctx.Writer.Flush()

			}
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

func (actions *Actions) TileConf(ctx *gin.Context) {
	data, err := actions.confLoader.GetConf(ctx.Param("id"))
	if err == tileconf.ErrNotFound {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusNotFound)
		return

	} else if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	uniresp.WriteRawJSONResponse(ctx.Writer, data)
}

func NewActions(
	ctx context.Context,
	apiRoutes http.Handler,
	wagTilesConfDir string,
) (*Actions, error) {
	var confLoader wagConfLoader
	if wagTilesConfDir != "" {
		var err error
		confLoader, err = loader.NewJSONFiles(ctx, wagTilesConfDir)
		if err != nil {
			return nil, fmt.Errorf("failed to create wagstream actions: %w", err)
		}

	} else {
		log.Warn().Msg("no wagTilesConfDir specified - APIGuard will not serve as WaG tile configuration provider")
		confLoader = &loader.Null{}
	}

	a := &Actions{
		apiRoutes:  apiRoutes,
		streams:    newStreams(),
		confLoader: confLoader,
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
	return a, nil
}
