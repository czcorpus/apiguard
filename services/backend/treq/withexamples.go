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
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/mquery-common/concordance"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

	serviceResp := tp.ProxyRequest(
		"/",
		req.URL.Query(),
		req.Method,
		req.Header,
		req.Body,
	)
	cached = serviceResp.IsCached()
	tp.WriteReport(&reporting.ProxyProcReport{
		DateTime: time.Now().In(tp.GlobalCtx().TimezoneLocation),
		ProcTime: time.Since(rt0).Seconds(),
		Status:   serviceResp.GetStatusCode(),
		Service:  tp.EnvironConf().ServiceKey,
		IsCached: cached,
	})
	if serviceResp.GetError() != nil {
		log.Error().Err(serviceResp.GetError()).Msgf("failed to proxy request %s", ctx.Request.URL.Path)
		http.Error(
			ctx.Writer,
			fmt.Sprintf("failed to proxy request: %s", serviceResp.GetError()),
			http.StatusInternalServerError,
		)
		return
	}

	defer serviceResp.CloseBodyReader()
	translatResp, err := io.ReadAll(serviceResp.GetBodyReader())
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

	ans := treqExtResponse{
		Sum:      translations.Sum,
		Lines:    make([]treqExtRespLine, len(translations.Lines)),
		FromCorp: fromCorp,
		ToCorp:   toCorp,
	}

	for i, translation := range translations.Lines {
		cql := fmt.Sprintf(
			`[word=="%s"] within %s:[word=="%s"]`,
			strings.ReplaceAll(translation.From, "\"", "\\\""),
			toCorp,
			strings.ReplaceAll(translation.To, "\"", "\\\""),
		)
		urlQuery := make(url.Values)
		urlQuery.Add("q", cql)
		urlQuery.Add("maxRows", "2") // TODO configurable
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

		concResp := customWriter.body.Bytes()
		var concData concResponse
		if err := json.Unmarshal(concResp, &concData); err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		fmt.Println("concData.Lines: ", concData.Lines)
		ans.Lines[i] = treqExtRespLine{
			Freq: translation.Freq,
			Perc: translation.Perc,
			From: translation.From,
			To: treqExtTranslation{
				Word: translation.To,
				Examples: &concExample{
					Text:          concData.Lines,
					InteractionID: uuid.New().String(),
				},
			},
		}

	}

	data, err := json.Marshal(ans)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	// TODO vvvvvvvvvvvvvvvvvvvvvvvv
	for k, v := range serviceResp.GetHeaders() {
		ctx.Writer.Header().Add(k, v[0]) // TODO duplicated headers for content-type
	}
	ctx.Writer.WriteHeader(serviceResp.GetStatusCode())
	ctx.Writer.Write(data)

}
