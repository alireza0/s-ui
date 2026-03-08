package cronjob

import (
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/logger"
)

type WALCheckpointJob struct{}

func NewWALCheckpointJob() *WALCheckpointJob {
	return &WALCheckpointJob{}
}

func (s *WALCheckpointJob) Run() {
	db := database.GetDB()
	_, err := db.Raw("PRAGMA wal_checkpoint(FULL)").Rows()
	if err != nil {
		logger.Error("Error checkpointing WAL: ", err.Error())
	}
}
