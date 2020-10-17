package ecs

const WORKER_ID_RANDOM int32 = -1

//Worker goroutine struct.
type Worker struct {
	runtime *Runtime
	id       int32
	p        *Pool
	jobQueue chan *Job
	stop     chan struct{}
}

//Start start goroutine pool.
func (w *Worker) Start() {
	go func() {
		var job *Job
		for {
			select {
			case job = <-w.jobQueue:
			case job = <-w.p.jobQueue:
				//task which worker id not nil will push into the target goroutine to insure data safety
				if job.WorkerID != WORKER_ID_RANDOM {
					if job.WorkerID >= 0 && job.WorkerID < w.p.numWorkers {
						w.p.workerQueue[job.WorkerID].jobQueue <- job
						continue
					}
				}
			case <-w.stop:
				return
			}
			//TODO handle Error
			ctx:=&JobContext{
				WorkerID: job.WorkerID,
				Runtime: w.runtime,
			}
			job.Job(ctx, job.Args...)
			w.p.jobPool.Put(job.Init())
		}
	}()
}
