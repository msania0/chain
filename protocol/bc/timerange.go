package bc

type TimeRange struct {
	body struct {
		MinTimeMS, MaxTimeMS uint64
		ExtHash              Hash
	}
}

const typeTimeRange = "timerange1"

func (TimeRange) Type() string             { return typeTimeRange }
func (tr *TimeRange) Body() interface{}    { return &tr.body }
func (tr *TimeRange) Witness() interface{} { return nil }

func (tr *TimeRange) MinTimeMS() uint64 {
	return tr.body.MinTimeMS
}

func (tr *TimeRange) MaxTimeMS() uint64 {
	return tr.body.MaxTimeMS
}

func newTimeRange(minTimeMS, maxTimeMS uint64) *TimeRange {
	tr := new(TimeRange)
	tr.body.MinTimeMS = minTimeMS
	tr.body.MaxTimeMS = maxTimeMS
	return tr
}
