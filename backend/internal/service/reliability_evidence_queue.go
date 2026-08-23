package service

import (
	"context"
	"log"
	"runtime/debug"
	"time"
)

const (
	reliabilityEvidenceBatchSize   = 64
	reliabilityEvidenceBatchWindow = 100 * time.Millisecond
	reliabilityEvidenceTimeout     = 5 * time.Second
)

type reliabilityEvidenceJob struct {
	final      *ReliabilityFinalOutcome
	attempt    *ReliabilityAttemptOutcome
	probe      *ReliabilityProbeOutcome
	pendingID  uint64
	observedAt time.Time
}

func (s *ReliabilityEvidenceService) Name() string { return "reliability-evidence" }

func (s *ReliabilityEvidenceService) Start(context.Context) error {
	if s == nil || !s.Enabled() {
		return nil
	}
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	if s.queueStarted || s.queueStopping {
		return nil
	}
	if s.queueSize <= 0 {
		s.queueSize = 4096
	}
	if s.workerCount <= 0 {
		s.workerCount = 2
	}
	s.queue = make(chan reliabilityEvidenceJob, s.queueSize)
	s.queueStarted = true
	s.queueWG.Add(s.workerCount)
	for i := 0; i < s.workerCount; i++ {
		go s.runReliabilityEvidenceWorker(s.queue)
	}
	s.running.Store(true)
	return nil
}

func (s *ReliabilityEvidenceService) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.stopDependents()
	s.queueMu.Lock()
	if !s.queueStarted || s.queueStopping {
		s.queueMu.Unlock()
		return nil
	}
	s.queueStopping = true
	queue := s.queue
	s.queue = nil
	close(queue)
	s.queueMu.Unlock()
	done := make(chan struct{})
	go func() {
		s.queueWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		s.running.Store(false)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *ReliabilityEvidenceService) SubmitFinalOutcome(outcome *ReliabilityFinalOutcome) bool {
	return s.enqueueReliabilityEvidence(reliabilityEvidenceJob{final: outcome}, outcome != nil)
}

func (s *ReliabilityEvidenceService) SubmitAttemptOutcome(outcome *ReliabilityAttemptOutcome) bool {
	return s.enqueueReliabilityEvidence(reliabilityEvidenceJob{attempt: outcome}, outcome != nil)
}

func (s *ReliabilityEvidenceService) SubmitProbeOutcome(outcome *ReliabilityProbeOutcome) bool {
	return s.enqueueReliabilityEvidence(reliabilityEvidenceJob{probe: outcome}, outcome != nil)
}

func (s *ReliabilityEvidenceService) enqueueReliabilityEvidence(job reliabilityEvidenceJob, valid bool) bool {
	if s == nil || !valid || !s.Enabled() {
		return false
	}
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	if !s.queueStarted || s.queueStopping || s.queue == nil {
		return false
	}
	s.queueDepth.Add(1)
	job.pendingID = s.nextPendingID.Add(1)
	job.observedAt = reliabilityEvidenceJobObservedAt(job)
	s.trackPendingReliabilityEvidence(job)
	s.publishedPendingID.Store(job.pendingID)
	select {
	case s.queue <- job:
		s.enqueued.Add(1)
		return true
	default:
		s.queueDepth.Add(-1)
		s.dropped.Add(1)
		s.untrackPendingReliabilityEvidence([]reliabilityEvidenceJob{job})
		s.maybeLogReliabilityEvidenceDrop()
		return false
	}
}

func (s *ReliabilityEvidenceService) runReliabilityEvidenceWorker(queue <-chan reliabilityEvidenceJob) {
	defer s.queueWG.Done()
	for first := range queue {
		s.queueDepth.Add(-1)
		s.inFlight.Add(1)
		batch := []reliabilityEvidenceJob{first}
		timer := time.NewTimer(reliabilityEvidenceBatchWindow)
	collect:
		for len(batch) < reliabilityEvidenceBatchSize {
			select {
			case next, ok := <-queue:
				if !ok {
					break collect
				}
				s.queueDepth.Add(-1)
				s.inFlight.Add(1)
				batch = append(batch, next)
			case <-timer.C:
				break collect
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		s.flushReliabilityEvidenceBatch(batch)
		s.untrackPendingReliabilityEvidence(batch)
		s.inFlight.Add(-int64(len(batch)))
	}
}

func reliabilityEvidenceJobObservedAt(job reliabilityEvidenceJob) time.Time {
	switch {
	case job.final != nil && !job.final.ObservedAt.IsZero():
		return job.final.ObservedAt
	case job.attempt != nil && !job.attempt.ObservedAt.IsZero():
		return job.attempt.ObservedAt
	case job.probe != nil && !job.probe.ObservedAt.IsZero():
		return job.probe.ObservedAt
	default:
		return time.Now()
	}
}

func (s *ReliabilityEvidenceService) trackPendingReliabilityEvidence(job reliabilityEvidenceJob) {
	s.pendingMu.Lock()
	if s.pendingObserved == nil {
		s.pendingObserved = make(map[uint64]time.Time)
	}
	s.pendingObserved[job.pendingID] = job.observedAt
	s.pendingMu.Unlock()
}

func (s *ReliabilityEvidenceService) untrackPendingReliabilityEvidence(batch []reliabilityEvidenceJob) {
	s.pendingMu.Lock()
	for _, job := range batch {
		delete(s.pendingObserved, job.pendingID)
	}
	s.pendingMu.Unlock()
}

func (s *ReliabilityEvidenceService) flushReliabilityEvidenceBatch(batch []reliabilityEvidenceJob) {
	defer func() {
		if recovered := recover(); recovered != nil {
			s.failed.Add(int64(len(batch)))
			log.Printf("[ReliabilityEvidence] worker panic: %v\n%s", recovered, debug.Stack())
		}
	}()
	finals := make([]*ReliabilityFinalOutcome, 0, len(batch))
	attempts := make([]*ReliabilityAttemptOutcome, 0, len(batch))
	probes := make([]*ReliabilityProbeOutcome, 0, len(batch))
	for _, job := range batch {
		if job.final != nil {
			finals = append(finals, job.final)
		}
		if job.attempt != nil {
			attempts = append(attempts, job.attempt)
		}
		if job.probe != nil {
			probes = append(probes, job.probe)
		}
	}
	if len(finals) > 0 {
		s.flushReliabilityEvidenceOutcomes(int64(len(finals)), func(ctx context.Context) (int64, error) { return s.recordFinalOutcomes(ctx, finals) })
	}
	if len(attempts) > 0 {
		s.flushReliabilityEvidenceOutcomes(int64(len(attempts)), func(ctx context.Context) (int64, error) { return s.recordAttemptOutcomes(ctx, attempts) })
	}
	if len(probes) > 0 {
		s.flushReliabilityEvidenceOutcomes(int64(len(probes)), func(ctx context.Context) (int64, error) { return s.recordProbeOutcomes(ctx, probes) })
	}
}

func (s *ReliabilityEvidenceService) flushReliabilityEvidenceOutcomes(count int64, record func(context.Context) (int64, error)) {
	ctx, cancel := context.WithTimeout(context.Background(), reliabilityEvidenceTimeout)
	written, err := record(ctx)
	cancel()
	if err != nil {
		s.failed.Add(count)
		return
	}
	s.processed.Add(count)
	s.written.Add(written)
}

func (s *ReliabilityEvidenceService) maybeLogReliabilityEvidenceDrop() {
	now := time.Now().Unix()
	for {
		last := s.lastDropLogAt.Load()
		if last != 0 && now-last < 60 {
			return
		}
		if s.lastDropLogAt.CompareAndSwap(last, now) {
			break
		}
	}
	stats := s.Completeness()
	log.Printf(
		"[ReliabilityEvidence] queue full; dropping evidence (queued=%d in_flight=%d enqueued=%d dropped=%d processed=%d written=%d failed=%d)",
		stats.QueueDepth, stats.InFlight, stats.Enqueued, stats.Dropped, stats.Processed, stats.Written, stats.Failed,
	)
}
