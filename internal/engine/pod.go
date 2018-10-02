package engine

import (
	"github.com/google/uuid"
)

const TiltRunIDLabel = "tilt-runid"

var TiltRunID = uuid.New().String()
