package treq

import (
	"apiguard/common"
	"apiguard/guard"
	"apiguard/proxy"
	"apiguard/reporting"
	"apiguard/services/backend"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/cnc-gokit/util"
	"github.com/czcorpus/mquery-common/concordance"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	maxLines = 10
)

type treqRespLine struct {
	Freq string `json:"freq"`
	Perc string `json:"perc"`
	From string `json:"from"`
	To   string `json:"to"`
}

type concExample struct {
	Text          []concordance.Line `json:"text"`
	InteractionID string             `json:"interactionId"`
	Ref           string             `json:"ref"`
}

type treqExtTranslation struct {
	Word     string       `json:"word"`
	Examples *concExample `json:"examples"`
	Error    string       `json:"error,omitempty"`
}

type treqExtRespLine struct {
	Freq string             `json:"freq"`
	Perc string             `json:"perc"`
	From string             `json:"from"`
	To   treqExtTranslation `json:"to"`
}

type treqExtResponse struct {
	Sum      int               `json:"sum"`
	Lines    []treqExtRespLine `json:"lines"`
	FromCorp string            `json:"fromCorp"`
	ToCorp   string            `json:"toCorp"`
}

type treqResponse struct {
	Sum   int            `json:"sum"`
	Lines []treqRespLine `json:"lines"`
}

type concResponse struct {
	Lines      []concordance.Line
	ConcSize   int
	CorpusSize int
	IPM        float64
	Error      error
}

func (cr *concResponse) UnmarshalJSON(data []byte) error {
	var tmp struct {
		Lines      []concordance.Line `json:"lines"`
		ConcSize   int                `json:"concSize"`
		CorpusSize int                `json:"corpusSize"`
		IPM        float64            `json:"ipm"`
		Error      string             `json:"error,omitempty"`
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	cr.Lines = tmp.Lines
	cr.ConcSize = tmp.ConcSize
	cr.CorpusSize = tmp.CorpusSize
	cr.IPM = tmp.IPM
	if tmp.Error != "" {
		cr.Error = errors.New(tmp.Error)
	}
	return nil
}

func (cr *concResponse) MarshalJSON() ([]byte, error) {
	var errMsg string
	if cr.Error != nil {
		errMsg = cr.Error.Error()
	}
	return json.Marshal(
		struct {
			Lines      []concordance.Line `json:"lines"`
			ConcSize   int                `json:"concSize"`
			CorpusSize int                `json:"corpusSize"`
			IPM        float64            `json:"ipm"`
			Error      string             `json:"error,omitempty"`
		}{
			Lines:    cr.Lines,
			ConcSize: cr.ConcSize,
			IPM:      cr.IPM,
			Error:    errMsg,
		},
	)
}

func (tp *TreqProxy) WithExamples(ctx *gin.Context) {
	var cached, indirectAPICall bool
	var clientID, humanID common.UserID
	t0 := time.Now().In(tp.GlobalCtx().TimezoneLocation)

	defer func(currUserID, currHumanID *common.UserID, indirect *bool, created time.Time) {
		loggedUserID := currUserID
		if currHumanID.IsValid() && tp.Guard().TestUserIsAnonymous(*currHumanID) {
			loggedUserID = currHumanID
		}
		tp.CountRequest(
			ctx,
			created,
			tp.EnvironConf().ServiceKey,
			*loggedUserID,
		)
		tp.GlobalCtx().BackendLogger.Log(
			ctx.Request,
			tp.EnvironConf().ServiceKey,
			time.Since(t0),
			cached,
			*loggedUserID,
			*indirect,
			reporting.BackendActionTypeQuery,
		)
	}(&clientID, &humanID, &indirectAPICall, t0)

	if !strings.HasPrefix(ctx.Request.URL.Path, tp.EnvironConf().ServicePath) {
		proxy.WriteError(ctx, fmt.Errorf("invalid path detected"), http.StatusInternalServerError)
		return
	}
	reqProps := tp.Guard().EvaluateRequest(ctx.Request, tp.authFallbackCookie)
	log.Debug().
		Str("reqPath", ctx.Request.URL.Path).
		Any("reqProps", reqProps).
		Msg("evaluated user treq/* request")
	clientID = reqProps.ClientID
	if reqProps.ProposedResponse == http.StatusUnauthorized {
		_, err, _ := tp.reauthSF.Do("reauth", func() (any, error) {
			resp := tp.LoginWithToken(tp.conf.CNCAuthToken)
			log.Debug().Msgf("reauthentication result: %s", resp.String())
			if resp.Err() == nil {
				c := resp.Cookie(tp.EnvironConf().CNCAuthCookie)
				cVal := "-"
				if c != nil {
					cVal = c.Value
				}
				log.Debug().
					Str("serviceId", tp.EnvironConf().ServiceKey).
					Str("cookieValue", cVal).
					Msg("performed reauthentication")
				if c != nil {
					tp.authFallbackCookie = c
					return true, nil
				}
				return false, nil
			}
			return false, resp.Err()
		})
		if err != nil {
			proxy.WriteError(
				ctx,
				fmt.Errorf("failed to proxy request: %w", err),
				reqProps.ProposedResponse,
			)
			return
		}
		if tp.authFallbackCookie == nil {
			proxy.WriteError(
				ctx,
				fmt.Errorf(
					"failed to proxy request: cnc auth cookie '%s' not found",
					tp.EnvironConf().CNCAuthCookie,
				),
				reqProps.ProposedResponse,
			)
			return
		}

	} else if reqProps.Error != nil {
		proxy.WriteError(
			ctx,
			fmt.Errorf("failed to proxy request: %w", reqProps.Error),
			reqProps.ProposedResponse,
		)
		return

	} else if reqProps.ForbidsAccess() {
		proxy.WriteError(
			ctx,
			errors.New(http.StatusText(reqProps.ProposedResponse)),
			reqProps.ProposedResponse,
		)
		return
	}

	if reqProps.RequiresFallbackCookie {
		log.Debug().
			Str("serviceId", tp.EnvironConf().ServiceKey).
			Str("value", tp.authFallbackCookie.Value).Msg("applying fallback cookie")
		tp.DeleteCookie(ctx.Request, tp.authFallbackCookie.Name)
		ctx.Request.AddCookie(tp.authFallbackCookie)
	}

	passedHeaders := ctx.Request.Header
	if tp.EnvironConf().CNCAuthCookie != tp.conf.FrontendSessionCookieName {
		var err error
		// here we reveal actual human user ID to the API (i.e. not a special fallback user)
		humanID, err = tp.Guard().DetermineTrueUserID(ctx.Request)
		clientID = humanID
		if err != nil {
			log.Error().Err(err).Msgf("failed to extract human user ID information (ignoring)")
		}
	}
	passedHeaders[backend.HeaderAPIUserID] = []string{clientID.String()}
	guard.RestrictResponseTime(
		ctx.Writer,
		ctx.Request,
		tp.EnvironConf().ReadTimeoutSecs,
		tp.Guard(),
		common.ClientID{
			IP: ctx.RemoteIP(),
			ID: clientID,
		},
	)

	// first, remap cookie names
	if tp.reqUsesMappedSession(ctx.Request) {
		err := backend.MapFrontendCookieToBackend(
			ctx.Request,
			tp.conf.FrontendSessionCookieName,
			tp.EnvironConf().CNCAuthCookie,
		)
		if err != nil {
			http.Error(
				ctx.Writer,
				err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}
	// then update auth cookie by x-api-key (if applicable)
	xApiKey := ctx.Request.Header.Get(backend.HeaderAPIKey)
	if xApiKey != "" {
		cookie, err := ctx.Request.Cookie(tp.EnvironConf().CNCAuthCookie)
		if err == nil {
			cookie.Value = xApiKey
		}
	}

	rt0 := time.Now().In(tp.GlobalCtx().TimezoneLocation)

	req := *ctx.Request
	req.Header = req.Header.Clone() // this prevents concurrent access to headers (= a map)
	if reqProps.RequiresFallbackCookie {
		log.Debug().
			Str("serviceId", tp.EnvironConf().ServiceKey).
			Str("value", tp.authFallbackCookie.Value).Msg("applying fallback cookie")
		tp.DeleteCookie(&req, tp.authFallbackCookie.Name)
		req.AddCookie(tp.authFallbackCookie)
	}
	req.Method = "GET"
	req.Body = io.NopCloser(strings.NewReader(""))
	req.URL.Path, _ = url.JoinPath(tp.EnvironConf().ServicePath, "/")
	resp := tp.HandleRequest(&req, reqProps, true)

	cached = resp.IsCacheHit()
	tp.WriteReport(&reporting.ProxyProcReport{
		DateTime: time.Now().In(tp.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   resp.Response().GetStatusCode(),
		Service:  tp.EnvironConf().ServiceKey,
		IsCached: cached,
	})
	if resp.Error() != nil {
		log.Error().Err(resp.Error()).Msgf("failed to proxy request %s", ctx.Request.URL.Path)
		http.Error(
			ctx.Writer,
			fmt.Sprintf("failed to proxy request: %s", resp.Error()),
			http.StatusInternalServerError,
		)
		return
	}

	translatResp, err := resp.ExportResponse()
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	var translations treqResponse
	if err := json.Unmarshal(translatResp, &translations); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	if len(translations.Lines) > maxLines {
		translations.Lines = translations.Lines[:maxLines]
	}

	fromCorp := ctx.Query("fromCorp")
	toCorp := ctx.Query("toCorp")

	ans := &treqExtResponse{
		Sum:      translations.Sum,
		Lines:    make([]treqExtRespLine, len(translations.Lines)),
		FromCorp: fromCorp,
		ToCorp:   toCorp,
	}
	var ansEditLock sync.Mutex
	var wg sync.WaitGroup
	sseEvent := ""
	if tp.EnvironConf().IsStreamingMode {
		sseEvent = fmt.Sprintf(" DataTile-%s.%d", ctx.Query("tileId"), 0)
	}

	for i, translation := range translations.Lines {
		wg.Add(1)
		go func(trLine treqRespLine, lineIdx int) {
			defer wg.Done()
			cql := fmt.Sprintf(
				`[word=="%s"] within %s:[word=="%s"]`,
				strings.ReplaceAll(trLine.From, "\"", "\\\""),
				toCorp,
				strings.ReplaceAll(trLine.To, "\"", "\\\""),
			)
			urlQuery := make(url.Values)
			urlQuery.Add("q", cql)
			urlQuery.Add("maxRows", strconv.Itoa(tp.conf.NumExamplesPerWord))
			urlQuery.Add("showTextProps", "1")
			mqueryURL, err := url.JoinPath(tp.conf.ConcMQueryServicePath, "concordance", fromCorp)
			if err != nil {
				uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
				return
			}

			req := *ctx.Request
			req.Header = req.Header.Clone() // this prevents concurrent access to headers (= a map)
			req.Method = "GET"
			fullURL := mqueryURL + "?" + urlQuery.Encode()
			parsedURL, err := url.Parse(fullURL)
			if err != nil {
				uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
				return
			}
			req.URL = parsedURL
			req.Body = io.NopCloser(strings.NewReader(""))
			customWriter := &customResponseWriter{
				body:   &bytes.Buffer{},
				header: make(http.Header),
			}
			tp.httpEngine.ServeHTTP(customWriter, &req)

			treqTranslation := treqExtTranslation{
				Word: trLine.To,
			}
			if customWriter.statusCode < 200 || customWriter.statusCode >= 300 {
				treqTranslation.Error = fmt.Sprintf(
					"failed to get translation examples for \u2039%s\u203A (status code %d)",
					trLine.To,
					customWriter.statusCode)
			}
			concResp := customWriter.body.Bytes()
			var concData concResponse
			if err := json.Unmarshal(concResp, &concData); err != nil {
				treqTranslation.Error = fmt.Sprintf(
					"failed to get translation examples for \u2039%s\u203A: %s",
					trLine.To,
					err.Error())
			}
			treqTranslation.Examples = &concExample{
				Text:          util.Ternary(treqTranslation.Error == "", concData.Lines, []concordance.Line{}),
				InteractionID: fmt.Sprintf("treqInteractionKey:%s", trLine.To),
			}
			ansEditLock.Lock()
			ans.Lines[lineIdx] = treqExtRespLine{
				Freq: trLine.Freq,
				Perc: trLine.Perc,
				From: trLine.From,
				To:   treqTranslation,
			}

			rawAns, err := json.Marshal(ans)
			ansEditLock.Unlock()

			if err != nil {
				log.Error().Err(err).Msg("failed to prepare EventSource data")
				return
			}

			_, err = fmt.Fprintf(
				ctx.Writer,
				"event:%s\ndata: %s\n\n", sseEvent, rawAns,
			)
			if err != nil {
				// not much we can do here
				log.Error().Err(err).Msg("failed to write EventSource data")
				return
			}

		}(translation, i)
	}
	wg.Wait()
	ctx.Writer.Flush()
}
