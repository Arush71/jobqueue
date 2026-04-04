package workers

import (
	"log"
	"time"

	"github.com/Arush71/jobqueue/internal/images"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/disintegration/imaging"
)

func DoWork(q *queue.Queue) {
	for {
		job, ok := q.GetNextJob()
		if !ok {
			time.Sleep(2 * time.Second)
			continue
		}
		log.Println("worker picked job: id =", job.JobId)
		q.UpdateJob(job, jobs.Processing)
		img, format, err := images.GetDecocdedImage(job.ImagePath)
		if err != nil {
			log.Println("error worker: couldn't either open or decode the image.", job.JobId)
			q.UpdateJob(job, jobs.Fail)
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
			q.UpdateJob(job, jobs.Fail)
			continue

		}
		log.Println("Job successfull", job.JobId)
		q.UpdateJob(job, jobs.Success)
	}
}
