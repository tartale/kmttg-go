package model

import "fmt"

func (job *Job) String() string {
	return fmt.Sprintf("action: %s ID: %s", job.Action, job.ID)
}
