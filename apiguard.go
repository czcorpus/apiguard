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

package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/czcorpus/apiguard-common/proxy"

	proxyImpl "github.com/czcorpus/apiguard/proxy"
	"github.com/czcorpus/apiguard/server"

	"github.com/czcorpus/apiguard/guard/token"
	"github.com/czcorpus/apiguard/services"

	"github.com/czcorpus/cnc-gokit/datetime"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var (
	defaultConfigPath string
	version           string
	buildDate         string
	gitCommit         string
	versionInfo       = services.VersionInfo{
		Version:   version,
		BuildDate: buildDate,
		GitCommit: gitCommit,
	}
)

type CmdOptions struct {
	Host              string
	Port              int
	ReadTimeoutSecs   int
	WriteTimeoutSecs  int
	LogPath           string
	LogLevel          string
	MaxAgeDays        int
	BanDurationStr    string
	IgnoreStoredState bool
	StreamingMode     bool
}

func (opts CmdOptions) BanDuration() (time.Duration, error) {
	// we test for '0' as the parser below does not like
	// numbers without suffix ('d', 'h', 's', ...)
	if opts.BanDurationStr == "" || opts.BanDurationStr == "0" {
		return 0, nil
	}
	return datetime.ParseDuration(opts.BanDurationStr)
}

func init() {
	if defaultConfigPath == "" {
		defaultConfigPath = "/usr/local/etc/apiguard.json"
	}
}

func init() {
	gob.Register(&proxy.BackendSimpleResponse{})
	gob.Register(&proxyImpl.BackendProxiedResponse{})
}

func determineConfigPath(argPos int) string {
	v := flag.Arg(argPos)
	if v != "" {
		return v
	}
	fmt.Fprintf(os.Stderr, "using default config in %s\n", defaultConfigPath)
	return defaultConfigPath
}

func main() {

	cmdOpts := new(CmdOptions)
	flag.StringVar(&cmdOpts.Host, "host", "", "Host to listen on")
	flag.IntVar(&cmdOpts.Port, "port", 0, "Port to listen on")
	flag.BoolVar(&cmdOpts.StreamingMode, "streaming-mode", false, "If used, APIGuard will run in the streaming mode no matter what is defined in its config JSON")
	flag.IntVar(&cmdOpts.ReadTimeoutSecs, "read-timeout", 0, "Server read timeout in seconds")
	flag.IntVar(&cmdOpts.ReadTimeoutSecs, "write-timeout", 0, "Server write timeout in seconds")
	flag.StringVar(&cmdOpts.LogPath, "log-path", "", "A file to log to (if empty then stderr is used)")
	flag.StringVar(&cmdOpts.LogLevel, "log-level", "", "A log level (debug, info, warn/warning, error)")
	flag.IntVar(&cmdOpts.MaxAgeDays, "max-age-days", 0, "When cleaning old records, this specifies the oldes records (in days) to keep in database.")
	flag.StringVar(&cmdOpts.BanDurationStr, "ban-duration", "0", "A duration for the ban (e.g. 90s, 2d, 8h30m)")
	flag.BoolVar(&cmdOpts.IgnoreStoredState, "ignore-stored-state", false, "If used then no alarm state will be loaded from a configured location. This is usefull e.g. in case of an application configuration change.")

	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"apiguard - CNC API protection and response data polishing"+
				"\n\nUsage:"+
				"\n\t%s [options] start [conf.json]"+
				"\n\t%s [options] status [session id / IP address] [conf.json]"+
				"\n\t%s [options] learn [conf.json]"+
				"\n\t%s generate-token"+
				"\n\t%s [options] version\n",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	action := flag.Arg(0)

	switch action {
	case "version":
		fmt.Printf("CNC APIGuard %s\nbuild date: %s\nlast commit: %s\n",
			versionInfo.Version, versionInfo.BuildDate, versionInfo.GitCommit)
		return
	case "start":
		conf := findAndLoadConfig(determineConfigPath(1), cmdOpts)
		log.Info().
			Str("version", versionInfo.Version).
			Str("buildDate", versionInfo.BuildDate).
			Str("last commit", versionInfo.GitCommit).
			Msg("Starting CNC APIGuard")

		server.RunService(conf)
	case "generate-token":
		id := uuid.New()
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			fmt.Println("failed to generate token: ", err)
			os.Exit(1)
			return
		}
		tk := base64.URLEncoding.EncodeToString(append([]byte(id.String()), bytes...))
		var tkJS token.TokenConf
		tkJS.HashedValue = fmt.Sprintf("%x", sha256.Sum256([]byte(tk)))
		tkJS.UserID = 1
		fmt.Println("token: ", tk)
		var jsonOut strings.Builder
		mrs := json.NewEncoder(&jsonOut)
		mrs.SetIndent("", "  ")
		err := mrs.Encode(tkJS)
		if err != nil {
			fmt.Println("failed to generate token: ", err)
			os.Exit(1)
			return
		}
		fmt.Printf("conf:\n%s", &jsonOut)
		return

	default:
		fmt.Printf("Unknown action [%s]. Try -h for help\n", flag.Arg(0))
		os.Exit(1)
	}

}
