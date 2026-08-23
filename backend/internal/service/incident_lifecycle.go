package service

import (
	"fmt"
	"sort"
	"time"
)

type IncidentPhase string

const (
	IncidentPhaseInvestigating IncidentPhase = "investigating"
	IncidentPhaseIdentified    IncidentPhase = "identified"
	IncidentPhaseMitigating    IncidentPhase = "mitigating"
	IncidentPhaseMonitoring    IncidentPhase = "monitoring"
	IncidentPhaseResolved      IncidentPhase = "resolved"

	incidentHealthyObservationWindow = 10 * time.Minute
)

func (p IncidentPhase) Valid() bool {
	switch p {
	case IncidentPhaseInvestigating, IncidentPhaseIdentified, IncidentPhaseMitigating, IncidentPhaseMonitoring, IncidentPhaseResolved:
		return true
	default:
		return false
	}
}

type IncidentCustomerFailure struct {
	ObservationID int64
	ObservedAt    time.Time
}

type IncidentImpactSegment struct {
	ID                 int64
	StartedAt          time.Time
	EndedAt            *time.Time
	StartObservationID int64
	EndObservationID   int64
}

type IncidentProductLifecycle struct {
	ID                    int64
	ProductID             int64
	CurrentStatus         ServiceStatus
	MonitoringSince       time.Time
	RecoveredAt           time.Time
	LastCustomerFailureAt time.Time
	Segments              []IncidentImpactSegment
}

type IncidentLifecycleState struct {
	Phase                  IncidentPhase
	MonitoringSince        time.Time
	ResolvedAt             *time.Time
	Products               []IncidentProductLifecycle
	Relapsed               bool
	CustomerImpactRelapsed bool
	EvidenceGap            bool
}

type IncidentProductSignal struct {
	ProductID        int64
	Status           ServiceStatus
	MonitoringSince  time.Time
	RecoveryObserved *IncidentCustomerFailure
	CustomerFailures []IncidentCustomerFailure
}

type IncidentLifecycleInput struct {
	Now      time.Time
	Products []IncidentProductSignal
}

func EvaluateIncidentLifecycle(state IncidentLifecycleState, input IncidentLifecycleInput) (IncidentLifecycleState, error) {
	if !state.Phase.Valid() || input.Now.IsZero() {
		return state, fmt.Errorf("incident lifecycle state is invalid")
	}
	if state.Phase == IncidentPhaseResolved {
		return state, nil
	}
	next := cloneIncidentLifecycleState(state)
	byProduct := make(map[int64]*IncidentProductLifecycle, len(next.Products))
	for index := range next.Products {
		byProduct[next.Products[index].ProductID] = &next.Products[index]
	}
	for _, signal := range input.Products {
		if signal.ProductID <= 0 || !statusValidForIncident(signal.Status) {
			return state, fmt.Errorf("incident product signal is invalid")
		}
		product := byProduct[signal.ProductID]
		if product == nil {
			next.Products = append(next.Products, IncidentProductLifecycle{ProductID: signal.ProductID, CurrentStatus: signal.Status})
			product = &next.Products[len(next.Products)-1]
			byProduct[signal.ProductID] = product
		}
		previouslyMonitoring := !product.MonitoringSince.IsZero()
		product.CurrentStatus = signal.Status
		if incidentStatusIsCustomerAbnormal(signal.Status) {
			if previouslyMonitoring {
				next.Relapsed = true
				next.Phase = IncidentPhaseInvestigating
				next.MonitoringSince = time.Time{}
			}
			sort.Slice(signal.CustomerFailures, func(i, j int) bool {
				return signal.CustomerFailures[i].ObservedAt.Before(signal.CustomerFailures[j].ObservedAt)
			})
			if len(signal.CustomerFailures) > 0 {
				first := signal.CustomerFailures[0]
				last := signal.CustomerFailures[len(signal.CustomerFailures)-1]
				if !hasOpenIncidentSegment(product.Segments) {
					product.Segments = append(product.Segments, IncidentImpactSegment{StartedAt: first.ObservedAt, StartObservationID: first.ObservationID})
				}
				product.LastCustomerFailureAt = last.ObservedAt
				if previouslyMonitoring {
					next.CustomerImpactRelapsed = true
				}
			}
			product.MonitoringSince = time.Time{}
			product.RecoveredAt = time.Time{}
			continue
		}

		if signal.Status == ServiceStatusMonitoring || signal.Status == ServiceStatusOperational {
			monitoringSince := signal.MonitoringSince
			if monitoringSince.IsZero() {
				monitoringSince = input.Now
			}
			closeIncidentProductSegment(product, monitoringSince, signal.RecoveryObserved)
			product.MonitoringSince = monitoringSince
			if signal.Status == ServiceStatusOperational {
				product.RecoveredAt = input.Now
			}
		}
	}
	sort.Slice(next.Products, func(i, j int) bool { return next.Products[i].ProductID < next.Products[j].ProductID })

	allObserved, allRecovered := len(next.Products) > 0, len(next.Products) > 0
	latestMonitoring := time.Time{}
	for _, product := range next.Products {
		if product.CurrentStatus != ServiceStatusMonitoring && product.CurrentStatus != ServiceStatusOperational {
			allObserved = false
		}
		if product.CurrentStatus != ServiceStatusOperational {
			allRecovered = false
		}
		if hasOpenIncidentSegment(product.Segments) {
			allObserved = false
			allRecovered = false
		}
		if product.MonitoringSince.After(latestMonitoring) {
			latestMonitoring = product.MonitoringSince
		}
	}
	if allObserved {
		next.Phase = IncidentPhaseMonitoring
		if next.MonitoringSince.IsZero() || latestMonitoring.After(next.MonitoringSince) {
			next.MonitoringSince = latestMonitoring
		}
	}
	if !next.EvidenceGap && allRecovered && !next.MonitoringSince.IsZero() && !input.Now.Before(next.MonitoringSince.Add(incidentHealthyObservationWindow)) {
		next.Phase = IncidentPhaseResolved
		resolvedAt := input.Now
		next.ResolvedAt = &resolvedAt
	}
	return next, nil
}

func ValidateIncidentPhaseTransition(state IncidentLifecycleState, target IncidentPhase, now time.Time) error {
	if !state.Phase.Valid() || !target.Valid() || now.IsZero() || state.Phase == target || state.Phase == IncidentPhaseResolved {
		return fmt.Errorf("incident phase transition is invalid")
	}
	allowed := false
	switch state.Phase {
	case IncidentPhaseInvestigating:
		allowed = target == IncidentPhaseIdentified || target == IncidentPhaseMitigating || target == IncidentPhaseMonitoring
	case IncidentPhaseIdentified:
		allowed = target == IncidentPhaseMitigating || target == IncidentPhaseMonitoring
	case IncidentPhaseMitigating:
		allowed = target == IncidentPhaseMonitoring
	case IncidentPhaseMonitoring:
		allowed = target == IncidentPhaseInvestigating || target == IncidentPhaseIdentified || target == IncidentPhaseMitigating
		if target == IncidentPhaseResolved {
			allowed = !state.EvidenceGap && !state.MonitoringSince.IsZero() && !now.Before(state.MonitoringSince.Add(incidentHealthyObservationWindow)) && !incidentStateHasOpenSegments(state) && incidentStateProductsOperational(state)
		}
	}
	if target == IncidentPhaseMonitoring && (!incidentStateProductsObserved(state) || incidentStateHasOpenSegments(state)) {
		allowed = false
	}
	if !allowed {
		return fmt.Errorf("incident phase transition from %s to %s is not allowed", state.Phase, target)
	}
	return nil
}

func incidentStateProductsOperational(state IncidentLifecycleState) bool {
	if len(state.Products) == 0 {
		return false
	}
	for _, product := range state.Products {
		if product.CurrentStatus != ServiceStatusOperational {
			return false
		}
	}
	return true
}

func incidentStateProductsObserved(state IncidentLifecycleState) bool {
	for _, product := range state.Products {
		if product.CurrentStatus != ServiceStatusMonitoring && product.CurrentStatus != ServiceStatusOperational {
			return false
		}
	}
	return true
}

func IncidentCompensableDuration(product IncidentProductLifecycle) time.Duration {
	var total time.Duration
	for _, segment := range product.Segments {
		if segment.EndedAt != nil && segment.EndedAt.After(segment.StartedAt) {
			total += segment.EndedAt.Sub(segment.StartedAt)
		}
	}
	return total
}

func incidentStatusIsCustomerAbnormal(status ServiceStatus) bool {
	return status == ServiceStatusDegradedPerformance || status == ServiceStatusPartialOutage || status == ServiceStatusMajorOutage
}

func statusValidForIncident(status ServiceStatus) bool {
	switch status {
	case ServiceStatusOperational, ServiceStatusDegradedPerformance, ServiceStatusPartialOutage, ServiceStatusMajorOutage, ServiceStatusMaintenance, ServiceStatusMonitoring:
		return true
	default:
		return false
	}
}

func hasOpenIncidentSegment(segments []IncidentImpactSegment) bool {
	for _, segment := range segments {
		if segment.EndedAt == nil {
			return true
		}
	}
	return false
}

func closeIncidentProductSegment(product *IncidentProductLifecycle, at time.Time, recovery *IncidentCustomerFailure) {
	if product == nil {
		return
	}
	for index := len(product.Segments) - 1; index >= 0; index-- {
		segment := &product.Segments[index]
		if segment.EndedAt != nil {
			continue
		}
		if at.Before(segment.StartedAt) {
			at = segment.StartedAt
		}
		segment.EndedAt = &at
		if recovery != nil {
			segment.EndObservationID = recovery.ObservationID
		}
		return
	}
}

func incidentStateHasOpenSegments(state IncidentLifecycleState) bool {
	for _, product := range state.Products {
		if hasOpenIncidentSegment(product.Segments) {
			return true
		}
	}
	return false
}

func cloneIncidentLifecycleState(state IncidentLifecycleState) IncidentLifecycleState {
	next := state
	next.Products = append([]IncidentProductLifecycle(nil), state.Products...)
	for index := range next.Products {
		next.Products[index].Segments = append([]IncidentImpactSegment(nil), state.Products[index].Segments...)
	}
	return next
}
