package service

import (
	"sort"
	"time"
)

type ServiceStatus string

const (
	ServiceStatusOperational         ServiceStatus = "operational"
	ServiceStatusDegradedPerformance ServiceStatus = "degraded_performance"
	ServiceStatusPartialOutage       ServiceStatus = "partial_outage"
	ServiceStatusMajorOutage         ServiceStatus = "major_outage"
	ServiceStatusMaintenance         ServiceStatus = "maintenance"
	ServiceStatusMonitoring          ServiceStatus = "monitoring"
)

type StatusObservation struct {
	Kind           ReliabilityFactType
	Outcome        ReliabilityOutcome
	CustomerImpact bool
	ObservedAt     time.Time
}

type StatusEvaluationInput struct {
	Now                   time.Time
	Freshness             time.Duration
	PreviousStatus        ServiceStatus
	MonitoringSince       time.Time
	RecoveryConfirmedAt   time.Time
	CriticalService       bool
	TotalCustomerRequests int
	Observations          []StatusObservation
}

type StatusEvaluation struct {
	Status               ServiceStatus
	Reason               string
	CustomerRequestCount int
	CustomerSuccessCount int
	CustomerFailureCount int
	ProbeCount           int
	ProbeSuccessCount    int
	ProbeFailureCount    int
	MonitoringResetAt    time.Time
	RecoveryConfirmedAt  time.Time
}

func EvaluateComputedStatus(input StatusEvaluationInput) StatusEvaluation {
	freshness := input.Freshness
	if freshness <= 0 {
		freshness = 5 * time.Minute
	}
	latest := time.Time{}
	fresh := make([]StatusObservation, 0, len(input.Observations))
	for _, observation := range input.Observations {
		if observation.Outcome == ReliabilityOutcomeExcluded || observation.ObservedAt.After(input.Now) || input.Now.Sub(observation.ObservedAt) > freshness {
			continue
		}
		if observation.Kind != ReliabilityFactCustomerRequest && observation.Kind != ReliabilityFactActiveProbe {
			continue
		}
		if observation.ObservedAt.After(latest) {
			latest = observation.ObservedAt
		}
		fresh = append(fresh, observation)
	}
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].ObservedAt.Before(fresh[j].ObservedAt) })
	customerRequests := 0
	customerSuccesses := 0
	customerFailures := 0
	probeCount := 0
	probeSuccesses := 0
	probeFailures := 0
	consecutiveCustomerFailures := 0
	consecutiveProbeFailures := 0
	consecutiveProbeSuccesses := 0
	probeRecoveryConfirmedAt := time.Time{}
	lastCustomerAt := time.Time{}
	lastProbeAt := time.Time{}
	for _, observation := range fresh {
		success := observation.Outcome == ReliabilityOutcomeSuccess || observation.Outcome == ReliabilityOutcomeRecovered
		switch observation.Kind {
		case ReliabilityFactCustomerRequest:
			lastCustomerAt = observation.ObservedAt
			customerRequests++
			if observation.Outcome == ReliabilityOutcomeFailure && observation.CustomerImpact {
				consecutiveProbeSuccesses = 0
				probeRecoveryConfirmedAt = time.Time{}
				customerFailures++
				consecutiveCustomerFailures++
			} else {
				if success {
					customerSuccesses++
				}
				consecutiveCustomerFailures = 0
			}
		case ReliabilityFactActiveProbe:
			lastProbeAt = observation.ObservedAt
			probeCount++
			if observation.Outcome == ReliabilityOutcomeFailure {
				consecutiveProbeSuccesses = 0
				probeRecoveryConfirmedAt = time.Time{}
				probeFailures++
				consecutiveProbeFailures++
			} else {
				if success {
					consecutiveProbeSuccesses++
					if consecutiveProbeSuccesses == 2 {
						probeRecoveryConfirmedAt = observation.ObservedAt
					}
					probeSuccesses++
				}
				consecutiveProbeFailures = 0
			}
		}
	}
	result := func(status ServiceStatus, reason string) StatusEvaluation {
		return StatusEvaluation{
			Status: status, Reason: reason,
			CustomerRequestCount: customerRequests, CustomerSuccessCount: customerSuccesses, CustomerFailureCount: customerFailures,
			ProbeCount: probeCount, ProbeSuccessCount: probeSuccesses, ProbeFailureCount: probeFailures,
		}
	}
	if latest.IsZero() {
		return result(ServiceStatusMonitoring, "stale_evidence")
	}

	customerEvidenceSparse := customerRequests < 3
	useProbeFailures := customerEvidenceSparse && (customerRequests == 0 || lastProbeAt.After(lastCustomerAt))
	failureStatus := ServiceStatusOperational
	failureReason := ""
	if input.CriticalService && customerRequests >= 3 && customerFailures == customerRequests && consecutiveCustomerFailures == customerRequests {
		failureStatus, failureReason = ServiceStatusMajorOutage, "broad_critical_loss"
	} else if consecutiveCustomerFailures >= 3 || (useProbeFailures && consecutiveProbeFailures >= 3) {
		failureStatus, failureReason = ServiceStatusPartialOutage, "customer_impact"
		if consecutiveCustomerFailures < 3 {
			failureReason = "probe_impact"
		}
	} else if consecutiveCustomerFailures == 2 || (useProbeFailures && consecutiveProbeFailures == 2) {
		failureStatus, failureReason = ServiceStatusDegradedPerformance, "repeated_customer_impact"
		if consecutiveCustomerFailures != 2 {
			failureReason = "repeated_probe_impact"
		}
	}

	// Fresh failures may escalate an already unhealthy state. Confirmed probe
	// recovery still wins when it occurs after the last failure because failures
	// reset consecutiveProbeSuccesses above.
	if input.PreviousStatus == ServiceStatusDegradedPerformance || input.PreviousStatus == ServiceStatusPartialOutage || input.PreviousStatus == ServiceStatusMajorOutage {
		if consecutiveProbeSuccesses >= 2 {
			evaluation := result(ServiceStatusMonitoring, "recovery_observed")
			evaluation.RecoveryConfirmedAt = probeRecoveryConfirmedAt
			return evaluation
		}
		if statusSeverity(failureStatus) > statusSeverity(input.PreviousStatus) {
			return result(failureStatus, failureReason)
		}
		return result(input.PreviousStatus, "recovery_unconfirmed")
	}
	if input.PreviousStatus == ServiceStatusMonitoring {
		relapseObservations := make([]StatusObservation, 0, len(fresh))
		postMonitoringProbeSuccesses := 0
		postMonitoringCustomerRequests := 0
		postMonitoringCustomerFailures := 0
		postMonitoringProbeFailures := 0
		postMonitoringLastCustomerAt := time.Time{}
		postMonitoringLastProbeAt := time.Time{}
		latestRelapseFailureAt := time.Time{}
		relapseRecoveryConfirmedAt := time.Time{}
		for _, observation := range fresh {
			if !input.MonitoringSince.IsZero() && !observation.ObservedAt.After(input.MonitoringSince) {
				continue
			}
			relapseObservations = append(relapseObservations, observation)
			if observation.Kind == ReliabilityFactCustomerRequest {
				postMonitoringCustomerRequests++
				postMonitoringLastCustomerAt = observation.ObservedAt
				if observation.Outcome == ReliabilityOutcomeFailure && observation.CustomerImpact {
					postMonitoringCustomerFailures++
					postMonitoringProbeSuccesses = 0
					relapseRecoveryConfirmedAt = time.Time{}
					latestRelapseFailureAt = observation.ObservedAt
				}
			}
			if observation.Kind == ReliabilityFactActiveProbe {
				postMonitoringLastProbeAt = observation.ObservedAt
				switch observation.Outcome {
				case ReliabilityOutcomeFailure:
					postMonitoringProbeFailures++
					postMonitoringProbeSuccesses = 0
					relapseRecoveryConfirmedAt = time.Time{}
					latestRelapseFailureAt = observation.ObservedAt
				case ReliabilityOutcomeSuccess, ReliabilityOutcomeRecovered:
					postMonitoringProbeSuccesses++
					if postMonitoringProbeSuccesses == 2 {
						relapseRecoveryConfirmedAt = observation.ObservedAt
					}
				}
			}
		}
		relapse := EvaluateComputedStatus(StatusEvaluationInput{Now: input.Now, Freshness: freshness, CriticalService: input.CriticalService, Observations: relapseObservations})
		probeRelapse := postMonitoringCustomerRequests < 3 && (postMonitoringCustomerRequests == 0 || postMonitoringLastProbeAt.After(postMonitoringLastCustomerAt)) && postMonitoringProbeFailures > 0
		if relapseRecoveryConfirmedAt.IsZero() && statusSeverity(relapse.Status) >= statusSeverity(ServiceStatusDegradedPerformance) {
			return result(relapse.Status, relapse.Reason)
		}
		if postMonitoringCustomerFailures > 0 || probeRelapse {
			evaluation := result(ServiceStatusMonitoring, "monitoring_relapse")
			evaluation.MonitoringResetAt = latestRelapseFailureAt
			if !relapseRecoveryConfirmedAt.IsZero() {
				evaluation.MonitoringResetAt = relapseRecoveryConfirmedAt
				evaluation.RecoveryConfirmedAt = relapseRecoveryConfirmedAt
			}
			return evaluation
		}
		if input.RecoveryConfirmedAt.IsZero() && !relapseRecoveryConfirmedAt.IsZero() {
			evaluation := result(ServiceStatusMonitoring, "recovery_observed")
			evaluation.MonitoringResetAt = relapseRecoveryConfirmedAt
			evaluation.RecoveryConfirmedAt = relapseRecoveryConfirmedAt
			return evaluation
		}
		if !input.RecoveryConfirmedAt.IsZero() && input.Now.Sub(input.RecoveryConfirmedAt) >= 10*time.Minute {
			return result(ServiceStatusOperational, "monitoring_complete")
		}
		evaluation := result(ServiceStatusMonitoring, "monitoring_observation")
		evaluation.RecoveryConfirmedAt = input.RecoveryConfirmedAt
		return evaluation
	}
	if statusSeverity(failureStatus) >= statusSeverity(ServiceStatusDegradedPerformance) {
		return result(failureStatus, failureReason)
	}
	if consecutiveCustomerFailures == 1 {
		return result(ServiceStatusOperational, "isolated_failure")
	}

	// Customer final outcomes have precedence. Probes only back availability
	// when no trailing customer-impacting failure is present.
	if useProbeFailures && consecutiveProbeFailures == 1 {
		return result(ServiceStatusOperational, "isolated_probe_failure")
	}
	if !customerEvidenceSparse {
		return result(ServiceStatusOperational, "fresh_customer_evidence")
	}
	return result(ServiceStatusOperational, "fresh_success")
}
