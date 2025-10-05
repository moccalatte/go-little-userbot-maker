package usecase

import (
	"sync"
	"time"

	"go-little-userbot-maker/pkg/logger"
)

type Worker struct {
	telegramID int64
	record     SessionRecord
	log        *logger.Logger
	interval   time.Duration
	stopCh     chan struct{}
	done       sync.WaitGroup
}

func NewWorker(telegramID int64, record SessionRecord, log *logger.Logger, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Worker{
		telegramID: telegramID,
		record:     record,
		log:        log, // The logger is already specific to this telegramID
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

func (w *Worker) Start() {
	w.done.Add(1)
	go w.loop()
}

func (w *Worker) Stop() {
	close(w.stopCh)
	w.done.Wait()
}

func (w *Worker) NotifyFeature(feature string, payload map[string]any) {
	w.log.Info("feature '%s' updated with payload: %v", feature, payload)
}

func (w *Worker) loop() {
	defer w.done.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.log.Info("worker started")
	for {
		select {
		case <-w.stopCh:
			w.log.Info("worker stopping")
			return
		case <-ticker.C:
			w.log.Debug("heartbeat: status=alive")
		}
	}
}