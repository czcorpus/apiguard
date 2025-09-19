// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cnc

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/czcorpus/apiguard-common/common"
	"github.com/czcorpus/apiguard-common/reporting"
	"github.com/czcorpus/apiguard/guard"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type loginResponse struct {
	code    int
	message string
	cookies []*http.Cookie
	err     error
}

func (resp loginResponse) Cookies() []*http.Cookie {
	return resp.cookies
}

func (resp loginResponse) Cookie(name string) *http.Cookie {
	for _, c := range resp.cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (resp loginResponse) Status() int {
	return resp.code
}

// String provides an informative overview about the value
func (resp loginResponse) String() string {
	var ans strings.Builder
	ans.WriteString(fmt.Sprintf("loginResponse[status: %d, cookies: ", resp.code))
	for i, c := range resp.cookies {
		if i > 0 {
			ans.WriteString(", ")
		}
		ans.WriteString(c.Name)
	}
	if resp.err != nil {
		ans.WriteString(fmt.Sprintf(", err: %s", resp.err.Error()))
	}
	ans.WriteString("]")
	return ans.String()
}

func (resp loginResponse) Err() error {
	return resp.err
}

func (resp loginResponse) isInvalidCredentials() bool {
	return resp.message == "Invalid credentials"
}

func (kp *Proxy) LoginWithToken(token string) loginResponse {
	postData := url.Values{}
	postData.Set(kp.rConf.AuthTokenEntry, token)
	req2, err := http.NewRequest(
		http.MethodPost,
		kp.rConf.CNCPortalLoginURL,
		strings.NewReader(postData.Encode()),
	)
	if err != nil {
		return loginResponse{
			code: http.StatusInternalServerError,
			err:  fmt.Errorf("failed to perform login action: %w", err),
		}
	}
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // only for internal network communication
		},
	}
	client := &http.Client{Transport: transport}

	resp, err := client.Do(req2)
	if err != nil {
		return loginResponse{
			code: resp.StatusCode,
			err:  fmt.Errorf("failed to perform login action: %w", err),
		}
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return loginResponse{
			code: http.StatusInternalServerError,
			err:  fmt.Errorf("failed to perform login action: %w", err),
		}
	}
	defer resp.Body.Close()
	respMsg := make([]string, 0, 1)
	err = json.Unmarshal(body, &respMsg)
	if err != nil {
		return loginResponse{
			code: http.StatusInternalServerError,
			err:  fmt.Errorf("failed to perform login action: %w", err),
		}
	}
	return loginResponse{
		code:    http.StatusOK,
		message: respMsg[0],
		cookies: resp.Cookies(),
	}
}

// loginFromCtx performs an HTTP request with
// CNC login based on current ctx (where we're
// interested mainly in user's request properties).
func (kp *Proxy) loginFromCtx(ctx *gin.Context) loginResponse {
	return kp.LoginWithToken(ctx.Request.FormValue(kp.rConf.AuthTokenEntry))
}

// applyLoginRespCookies applies respose cookies as obtained
// from a login request. The method does not care whether the
// response represents a successful login or not.
// The method also tries to find matching user ID and sets
func (kp *Proxy) applyLoginRespCookies(
	ctx *gin.Context,
	resp loginResponse,
) (userID common.UserID) {
	for _, cookie := range resp.cookies {
		cCopy := *cookie
		if cCopy.Name == kp.rConf.CNCAuthCookie && kp.conf.FrontendSessionCookieName != "" {
			var err error
			sessionID := kp.sessionValFactory().UpdatedFrom(cCopy.Value)
			userID, err = guard.FindUserBySession(kp.globalCtx.CNCDB, sessionID)
			if err != nil {
				log.Error().Err(err).Msg("Failed to obtain user ID after successful. Ignoring.")
			}
			cCopy.Name = kp.conf.FrontendSessionCookieName
			log.Debug().
				Str("backendCookie", kp.rConf.CNCAuthCookie).
				Str("frontendCookie", kp.conf.FrontendSessionCookieName).
				Str("value", cCopy.Value).
				Msg("login action - mapping back internal cookie")
		}
		http.SetCookie(ctx.Writer, &cCopy)
	}
	return
}

// Login is a custom Proxy for CNC portals' central login action.
// We use it in situations where we need a "hidden" API login using
// a special user account for unauthorized users. Without additional
// action, such CNC login would still return standard cookies and made
// hidden user visible. So our proxy action will rename a respective
// cookie and will allow a custom web application (e.g. WaG)
// to use this special cookie.
func (kp *Proxy) Login(ctx *gin.Context) {
	t0 := time.Now().In(kp.globalCtx.TimezoneLocation)
	userId := common.InvalidUserID

	defer func(currUserID *common.UserID) {
		kp.globalCtx.BackendLoggers.Get(kp.EnvironConf().ServiceKey).Log(
			ctx.Request,
			kp.rConf.ServiceKey,
			time.Since(t0),
			false,
			*currUserID,
			true,
			reporting.BackendActionTypeLogin,
		)
	}(&userId)

	resp := kp.loginFromCtx(ctx)
	if resp.err != nil {
		log.Error().Err(resp.err).Msgf("failed to perform login")
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionErrorFrom(resp.err), resp.code)
		return
	}

	if resp.isInvalidCredentials() {
		log.Error().Err(fmt.Errorf("invalid credentials")).Msgf("failed to perform login")
		uniresp.WriteCustomJSONErrorResponse(ctx.Writer, resp.message, http.StatusUnauthorized)
		return
	}

	userId = kp.applyLoginRespCookies(ctx, resp)
	log.Debug().Str("userId", userId.String()).Msg("user authenticated via CNC auth")

	uniresp.WriteJSONResponse(ctx.Writer, resp.message)
}
