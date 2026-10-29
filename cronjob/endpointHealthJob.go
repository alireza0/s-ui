package cronjob

import (
	"github.com/alireza0/s-ui/service"
)

// EndpointHealthJob keeps one service instance for its whole life: the
// failure counts and rebuild history that rate-limit rebuilds live in it.
type EndpointHealthJob struct {
	health *service.EndpointHealthService
}

func NewEndpointHealthJob() *EndpointHealthJob {
	return &EndpointHealthJob{health: service.NewEndpointHealthService()}
}

func (j *EndpointHealthJob) Run() {
	j.health.Run()
}
