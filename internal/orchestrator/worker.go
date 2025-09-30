package orchestrator

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

type Worker struct {
	telegramID int64
	record     SessionRecord
	log        *zap.Logger
	interval   time.Duration
	stopCh     chan struct{}
	done       sync.WaitGroup
}

func NewWorker(telegramID int64, record SessionRecord, log *zap.Logger, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Worker{
		telegramID: telegramID,
		record:     record,
		log:        log.With(zap.Int64("telegram_id", telegramID)),
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
	w.log.Info("feature updated", zap.String("feature", feature), zap.Any("payload", payload))
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
			w.log.Debug("heartbeat", zap.String("status", "alive"))
		}
	}
}
