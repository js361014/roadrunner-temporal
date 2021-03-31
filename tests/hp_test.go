package tests

import (
	"context"
	"crypto/rand"
	"crypto/sha512"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.temporal.io/api/common/v1"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
)

func init() { //nolint:gochecknoinits
	color.NoColor = false
}

type User struct {
	Name  string
	Email string
}

func Test_VerifyRegistration(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	assert.Contains(t, s.workflows.WorkflowNames(), "SimpleWorkflow")

	assert.Contains(t, s.activities.ActivityNames(), "SimpleActivity.echo")
	assert.Contains(t, s.activities.ActivityNames(), "HeartBeatActivity.doSomething")

	assert.Contains(t, s.activities.ActivityNames(), "SimpleActivity.lower")
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ExecuteSimpleWorkflow_1(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ExecuteSimpleDTOWorkflow(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleDTOWorkflow",
		User{
			Name:  "Antony",
			Email: "email@world.net",
		},
	)
	assert.NoError(t, err)

	var result struct{ Message string }
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "Hello Antony <email@world.net>", result.Message)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ExecuteSimpleWorkflowWithSequenceInBatch(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"WorkflowWithSequence",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "OK", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_MultipleWorkflowsInSingleWorker(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	w2, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD", result)

	assert.NoError(t, w2.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ExecuteWorkflowWithParallelScopes(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"ParallelScopesWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD|Hello World|hello world", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_Timer(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	start := time.Now()
	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
	assert.True(t, time.Since(start).Seconds() > 1)

	s.AssertContainsEvent(t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_TIMER_STARTED
	})
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SideEffect(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SideEffectWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Contains(t, result, "hello world-")

	s.AssertContainsEvent(t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_MARKER_RECORDED
	})
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_EmptyWorkflow(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"EmptyWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result int
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, 42, result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_PromiseChaining(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"ChainedWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "result:hello world", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ActivityHeartbeat(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleHeartbeatWorkflow",
		2,
	)
	assert.NoError(t, err)

	time.Sleep(time.Second)

	we, err := s.Client().DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)
	assert.Len(t, we.PendingActivities, 1)

	act := we.PendingActivities[0]

	assert.Equal(t, `{"value":2}`, string(act.HeartbeatDetails.Payloads[0].Data))

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "OK", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_BinaryPayload(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	rnd := make([]byte, 2500)

	_, err := rand.Read(rnd)
	assert.NoError(t, err)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"BinaryWorkflow",
		rnd,
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))

	assert.Equal(t, fmt.Sprintf("%x", sha512.Sum512(rnd)), result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ContinueAsNew(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"ContinuableWorkflow",
		1,
	)
	assert.NoError(t, err)

	time.Sleep(time.Second)

	we, err := s.Client().DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	assert.Equal(t, "ContinuedAsNew", we.WorkflowExecutionInfo.Status.String())

	time.Sleep(time.Second)

	// the result of the final workflow
	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "OK6", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ActivityStubWorkflow(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"ActivityStubWorkflow",
		"hello world",
	)
	assert.NoError(t, err)

	// the result of the final workflow
	var result []string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, []string{
		"HELLO WORLD",
		"invalid method call",
		"UNTYPED",
	}, result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ExecuteProtoWorkflow(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"ProtoPayloadWorkflow",
	)
	assert.NoError(t, err)

	var result common.WorkflowExecution
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "updated", result.RunId)
	assert.Equal(t, "workflow id", result.WorkflowId)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SagaWorkflow(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, false)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SagaWorkflow",
	)
	assert.NoError(t, err)

	var result string
	assert.Error(t, w.Get(context.Background(), &result))
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_VerifyRegistrationProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	assert.Contains(t, s.workflows.WorkflowNames(), "SimpleWorkflow")

	assert.Contains(t, s.activities.ActivityNames(), "SimpleActivity.echo")
	assert.Contains(t, s.activities.ActivityNames(), "HeartBeatActivity.doSomething")

	assert.Contains(t, s.activities.ActivityNames(), "SimpleActivity.lower")
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ExecuteSimpleWorkflow_1Proto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ExecuteSimpleDTOWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleDTOWorkflow",
		User{
			Name:  "Antony",
			Email: "email@world.net",
		},
	)
	assert.NoError(t, err)

	var result struct{ Message string }
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "Hello Antony <email@world.net>", result.Message)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ExecuteSimpleWorkflowWithSequenceInBatchProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"WorkflowWithSequence",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "OK", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_MultipleWorkflowsInSingleWorkerProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	w2, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD", result)

	assert.NoError(t, w2.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ExecuteWorkflowWithParallelScopesProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"ParallelScopesWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "HELLO WORLD|Hello World|hello world", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_TimerProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	start := time.Now()
	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"TimerWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "hello world", result)
	assert.True(t, time.Since(start).Seconds() > 1)

	s.AssertContainsEvent(t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_TIMER_STARTED
	})
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SideEffectProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SideEffectWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Contains(t, result, "hello world-")

	s.AssertContainsEvent(t, w, func(event *history.HistoryEvent) bool {
		return event.EventType == enums.EVENT_TYPE_MARKER_RECORDED
	})
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_EmptyWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"EmptyWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result int
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, 42, result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_PromiseChainingProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"ChainedWorkflow",
		"Hello World",
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "result:hello world", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ActivityHeartbeatProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SimpleHeartbeatWorkflow",
		2,
	)
	assert.NoError(t, err)

	time.Sleep(time.Second)

	we, err := s.Client().DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)
	assert.Len(t, we.PendingActivities, 1)

	act := we.PendingActivities[0]

	assert.Equal(t, `{"value":2}`, string(act.HeartbeatDetails.Payloads[0].Data))

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "OK", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_BinaryPayloadProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	rnd := make([]byte, 2500)

	_, err := rand.Read(rnd)
	assert.NoError(t, err)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"BinaryWorkflow",
		rnd,
	)
	assert.NoError(t, err)

	var result string
	assert.NoError(t, w.Get(context.Background(), &result))

	assert.Equal(t, fmt.Sprintf("%x", sha512.Sum512(rnd)), result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ContinueAsNewProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"ContinuableWorkflow",
		1,
	)
	assert.NoError(t, err)

	time.Sleep(time.Second)

	we, err := s.Client().DescribeWorkflowExecution(context.Background(), w.GetID(), w.GetRunID())
	assert.NoError(t, err)

	assert.Equal(t, "ContinuedAsNew", we.WorkflowExecutionInfo.Status.String())

	time.Sleep(time.Second)

	// the result of the final workflow
	var result string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "OK6", result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ActivityStubWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"ActivityStubWorkflow",
		"hello world",
	)
	assert.NoError(t, err)

	// the result of the final workflow
	var result []string
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, []string{
		"HELLO WORLD",
		"invalid method call",
		"UNTYPED",
	}, result)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_ExecuteProtoWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"ProtoPayloadWorkflow",
	)
	assert.NoError(t, err)

	var result common.WorkflowExecution
	assert.NoError(t, w.Get(context.Background(), &result))
	assert.Equal(t, "updated", result.RunId)
	assert.Equal(t, "workflow id", result.WorkflowId)
	stopCh <- struct{}{}
	wg.Wait()
}

func Test_SagaWorkflowProto(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	s := NewTestServer(t, stopCh, wg, true)

	w, err := s.Client().ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			TaskQueue: "default",
		},
		"SagaWorkflow",
	)
	assert.NoError(t, err)

	var result string
	assert.Error(t, w.Get(context.Background(), &result))
	stopCh <- struct{}{}
	wg.Wait()
}
