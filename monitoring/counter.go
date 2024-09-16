// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package monitoring

import (
	"apiguard/common"
	"apiguard/guard"
	"encoding/json"
	"fmt"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
)

const ()

type exceeding struct {
	Value       int
	MeasureTime time.Time
}

type limitExceedingItemJSONExport struct {
	Exceeding float64 `json:"exceeding"`
}

// limitExceedings keeps information about several recent
// num. of requests overflow (for a specified CheckInterval).
// This allows for hysteresis-like behavior where we rely not
// just on actual single limit exceeding, but also on the situation
// in the near past.
type limitExceedings struct {
	Entries         map[common.CheckInterval]*collections.CircularList[exceeding]
	LastMeasurement time.Time
	Conf            *LimitingConf
}

func NewLimitExceedings(conf *LimitingConf) *limitExceedings {
	return &limitExceedings{
		Entries: make(map[common.CheckInterval]*collections.CircularList[exceeding]),
		Conf:    conf,
	}
}

func (lime *limitExceedings) Get(interval common.CheckInterval) (*collections.CircularList[exceeding], bool) {
	v, ok := lime.Entries[interval]
	return v, ok
}

func (lime *limitExceedings) Set(interval common.CheckInterval, v *collections.CircularList[exceeding]) {
	lime.Entries[interval] = v
}

func (lime *limitExceedings) Touch(at time.Time) {
	lime.LastMeasurement = at
}

// registerMeasurement makes a measurement comparing a provided limit and interval
// with actual number of requests. If no exceeding is detected, the method stores
// nothing.
func (lime *limitExceedings) registerMeasurement(
	at time.Time,
	interval common.CheckInterval,
	numReq, limit int,
) {
	lime.LastMeasurement = at
	if numReq < limit {
		return
	}
	list, ok := lime.Get(interval)
	if !ok {
		list = collections.NewCircularList[exceeding](
			lime.Conf.ExceedingsBufferSize)
		lime.Set(interval, list)
	}
	list.Append(exceeding{Value: max(0, numReq-limit), MeasureTime: at})
}

func (lime *limitExceedings) Range() func(yield func(common.CheckInterval, *collections.CircularList[exceeding]) bool) {
	return func(yield func(k common.CheckInterval, v *collections.CircularList[exceeding]) bool) {
		for k, v := range lime.Entries {
			if !yield(k, v) {
				break
			}
		}
	}
}

func (limit *limitExceedings) MarshalJSON() ([]byte, error) {
	ans := make(map[string]limitExceedingItemJSONExport)
	for k := range limit.Range() {
		item := limitExceedingItemJSONExport{
			Exceeding: limit.absoluteExceeding(limit.LastMeasurement, k),
		}
		ans[k.String()] = item
	}

	return json.Marshal(ans)
}

// relativeExceeding calculates relative (to the specified `interval`) exceeding
// of the access limit. The included measurements are weighted by their age
// (i.e. recent measurement has higher weight than an older one).
func (lime *limitExceedings) relativeExceeding(
	at time.Time,
	interval common.CheckInterval,
	limit int,
) (ans float64) {
	list, ok := lime.Get(interval)
	if !ok {
		return
	}
	list.ForEach(func(i int, item exceeding) bool {
		timeDiff := at.Sub(item.MeasureTime)
		if timeDiff > time.Duration(interval) {
			return true
		}
		// decrRatio: we need to spread the weights in a way relative to the `interval`
		// e.g. 2 minutes old record means something different when the interval is 5 minutes
		// and whe the interval is 24 hours.
		decrRatio := 9 * min(1, float64(timeDiff.Seconds())/float64(interval.ToSeconds()))
		ans += float64(item.Value) / (1 + timeDiff.Seconds()*decrRatio)
		return true
	})
	ans = ans / float64(list.Len()) / float64(limit)
	return
}

func (lime *limitExceedings) absoluteExceeding(
	at time.Time,
	interval common.CheckInterval,
) (ans float64) {
	list, ok := lime.Get(interval)
	if !ok {
		return
	}
	list.ForEach(func(i int, item exceeding) bool {
		timeDiff := at.Sub(item.MeasureTime)
		if timeDiff > time.Duration(interval) {
			return true
		}
		decrRatio := 9 * min(1, float64(timeDiff.Seconds())/float64(interval.ToSeconds()))
		ans += float64(item.Value) / (1 + timeDiff.Seconds()*decrRatio)
		return true
	})
	ans = ans / float64(list.Len())
	return
}

// -------------------------------------------------

// UserActivity stores recent requests and limit exceeding
// info for a concrete service and concrete user using
// the service.
type UserActivity struct {

	// Requests contains information about recent N request.
	Requests *collections.CircularList[reqCounterItem]

	// NumReqAboveLimit specifies how many requests were above
	// when considering specified check interval during a checking
	// period. This determines possible response delay or ban.
	NumReqAboveLimit *limitExceedings
}

// NumReqSince counts requests with time after a specified interval
// from now (e.g. "newer than 2 hours ago")
func (ulm *UserActivity) NumReqSince(interval time.Duration, loc *time.Location) int {
	limit := time.Now().In(loc).Add(-interval)
	var ans int
	ulm.Requests.ForEach(func(i int, item reqCounterItem) bool {
		if item.Created.After(limit) {
			ans++
		}
		return true
	})
	return ans
}

// ClientRequests collects information about recent clients
// and their activity.
type ClientRequests struct {
	collections.ConcurrentMap[string, *UserActivity]
}

func (cr *ClientRequests) mkKey(props guard.RequestInfo) string {
	return fmt.Sprintf("%d@%s", props.UserID, props.IP)
}

func (cr *ClientRequests) GetByProps(props guard.RequestInfo) *UserActivity {
	return cr.Get(cr.mkKey(props))
}

func (cr *ClientRequests) HasByProps(props guard.RequestInfo) bool {
	return cr.HasKey(cr.mkKey(props))
}

func (cr *ClientRequests) SetByProps(props guard.RequestInfo, v *UserActivity) {
	cr.Set(cr.mkKey(props), v)
}

func NewClientRequests() *ClientRequests {
	return &ClientRequests{
		*collections.NewConcurrentMap[string, *UserActivity](),
	}
}

func NewClientRequestsFrom(data map[string]*UserActivity) *ClientRequests {
	return &ClientRequests{
		*collections.NewConcurrentMapFrom(data),
	}
}

func (cr *ClientRequests) CountRequests() (ans int) {
	cr.ForEach(func(k string, v *UserActivity, ok bool) {
		if !ok {
			return
		}
		ans += v.Requests.Len()
	})
	return
}
