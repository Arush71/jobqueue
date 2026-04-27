package queue

type Request interface {
	execute(q *Queue)
}
type AddReq struct {
	JobId int64
}

func (a AddReq) execute(q *Queue) {
	q.qS = append(q.qS, a.JobId)
	select {
	case q.notifyCh <- struct{}{}:
	default:
	}
}

type GetQueueJob struct {
	Job_id int64
	OK     bool
}
type GetWorkS struct {
	SendChan chan<- GetQueueJob
}

func (g GetWorkS) execute(q *Queue) {
	len_job := len(q.qS)
	if len_job == 0 {
		g.SendChan <- GetQueueJob{
			OK: false,
		}
		return
	}
	job_id := q.qS[0]
	q.qS = q.qS[1:]
	g.SendChan <- GetQueueJob{
		OK:     true,
		Job_id: job_id,
	}
}
