// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package proxy

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type PublicAPIProxy struct {
	InternalURL string
	ExternalURL string
	client      *http.Client
	basicProxy  APIProxy
}

func (proxy *PublicAPIProxy) AnyPath(ctx *gin.Context) {

}

func NewPublicAPIProxy() *PublicAPIProxy {
	return &PublicAPIProxy{}
}
