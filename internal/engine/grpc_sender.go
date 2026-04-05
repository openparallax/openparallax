package engine

import (
	pb "github.com/openparallax/openparallax/internal/types/pb"
)

// grpcEventSender adapts the gRPC stream to the EventSender interface.
type grpcEventSender struct {
	stream pb.ClientService_SendMessageServer
}

func newGRPCEventSender(stream pb.ClientService_SendMessageServer) EventSender {
	return &grpcEventSender{stream: stream}
}

func (g *grpcEventSender) SendEvent(event *PipelineEvent) error {
	pbEvent := &pb.PipelineEvent{
		SessionId: event.SessionID,
		MessageId: event.MessageID,
	}

	switch event.Type {
	case EventLLMToken:
		pbEvent.EventType = pb.PipelineEventType_LLM_TOKEN
		pbEvent.LlmToken = &pb.LLMToken{Text: event.LLMToken.Text}

	case EventActionStarted:
		pbEvent.EventType = pb.PipelineEventType_ACTION_STARTED
		pbEvent.ActionStarted = &pb.ActionStarted{Summary: event.ActionStarted.Summary}

	case EventShieldVerdict:
		pbEvent.EventType = pb.PipelineEventType_SHIELD_VERDICT
		pbEvent.ShieldVerdict = &pb.ShieldVerdict{
			Decision:   verdictStringToProto(event.ShieldVerdict.Decision),
			Tier:       int32(event.ShieldVerdict.Tier),
			Confidence: event.ShieldVerdict.Confidence,
			Reasoning:  event.ShieldVerdict.Reasoning,
		}

	case EventActionCompleted:
		pbEvent.EventType = pb.PipelineEventType_ACTION_COMPLETED
		pbEvent.ActionCompleted = &pb.ActionCompleted{
			Success: event.ActionCompleted.Success,
			Summary: event.ActionCompleted.Summary,
		}

	case EventResponseComplete:
		pbEvent.EventType = pb.PipelineEventType_RESPONSE_COMPLETE
		pbEvent.ResponseComplete = &pb.ResponseComplete{Content: event.ResponseComplete.Content}

	case EventOTRBlocked:
		pbEvent.EventType = pb.PipelineEventType_OTR_BLOCKED
		pbEvent.OtrBlocked = &pb.OTRBlocked{Reason: event.OTRBlocked.Reason}

	case EventError:
		pbEvent.EventType = pb.PipelineEventType_ERROR
		pbEvent.PipelineError = &pb.PipelineError{
			Code: event.Error.Code, Message: event.Error.Message,
			Recoverable: event.Error.Recoverable,
		}
	}

	return g.stream.Send(pbEvent)
}

func verdictStringToProto(d string) pb.VerdictDecision {
	switch d {
	case "ALLOW":
		return pb.VerdictDecision_ALLOW
	case "BLOCK":
		return pb.VerdictDecision_BLOCK
	case "ESCALATE":
		return pb.VerdictDecision_ESCALATE
	default:
		return pb.VerdictDecision_VERDICT_DECISION_UNSPECIFIED
	}
}
