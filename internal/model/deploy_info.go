package model

import (
	"strconv"
	"time"
)

type DeployID int64 // Unix ns after epoch -- uniquely identify a deploy

func NewDeployID() DeployID {
	return DeployID(time.Now().UnixNano())
}

func (dID DeployID) String() string { return strconv.Itoa(int(dID)) }
func (dID DeployID) Empty() bool    { return dID == 0 }
