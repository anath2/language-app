package discovery

import (
	"context"
	"log"
	"time"
)

type Scheduler struct {
	pipeline *Pipeline
	interval time.Duration
	stop     chan struct{}
}

func NewScheduler(pipeline *Pipeline, intervalHours int) *Scheduler {
	return &Scheduler{
		pipeline: pipeline,
		interval: time.Duration(intervalHours) * time.Hour,
		stop:     make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	go s.run()
}

func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) run() {
	log.Printf("discovery scheduler started: interval=%s", s.interval)

	// Fire once on startup
	s.tick()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.tick()
		case <-s.stop:
			log.Printf("discovery scheduler stopped")
			return
		}
	}
}

func (s *Scheduler) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Printf("discovery scheduled run starting")
	if err := s.pipeline.Run(ctx, "scheduled"); err != nil {
		log.Printf("discovery scheduled run failed: %v", err)
	} else {
		log.Printf("discovery scheduled run completed")
	}
}
