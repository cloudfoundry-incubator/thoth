package benchmark

import (
	"strconv"
	"time"
)

type BenchmarkResponse struct {
	TotalRoundrip time.Duration
	TimeInApp     time.Duration
	TimeInRouter  time.Duration
	RestOfTime    time.Duration
	Timestamp     time.Time

	ResponseCode int
}

func (br BenchmarkResponse) ToDatadog(deploymentName string) map[string]interface{} {
	now := br.Timestamp
	tags := []string{
		"status:" + strconv.Itoa(br.ResponseCode),
		"deployment:" + deploymentName,
	}
	return map[string]interface{}{
		"series": []map[string]interface{}{
			{
				"metric": "app_benchmarking.total_roundtrip",
				"points": [][]int64{
					{now.Unix(), br.TotalRoundrip.Nanoseconds()},
				},
				"tags": tags,
			},
			{
				"metric": "app_benchmarking.time_in_gorouter",
				"points": [][]int64{
					{now.Unix(), br.TimeInRouter.Nanoseconds()},
				},
				"tags": tags,
			},
			{
				"metric": "app_benchmarking.time_in_app",
				"points": [][]int64{
					{now.Unix(), br.TimeInApp.Nanoseconds()},
				},
				"tags": tags,
			},
			{
				"metric": "app_benchmarking.rest_of_time",
				"points": [][]int64{
					{now.Unix(), br.RestOfTime.Nanoseconds()},
				},
				"tags": tags,
			},
		},
	}
}
