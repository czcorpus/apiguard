// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package alarms

import (
	"fmt"
	"html/template"
	"net/http"
	"path"
	"runtime"

	"github.com/rs/zerolog/log"
)

func loadTemplateFile(relativePath string) (*template.Template, error) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "assets", relativePath)
	/*
		content, err := os.ReadFile(srcPath)
		if err != nil {
			return "", err
		}*/
	return template.New(relativePath).ParseFiles(srcPath)
}

type reportPageData struct {
	ConfirmationUrl string
	Error           error
	ReviewerMail    string
}

func (aticker *AlarmTicker) HandleConfirmationPage(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	tpl, err := loadTemplateFile("report.html")
	if err != nil {
		log.Error().Err(err).Send() // TODO
	}
	var data reportPageData
	alarmID := req.URL.Query().Get("id")
	data.ReviewerMail = req.URL.Query().Get("reviewer")
	var srchReport *AlarmReport
	for _, report := range aticker.reports {
		if report.ReviewCode == alarmID {
			srchReport = report
			data.ConfirmationUrl = aticker.createConfirmationURL(report, data.ReviewerMail)
			break
		}
	}
	if srchReport == nil {
		data.Error = fmt.Errorf("report ID %s not found", alarmID)
		log.Error().Err(err).Send() // TODO
	}
	err = tpl.ExecuteTemplate(w, "report.html", data)
}
