package main

type Work struct {
	workFunc   func(interface{}) error
	workInput  interface{}
	resultChan chan error
}

type WorkPool struct {
	inputChan chan Work
}

func NewWorkPool(threads int) *WorkPool {
	w := &WorkPool{
		inputChan: make(chan Work, threads),
	}
	w.startWorkers(threads)
	return w
}

func (w *WorkPool) startWorkers(threads int) {
	for i := 0; i < threads; i++ {
		go func() {
			for w := range w.inputChan {
				err := w.workFunc(w.workInput)
				w.resultChan <- err
			}
		}()
	}
}

func (w *WorkPool) doWork(work func(interface{}) error, input interface{}, resultChan chan error) {
	go func() {
		w.inputChan <- Work{workInput: input, workFunc: work, resultChan: resultChan}
	}()
}
