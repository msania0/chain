package bc

type Retirement struct {
	body struct {
		Source  valueSource
		Data    *EntryRef
		ExtHash Hash
	}
}

const typeRetirement = "retirement1"

func (Retirement) Type() string            { return typeRetirement }
func (r *Retirement) Body() interface{}    { return &r.body }
func (r *Retirement) Witness() interface{} { return nil }

func (r *Retirement) AssetID() AssetID {
	return r.body.Source.Value.AssetID
}

func (r *Retirement) Amount() uint64 {
	return r.body.Source.Value.Amount
}

func (r *Retirement) Data() *EntryRef {
	return r.body.Data
}

func newRetirement(source valueSource, data *EntryRef) *Retirement {
	r := new(Retirement)
	r.body.Source = source
	r.body.Data = data
	return r
}
