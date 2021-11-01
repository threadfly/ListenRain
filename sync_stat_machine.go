package listenrain

import (
	"errors"
	"log"
	"sync"
	"time"
)

type SyncStatMachine_Type uint8

const (
	SSM_INIT SyncStatMachine_Type = iota
	SSM_SUCC
	SSM_TIMEOUT
)

var (
	SSM_TIMEOUT_ERROR = errors.New("response timeout")
)

type SyncStatMachine struct {
	// TODO
	// Whether to use pipelines for synchronization brings less overhead
	// and needs to be benchmarked to verify
	sync.WaitGroup
	v     interface{}
	s     SyncStatMachine_Type
	start time.Time
}

func (ssm *SyncStatMachine) Process(msgId string, v interface{}) {
	ssm.v = v
	ssm.s = SSM_SUCC
	ssm.Done()
}

func (ssm *SyncStatMachine) Timeout(msgId string) {
	log.Printf("timeout msgId:%s begin:%s", msgId, ssm.start.String())
	ssm.s = SSM_TIMEOUT
	ssm.Done()
}

func (ssm *SyncStatMachine) Fire() {
	ssm.start = time.Now()
	ssm.Add(1)
}

func (ssm *SyncStatMachine) ShutDown() {
	ssm.Done()
}

func (ssm *SyncStatMachine) Return() (v interface{}, err error) {
	ssm.Wait()
	switch ssm.s {
	case SSM_INIT:
		return nil, errors.New("sync state machine is not being used correctly")
	case SSM_SUCC:
		v, err = ssm.v, nil
	case SSM_TIMEOUT:
		v, err = nil, SSM_TIMEOUT_ERROR
	default:
		return nil, errors.New("sync state machine is invalid stat")
	}

	ssm.v = nil
	ssm.s = SSM_INIT
	return
}
