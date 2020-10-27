package workflow

import (
	"encoding/json"
	"github.com/spiral/endure/errors"
	rrt "github.com/temporalio/roadrunner-temporal"
	"go.temporal.io/api/common/v1"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/workflow"
	"time"
)

const (
	StartWorkflowCommand    = "StartWorkflow"
	ExecuteActivityCommand  = "ExecuteActivity"
	NewTimerCommand         = "NewTimer"
	CompleteWorkflowCommand = "CompleteWorkflow"
)

// StartWorkflow sends worker command to start workflow.
type StartWorkflow struct {
	Name      string            `json:"name"`
	Wid       string            `json:"wid"`
	Rid       string            `json:"rid"`
	TaskQueue string            `json:"taskQueue"`
	Input     []json.RawMessage `json:"args"`
}

// FromEnvironment maps start command from environment.
func (start *StartWorkflow) FromEnvironment(env internalbindings.WorkflowEnvironment, input *common.Payloads) error {
	info := env.WorkflowInfo()

	start.Name = info.WorkflowType.Name
	start.Wid = info.WorkflowExecution.ID
	start.Rid = info.WorkflowExecution.RunID
	start.TaskQueue = info.TaskQueueName

	return rrt.FromPayload(env.GetDataConverter(), input, &start.Input)
}

// ExecuteActivity command by workflow worker.
type ExecuteActivity struct {
	// Name defines activity name.
	Name string `json:"name"`

	// Args to pass to the activity.
	Args []json.RawMessage `json:"arguments"`

	// Options to run activity as.
	Options workflow.ActivityOptions `json:"options,omitempty"`

	// ArgsPayload represents Args converted into Temporal payload format.
	ArgsPayload *commonpb.Payloads
}

// NewTimer starts new timer.
type NewTimer struct {
	// Milliseconds defines timer duration.
	Milliseconds int `json:"ms"`
}

// ToDuration converts timer command to time.Duration.
func (t NewTimer) ToDuration() time.Duration {
	return time.Millisecond * time.Duration(t.Milliseconds)
}

// CompleteWorkflow sent by worker to complete workflow.
type CompleteWorkflow struct {
	// Result defines workflow execution result.
	Result []json.RawMessage `json:"result"`

	// ResultPayload represents Result converted into Temporal payload format.
	ResultPayload *commonpb.Payloads
}

// maps worker parameters into internal command representation.
func parseCommand(dc converter.DataConverter, name string, params json.RawMessage) (interface{}, error) {
	switch name {
	case ExecuteActivityCommand:
		cmd := ExecuteActivity{}

		if err := json.Unmarshal(params, &cmd); err != nil {
			return nil, err
		}

		if err := rrt.ToPayload(dc, cmd.Args, cmd.ArgsPayload); err != nil {
			return nil, err
		}

		return cmd, nil

	case NewTimerCommand:
		cmd := NewTimer{}
		if err := json.Unmarshal(params, &cmd); err != nil {
			return nil, err
		}

		return cmd, nil

	case CompleteWorkflowCommand:
		cmd := CompleteWorkflow{}
		if err := json.Unmarshal(params, &cmd); err != nil {
			return nil, err
		}

		if err := rrt.ToPayload(dc, cmd.Result, cmd.ResultPayload); err != nil {
			return nil, err
		}

		return cmd, nil

		// todo: map other commands

	default:
		return nil, errors.E(errors.Op("parseCommand"), "undefined command type", errors.Str(name))
	}
}
