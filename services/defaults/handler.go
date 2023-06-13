// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package defaults

import (
	"errors"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

var (
	errNoSuchService = errors.New("no such service")
)

type DefaultsProvider interface {
	SetDefault(req *http.Request, key, value string) error
	GetDefault(req *http.Request, key string) (string, error)
	GetDefaults(req *http.Request) (Args, error)
}

type Actions struct {
	defaultsProviders map[string]DefaultsProvider
}

func (a *Actions) findService(name string) (DefaultsProvider, error) {
	s, ok := a.defaultsProviders[name]
	if !ok {
		return nil, errNoSuchService
	}
	return s, nil
}

func (a *Actions) Set(ctx *gin.Context) {
	service, err := a.findService(ctx.Param("serviceID"))
	if err == errNoSuchService {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusNotFound)
		return
	}
	service.SetDefault(ctx.Request, ctx.Param("key"), ctx.Request.URL.Query().Get("value"))
	uniresp.WriteJSONResponse(ctx.Writer, map[string]bool{"ok": true})
}

func (a *Actions) Get(ctx *gin.Context) {
	service, err := a.findService(ctx.Param("serviceID"))
	if err == errNoSuchService {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusNotFound)
		return
	}
	val, err := service.GetDefault(ctx.Request, ctx.Param("key"))
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusNotFound)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, map[string]string{ctx.Param("key"): val})
}

func (a *Actions) GetAll(ctx *gin.Context) {
	service, err := a.findService(ctx.Param("serviceID"))
	if err == errNoSuchService {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusNotFound)
		return
	}
	val, err := service.GetDefault(ctx.Request, ctx.Param("key"))
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionErrorFrom(err), http.StatusNotFound)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, map[string]string{ctx.Param("key"): val})
}

func NewActions(defaultsProviders map[string]DefaultsProvider) *Actions {
	return &Actions{
		defaultsProviders: make(map[string]DefaultsProvider),
	}
}
