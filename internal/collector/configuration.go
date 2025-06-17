package collector

import (
	"time"

	"codeberg.org/clambin/go-common/flagger"
)

type Configuration struct {
	Interval time.Duration `flagger.usage:"Interval to collect statistics"`
	Device   string        `flagger.usage:"Device to collect statistics from (-d parameter of intel_gpu_top)"`
	flagger.Log
	flagger.Prom
}
