package api

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/blacken57/heimdall/internal/db"
)

// serviceView is the template-facing representation of a service.
type serviceView struct {
	Name          string
	URL           string
	IsUp          bool
	StatusBadge   string
	UptimePct     string
	AvgResponseMs string
	LastChecked   string
	HasData       bool
	TotalChecks   string
	History       []segmentView
}

type segmentView struct {
	Color   string // "up", "down", or "no-data"
	Tooltip string // e.g. "Mar 01 · 99.5% · 288 checks"
}

func toView(s db.ServiceSummary) serviceView {
	v := serviceView{
		Name: s.Name,
		URL:  s.URL,
		IsUp: s.IsUp,
	}

	v.TotalChecks = fmt.Sprintf("%d checks", s.TotalChecks)
	v.History = make([]segmentView, len(s.History))
	for i, b := range s.History {
		label := b.Date.Format("Jan 02")
		if !b.HasData {
			v.History[i] = segmentView{Color: "no-data", Tooltip: label + " · no data"}
			continue
		}
		var pct float64
		if b.TotalChecks > 0 {
			pct = float64(b.UpChecks) / float64(b.TotalChecks) * 100
		}
		color := "down"
		if pct >= 90.0 {
			color = "up"
		}
		v.History[i] = segmentView{
			Color:   color,
			Tooltip: fmt.Sprintf("%s · %.1f%% · %d checks", label, pct, b.TotalChecks),
		}
	}

	if s.LastChecked.IsZero() {
		v.StatusBadge = "PENDING"
		v.UptimePct = "—"
		v.AvgResponseMs = "—"
		v.LastChecked = "never"
		return v
	}

	v.HasData = true

	if s.IsUp {
		v.StatusBadge = "UP"
	} else {
		v.StatusBadge = "DOWN"
	}

	v.UptimePct = fmt.Sprintf("%.1f%%", s.UptimePct)
	v.AvgResponseMs = fmt.Sprintf("%.0f ms", s.AvgResponseMs)

	diff := time.Since(s.LastChecked)
	switch {
	case diff < time.Minute:
		v.LastChecked = fmt.Sprintf("%ds ago", int(diff.Seconds()))
	case diff < time.Hour:
		v.LastChecked = fmt.Sprintf("%dm ago", int(diff.Minutes()))
	default:
		v.LastChecked = s.LastChecked.Format("Jan 2 15:04")
	}

	return v
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "ok")
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.ParseFiles(
		"web/templates/base.html",
		"web/templates/index.html",
		"web/templates/partials/service_list.html",
		"web/templates/partials/service_card.html",
	)
	if err != nil {
		log.Printf("parse templates: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	summaries, err := s.db.GetAllServiceSummaries()
	if err != nil {
		log.Printf("get summaries: %v", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	views := make([]serviceView, len(summaries))
	for i, sm := range summaries {
		views[i] = toView(sm)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", views); err != nil {
		log.Printf("execute template: %v", err)
	}
}

func (s *Server) handlePartialServices(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles(
		"web/templates/partials/service_list.html",
		"web/templates/partials/service_card.html",
	)
	if err != nil {
		log.Printf("parse partial templates: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	summaries, err := s.db.GetAllServiceSummaries()
	if err != nil {
		log.Printf("get summaries: %v", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	views := make([]serviceView, len(summaries))
	for i, sm := range summaries {
		views[i] = toView(sm)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "service_list", views); err != nil {
		log.Printf("execute partial template: %v", err)
	}
}
