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

package globctx

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/czcorpus/apiguard/common"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	lua "github.com/yuin/gopher-lua"
)

type ElasticOutputRecord struct {
	Service        string         `json:"service"`
	ActionType     string         `json:"actionType"`
	ProcTime       float64        `json:"procTime"`
	IsCached       bool           `json:"isCached"`
	FirstPartyCall bool           `json:"isFirstPartyCall"`
	UserID         common.UserID  `json:"userId,omitempty"`
	IPAddress      string         `json:"ipAddress,omitempty"`
	UserAgent      string         `json:"userAgent,omitempty"`
	RequestPath    string         `json:"requestPath,omitempty"`
	Args           map[string]any `json:"args,omitempty"`
	CountryName    string         `json:"countryName,omitempty"`
	Latitude       float32        `json:"latitude,omitempty"`
	Longitude      float32        `json:"longitude,omitempty"`
	Timezone       string         `json:"timezone,omitempty"`
	Time           time.Time      `json:"time"`
}

// SetLocation sets the geographical location data for the record.
func (r *ElasticOutputRecord) SetLocation(countryName string, latitude float32, longitude float32, timezone string) {
	r.CountryName = countryName
	r.Latitude = latitude
	r.Longitude = longitude
	r.Timezone = timezone
}

// ToJSON creates an object suitable for storing to ElasticSearch, CouchDB and other
// document-oriented databases.
func (r *ElasticOutputRecord) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// GetID creates an idempotent unique identifier of the record.
// This can be typically accomplished by hashing the original log record.
// This generates a random UUID-based ID for uniqueness.
func (r *ElasticOutputRecord) GetID() string {
	id := uuid.New()
	sum := sha1.New()
	_, err := sum.Write([]byte(id.String()))
	if err != nil {
		log.Error().Err(err).Msg("problem generating hash")
		return ""
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// GenerateDeterministicID generates the same ID for the same record properties.
// This is essential for easier update in the Elasticsearch archive.
func (r *ElasticOutputRecord) GenerateDeterministicID() string {
	// Create a deterministic hash from all record fields
	data := fmt.Sprintf("%s|%s|%f|%v|%v|%s|%f|%f|%s|%s|%s",
		r.Service,
		r.ActionType,
		r.ProcTime,
		r.IsCached,
		r.FirstPartyCall,
		r.IPAddress,
		r.Latitude,
		r.Longitude,
		r.Timezone,
		r.CountryName,
		r.Time.String(),
	)

	sum := sha1.New()
	_, err := sum.Write([]byte(data))
	if err != nil {
		log.Error().Err(err).Msg("problem generating deterministic hash")
		return ""
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// GetType returns the app type as defined by an external convention.
// Examples: kontext, syd, morfio, treq, apiguard, apiguard-kontext, etc.
func (r *ElasticOutputRecord) GetType() string {
	if r.Service != "" {
		return r.Service
	}
	return "apiguard"
}

// GetTime returns the time of the log record.
func (r *ElasticOutputRecord) GetTime() time.Time {
	return r.Time
}

// SetTime sets the time of the log record (used for Lua scripting).
func (r *ElasticOutputRecord) SetTime(t time.Time) {
	r.Time = t
}

func (r *ElasticOutputRecord) LSetProperty(name string, value lua.LValue) error {
	return fmt.Errorf("scripting not supported")
}
