package checker

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/blacken57/heimdall/internal/config"
	"github.com/blacken57/heimdall/internal/db"
)

type Checker struct {
	cfg        *config.Config
	db         *db.DB
	serviceIDs map[string]int64 // name → db id
	client     *http.Client
}

func New(cfg *config.Config, database *db.DB, serviceIDs map[string]int64) *Checker {
	return &Checker{
		cfg:        cfg,
		db:         database,
		serviceIDs: serviceIDs,
		client: &http.Client{
			Timeout: time.Duration(cfg.HTTPTimeout) * time.Second,
			// Don't follow redirects blindly; treat 3xx as up.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Run starts one goroutine per service and blocks until ctx is cancelled.
func (c *Checker) Run(ctx context.Context) {
	for _, svc := range c.cfg.Services {
		svc := svc // capture for goroutine
		go c.runService(ctx, svc)
	}
	<-ctx.Done()
}

func (c *Checker) runService(ctx context.Context, svc config.Service) {
	id, ok := c.serviceIDs[svc.Name]
	if !ok {
		log.Printf("checker: unknown service %q — skipping", svc.Name)
		return
	}

	// Do an immediate first check before waiting for the ticker.
	c.poll(svc, id)

	ticker := time.NewTicker(time.Duration(c.cfg.PollInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.poll(svc, id)
		}
	}
}

func (c *Checker) poll(svc config.Service, id int64) {
	start := time.Now()
	statusCode := 0
	isUp := false
	errMsg := ""

	resp, err := c.client.Get(svc.URL)
	responseMs := int(time.Since(start).Milliseconds())

	if err != nil {
		errMsg = err.Error()
	} else {
		resp.Body.Close()
		statusCode = resp.StatusCode
		isUp = statusCode >= 200 && statusCode < 400
	}

	if dbErr := c.db.InsertCheck(id, statusCode, responseMs, isUp, errMsg); dbErr != nil {
		log.Printf("checker: insert check for %q: %v", svc.Name, dbErr)
	}

	status := "UP"
	if !isUp {
		status = fmt.Sprintf("DOWN (code=%d)", statusCode)
		if errMsg != "" {
			status = fmt.Sprintf("DOWN (%s)", errMsg)
		}
	}
	log.Printf("checked %s [%s] %dms %s", svc.Name, svc.URL, responseMs, status)
}
