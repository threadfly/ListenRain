package listenrain

type DefaultExecutor struct {
}

func (e *DefaultExecutor) Process(r processRunner, payload []byte) {
	go func() {
		r.Process(payload)
	}()
}

func (e *DefaultExecutor) Timeout(r timeoutRunner, msgId string) {
	go func() {
		r.Timeout(msgId)
	}()
}

func DefaultExecutorGenerator(key TransportKey) (Executor, error) {
	return &DefaultExecutor{}, nil
}
