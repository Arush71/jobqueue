package workers

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/Arush71/jobqueue/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createIntegrationImage(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.jpg")
	f, err := os.Create(path)
	require.NoError(t, err)
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(1, 1, color.White)
	require.NoError(t, jpeg.Encode(f, img, nil))
	require.NoError(t, f.Close())
	return path
}

func waitForState(t *testing.T, queries *db.Queries, id int64, expected db.JobState) db.Job {
	t.Helper()
	var job db.Job
	require.Eventually(t, func() bool {
		var err error
		job, err = queries.GetJobById(context.Background(), id)
		return err == nil && job.State == expected
	}, 3*time.Second, 10*time.Millisecond)
	return job
}

func TestWorkerEndToEndIntegration(t *testing.T) {
	pool, queries := testutil.OpenTestDB(t)
	create := func(jobType string, payload []byte) int64 {
		id, err := queries.CreateJob(context.Background(), db.CreateJobParams{
			JobType: jobType, State: db.JobStateQueued, Payload: payload, JobPriority: db.QueuePriorityNormal,
		})
		require.NoError(t, err)
		return id
	}

	flakyID := create(jobs.JobFlakyType, []byte(`{"fail_rate":0,"delay_sec":0}`))
	imagePath := createIntegrationImage(t)
	imagePayload, err := json.Marshal(map[string]any{
		"image_job_type": "resize", "image_path": imagePath,
		"params": map[string]float64{"width": 4, "height": 3},
	})
	require.NoError(t, err)
	imageID := create(jobs.JobImageType, imagePayload)
	failingID := create(jobs.JobFlakyType, []byte(`{"fail_rate":1,"delay_sec":0}`))
	_, err = pool.Exec(context.Background(), `UPDATE jobs SET retry_counter = $1 WHERE id = $2`, jobs.MaxRetries, failingID)
	require.NoError(t, err)

	q := queue.SetupQueue(workerLogger(), 1)
	scheduler := queue.InitScheduler(q)
	workerPool := NewPool(1, workerLogger(), q, scheduler, queries)
	workerPool.Start()
	t.Cleanup(func() {
		workerPool.Stop(time.Second)
		scheduler.Stop()
	})
	for _, id := range []int64{flakyID, imageID, failingID} {
		require.NoError(t, q.EnqueueJob(id, queue.Normal, false))
	}

	flakyJob := waitForState(t, queries, flakyID, db.JobStateSuccess)
	assert.JSONEq(t, `{"status":"success"}`, string(flakyJob.Results))
	imageJob := waitForState(t, queries, imageID, db.JobStateSuccess)
	assert.NotEmpty(t, imageJob.Results)
	assert.FileExists(t, filepath.Join(filepath.Dir(imagePath), "source_output.jpg"))
	failedJob := waitForState(t, queries, failingID, db.JobStateFail)
	assert.True(t, failedJob.Error.Valid)
}
