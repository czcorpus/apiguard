// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package monitoring

import (
	"apiguard/guard"
	"fmt"
	"html/template"
	"path"
	"runtime"

	"github.com/gin-gonic/gin"
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
	Report          *AlarmReport
	Ban             *guard.UserBan
}

func (aticker *AlarmTicker) HandleConfirmationPage(ctx *gin.Context) {
	ctx.Writer.Header().Add("Content-Type", "text/html")
	tpl, err := loadTemplateFile("report.html")
	if err != nil {
		log.Error().Err(err).Send() // TODO
	}
	var data reportPageData
	alarmID := ctx.Request.URL.Query().Get("id")
	data.ReviewerMail = ctx.Request.URL.Query().Get("reviewer")
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
		log.Error().Err(data.Error).Send()

	} else {
		ban, err := guard.FindBanByReport(
			aticker.ctx.CNCDB,
			aticker.location,
			alarmID,
		)
		data.Error = err
		data.Ban = ban
		data.Report = srchReport
	}
	err = tpl.ExecuteTemplate(ctx.Writer, "report.html", data)
	if err != nil {
		log.Error().Err(err).Msg("Failed to render HTML template")
	}
}
