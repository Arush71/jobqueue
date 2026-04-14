package workers

import (
	"log"

	"github.com/Arush71/jobqueue/internal/images"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/disintegration/imaging"
)

func DoWork(q *queue.Queue) {
	for {
		job := q.GetWork()
		log.Println("worker picked job: id =", job.JobId)
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
