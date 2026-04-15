package workers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/images"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/Arush71/jobqueue/internal/store"
	"github.com/disintegration/imaging"
)

func DoWork(q *queue.Queue, dbQ *db.Queries) {
	for {
		job_id := q.GetWork()
		log.Println("worker picked job: id =", job_id)

		img, format, err := images.GetDecocdedImage(job.ImagePath)
		if err != nil {
			log.Println("error worker: couldn't either open or decode the image.", job.JobId)
			if err := q.UpdateJob(job.JobId, jobs.Fail); err != nil {
				log.Printf("[CRITICAL] invariant violation: job %d not found during update to state %s", job.JobId, job.State)
			}
			continue
		}
		proccessedImg := img
		quality := 100
		switch job.JobType {
		case jobs.Compress:
			quality = int(job.Params["quantity"])
		case jobs.GrayScale:
			proccessedImg = imaging.Grayscale(img)
		case jobs.Resize:
			proccessedImg = imaging.Resize(img, int(job.Params["width"]), int(job.Params["height"]), imaging.Lanczos)
		}
		_, err = images.SaveImage(proccessedImg, format, job.ImagePath, quality)
		if err != nil {
			log.Println("error worker: couldn't save image.", job.JobId)
			if err := q.UpdateJob(job.JobId, jobs.Fail); err != nil {
				log.Printf("[CRITICAL] invariant violation: job %d not found during update to state %s", job.JobId, job.State)
			}
			continue

		}
		if err := q.UpdateJob(job.JobId, jobs.Success); err != nil {
			log.Printf("[CRITICAL] invariant violation: job %d not found during update to state %s", job.JobId, job.State)
			continue
		}
		log.Println("Job successfull", job.JobId)
	}
}

func GetJobFromId(job_id int64, dbQ *db.Queries) (jobs.Job, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	job, err := dbQ.GetJobById(ctx, job_id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("Important: worker picked id that's not available in db, id:%d", job_id)
			return jobs.Job{}, fmt.Errorf("id with such row does not exist")
		}
		log.Printf("Important: db error, fail to fetch job with id: %d", job_id)
		return jobs.Job{}, fmt.Errorf(err.Error()+"id of %d", job_id)
	}
	parms, err := store.FromDBParams(job.Params)
	if err != nil {
		// TODO: change stage to fail.
		log.Printf("Parms failed for id %d", job_id)
		return jobs.Job{}, fmt.Errorf("invalid parms for id %d :", job_id)
	}
	return jobs.Job{
		JobId:     job.ID,
		State:     jobs.JobState(job.State),
		ImagePath: job.ImagePath,
		JobType:   jobs.JT(job.Type),
		Params:    parms,
	}, nil
}

// TODO: work on the doWork functions make it work with db, fix the errors.
