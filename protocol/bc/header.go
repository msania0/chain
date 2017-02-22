package bc

type Header struct {
	body struct {
		Version              uint64
		Results              []*EntryRef
		Data                 Hash
		MinTimeMS, MaxTimeMS uint64
		ExtHash              Hash
	}
}

const typeHeader = "txheader"

func (Header) Type() string            { return typeHeader }
func (h *Header) Body() interface{}    { return &h.body }
func (h *Header) Witness() interface{} { return nil }

func (h *Header) Version() uint64 {
	return h.body.Version
}

func (h *Header) MinTimeMS() uint64 {
	return h.body.MinTimeMS
}

func (h *Header) MaxTimeMS() uint64 {
	return h.body.MaxTimeMS
}

func (h *Header) Results() []*EntryRef {
	return h.body.Results
}

func (h *Header) Data() Hash {
	return h.body.Data
}

func (h *Header) Walk(visitor func(*EntryRef) error) error {
	visited := make(map[Hash]bool)
	visit := func(e *EntryRef) error {
		if e == nil {
			return nil
		}
		h := e.Hash()
		if visited[h] {
			return nil
		}
		visited[h] = true
		return visitor(e)
	}
	for _, res := range h.body.Results {
		err := visit(res)
		if err != nil {
			return err
		}
		switch e2 := res.Entry.(type) {
		case *Issuance:
			err = visit(e2.body.Anchor)
			if err != nil {
				return err
			}
			err = visit(e2.witness.Destination.Ref)
			if err != nil {
				return err
			}
		case *mux:
			for _, vs := range e2.body.Sources {
				err = visit(vs.Ref)
				if err != nil {
					return err
				}
			}
		case *Nonce:
			err = visit(e2.body.TimeRange)
			if err != nil {
				return err
			}
		case *Output:
			err = visit(e2.body.Source.Ref)
			if err != nil {
				return err
			}
		case *Retirement:
			err = visit(e2.body.Source.Ref)
			if err != nil {
				return err
			}
		case *Spend:
			err = visit(e2.body.SpentOutput)
			if err != nil {
				return err
			}
			err = visit(e2.witness.Destination.Ref)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Inputs returns all input entries (as two lists: spends and
// issuances) reachable from a header's result entries.
func (h *Header) Inputs() (spends, issuances []*EntryRef) {
	h.Walk(func(e *EntryRef) error {
		switch e.Entry.(type) {
		case *Spend:
			spends = append(spends, e)
		case *Issuance:
			issuances = append(issuances, e)
		}
		return nil
	})
	return
}

func newHeader(version uint64, results []*EntryRef, data Hash, minTimeMS, maxTimeMS uint64) *Header {
	h := new(Header)
	h.body.Version = version
	h.body.Results = results
	h.body.Data = data
	h.body.MinTimeMS = minTimeMS
	h.body.MaxTimeMS = maxTimeMS
	return h
}
