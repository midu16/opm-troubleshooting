package datasource

import (
	"github.com/midu16/opm-troubleshooting/internal/mustgather"
)

// OperatorFromMustGather converts a mustgather.OperatorState to datasource.OperatorState.
func OperatorFromMustGather(op mustgather.OperatorState) OperatorState {
	ds := OperatorState{
		PackageName:      op.PackageName,
		Namespace:        op.Namespace,
		Channel:          op.Channel,
		InstalledCSV:     op.InstalledCSV,
		CurrentCSV:       op.CurrentCSV,
		InstalledVersion: op.InstalledVersion,
		State:            op.State,
		InstallPlanRef:   op.InstallPlanRef,
		Faulty:           op.Faulty,
		FailureReason:    op.FailureReason,
	}
	for _, c := range op.Conditions {
		ds.Conditions = append(ds.Conditions, Condition{
			Type:               c.Type,
			Status:             c.Status,
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime,
		})
	}
	if op.RootCause != nil {
		ds.RootCause = &RootCauseDetail{
			MissingCRDs:         op.RootCause.MissingCRDs,
			MissingAPIs:         op.RootCause.MissingAPIs,
			UnknownResources:    op.RootCause.UnknownResources,
			NotPresentResources: op.RootCause.NotPresentResources,
			PodErrors:           op.RootCause.PodErrors,
			RawFailureMessage:   op.RootCause.RawFailureMessage,
		}
	}
	return ds
}

// OperatorToMustGather converts a datasource.OperatorState back to mustgather.OperatorState
// for compatibility with existing analysis pipeline.
func OperatorToMustGather(ds OperatorState) mustgather.OperatorState {
	op := mustgather.OperatorState{
		PackageName:      ds.PackageName,
		Namespace:        ds.Namespace,
		Channel:          ds.Channel,
		InstalledCSV:     ds.InstalledCSV,
		CurrentCSV:       ds.CurrentCSV,
		InstalledVersion: ds.InstalledVersion,
		State:            ds.State,
		InstallPlanRef:   ds.InstallPlanRef,
		Faulty:           ds.Faulty,
		FailureReason:    ds.FailureReason,
	}
	for _, c := range ds.Conditions {
		op.Conditions = append(op.Conditions, mustgather.Condition{
			Type:               c.Type,
			Status:             c.Status,
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime,
		})
	}
	if ds.RootCause != nil {
		op.RootCause = &mustgather.RootCauseDetail{
			MissingCRDs:         ds.RootCause.MissingCRDs,
			MissingAPIs:         ds.RootCause.MissingAPIs,
			UnknownResources:    ds.RootCause.UnknownResources,
			NotPresentResources: ds.RootCause.NotPresentResources,
			PodErrors:           ds.RootCause.PodErrors,
			RawFailureMessage:   ds.RootCause.RawFailureMessage,
		}
	}
	return op
}

// DeploymentFromMustGather converts a mustgather.DeploymentState to datasource.DeploymentState.
func DeploymentFromMustGather(d mustgather.DeploymentState) DeploymentState {
	return DeploymentState{
		Name:           d.Name,
		Replicas:       d.Replicas,
		ReadyReplicas:  d.ReadyReplicas,
		Available:      d.Available,
		Progressing:    d.Progressing,
		ProgressingMsg: d.ProgressingMsg,
		UnavailableMsg: d.UnavailableMsg,
	}
}

// PodFromMustGather converts a mustgather.PodState to datasource.PodState.
func PodFromMustGather(p mustgather.PodState) PodState {
	return PodState{
		Name:             p.Name,
		Phase:            p.Phase,
		Ready:            p.Ready,
		RestartCount:     p.RestartCount,
		WaitingReason:    p.WaitingReason,
		WaitingMessage:   p.WaitingMessage,
		TerminatedReason: p.TerminatedReason,
	}
}

// EventFromMustGather converts a mustgather.EventState to datasource.EventState.
func EventFromMustGather(e mustgather.EventState) EventState {
	return EventState{
		Type:    e.Type,
		Reason:  e.Reason,
		Message: e.Message,
		Object:  e.Object,
	}
}
