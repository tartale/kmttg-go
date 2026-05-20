package jobs

import (
	"context"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/puzpuzpuz/xsync"
	"github.com/tartale/go/pkg/primitives"
	"github.com/tartale/kmttg-plus/go/pkg/logz"
	"github.com/tartale/kmttg-plus/go/pkg/model"
	"github.com/tartale/kmttg-plus/go/pkg/tivos"
	"go.uber.org/zap"
)

var jobs = xsync.NewMapOf[*Job]()

type Job struct {
	*model.Job
	pipeline *Pipeline
}

func NewJob(job *model.Job) (*Job, error) {
	show, err := tivos.GetShowForID(job.ShowID)
	if err != nil {
		logz.Logger.Warn("Error attempting to start job", zap.Error(err))
		return nil, err
	}

	job.ID = primitives.Ref(uuid.NewString())
	logz.Logger.Info("Started job", zap.String("job", job.String()), zap.String("jobID", *job.ID))
	return &Job{
		Job:      job,
		pipeline: NewPipeline(job, show),
	}, nil
}

func StartJob(ctx context.Context, job *Job) (*model.JobStatus, error) {
	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Var(job.ShowID, "filepath")
	if err != nil {
		return nil, err
	}

	err = job.pipeline.Start()
	if err != nil {
		return nil, err
	}
	jobs.Store(*job.ID, job)

	return job.pipeline.Status(), nil
}

func List(ctx context.Context, filters []*model.JobFilter) ([]*model.JobStatus, error) {
	result := []*model.JobStatus{}

	jobs.Range(func(key string, val *Job) bool {
		jobStatus := val.pipeline.Status()
		result = append(result, jobStatus)
		return true
	})

	return result, nil
}
