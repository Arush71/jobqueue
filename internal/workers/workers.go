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
		job, err := GetJobFromId(job_id, dbQ)
		if err != nil {
			log.Printf("Error from worker: %s", err.Error())
			continue
		}
		err = ManageJobImageProccessingWithContext(job)
		if err != nil {
			log.Println(err.Error())
			if err := ChangeJobStateFromId(job.JobId, dbQ, jobs.Fail); err != nil {
				log.Printf("[CRITICAL] invariant violation: job %d not found during update to state %s", job.JobId, job.State)
			}
			continue
		}
		if err := ChangeJobStateFromId(job.JobId, dbQ, jobs.Success); err != nil {
			log.Printf("[CRITICAL] invariant violation: job %d not found during update to state %s", job.JobId, job.State)
		}
		log.Println("Job successfull", job.JobId)
	}
}

func GetJobFromId(job_id int64, dbQ *db.Queries) (jobs.Job, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	job, err := dbQ.GetJobIfQueued(ctx, job_id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("Important: worker picked id that's not available in db or is already taken id:%d", job_id)
			return jobs.Job{}, fmt.Errorf("%d not claimable", job_id)
		}
		log.Printf("Important: db error, fail to fetch job with id: %d", job_id)
		return jobs.Job{}, fmt.Errorf("%s id of %d", err.Error(), job_id)
	}
	parms, err := store.FromDBParams(job.Params)
	if err != nil {
		log.Printf("Parms failed for id %d", job_id)
		err := ChangeJobStateFromId(job_id, dbQ, jobs.Fail)
		if err != nil {
			log.Printf("Important: Failed to set the job state to failure after data corruption for id: %d .", job_id)
		}
		return jobs.Job{}, fmt.Errorf("invalid parms for id %d ", job_id)
	}
	return jobs.Job{
		JobId:     job.ID,
		State:     jobs.JobState(job.State),
		ImagePath: job.ImagePath,
		JobType:   jobs.JT(job.Type),
		Params:    parms,
	}, nil
}

func ChangeJobStateFromId(job_id int64, dbQ *db.Queries, state jobs.JobState) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := dbQ.UpdateJobId(ctx, db.UpdateJobIdParams{
		ID:    job_id,
		State: db.JobState(state),
	})
	if err != nil {
		log.Printf("Important: Job state to %s failed for id %d", state, job_id)
		return err
	}
	return nil
}

func ManageJobImageProccessing(job jobs.Job, done chan error) {
	img, format, err := images.GetDecocdedImage(job.ImagePath)
	if err != nil {
		done <- fmt.Errorf("error worker: couldn't either open or decode the image, for id: %d", job.JobId)
		return
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
		done <- fmt.Errorf("error worker: couldn't save image. job id: %d", job.JobId)
		return
	}
	done <- nil
}

func ManageJobImageProccessingWithContext(job jobs.Job) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go ManageJobImageProccessing(job, done)
	select {
	case result := <-done:
		return result
	case <-ctx.Done():
		return fmt.Errorf("the image processing hung")
	}
}
